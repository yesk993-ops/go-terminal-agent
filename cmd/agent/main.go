package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/agent/ai-terminal/internal/agent"
	"github.com/agent/ai-terminal/internal/cache"
	"github.com/agent/ai-terminal/internal/config"
	"github.com/agent/ai-terminal/internal/core"
	"github.com/agent/ai-terminal/internal/logger"
	"github.com/agent/ai-terminal/internal/provider"
	"github.com/agent/ai-terminal/internal/session"
	"github.com/agent/ai-terminal/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
)

func main() {
	cfgPath := flag.String("config", "", "Path to config file")
	providerFlag := flag.String("provider", "", "LLM provider to use")
	modelFlag := flag.String("model", "", "Model to use")
	listProviders := flag.Bool("list-providers", false, "List available providers")
	flag.Parse()

	if *listProviders {
		fmt.Println("Available providers:")
		for _, name := range provider.ListAvailable() {
			fmt.Printf("  - %s\n", name)
		}
		os.Exit(0)
	}

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	logger.Init(cfg.Logging.Level, cfg.Logging.Format, cfg.Logging.Output)
	log := logger.L()

	providerName := cfg.Provider.Default
	if *providerFlag != "" {
		providerName = *providerFlag
	}

	p := setupProvider(providerName, cfg, *modelFlag)
	if p == nil {
		log.Error("provider not available", "provider", providerName)
		fmt.Fprintf(os.Stderr, "Provider %q not found or not configured. Use --list-providers to see available providers.\n", providerName)
		os.Exit(1)
	}

	args := flag.Args()
	if len(args) > 0 {
		prompt := strings.Join(args, " ")
		var opts []agent.Option
		if cfg.Cache.Enabled {
			opts = append(opts, agent.WithCache(cache.New(cfg.Cache.MaxSize, cfg.Cache.DefaultTTL)))
		}
		ag := agent.New(p, nil, opts...)
		if err := runOnce(ag, prompt, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	var sess core.Session
	sessionStore := session.NewStore(
		cfg.Session.MaxMessages,
		cfg.Session.SavePath,
		cfg.Session.AutoSave,
	)
	sess = sessionStore.GetOrCreate("default")

	var c *cache.LRUCache
	if cfg.Cache.Enabled {
		c = cache.New(cfg.Cache.MaxSize, cfg.Cache.DefaultTTL)
	}

	ag := agent.New(p, nil,
		agent.WithCache(c),
		agent.WithSession(sess),
	)

	modelName := *modelFlag
	if modelName == "" {
		cfg2 := resolveProviderConfig(cfg, providerName, *modelFlag)
		modelName = cfg2.Model
	}
	if modelName == "" {
		modelName = "default"
	}

	setCore := func(newProvider, newModel string) error {
		if newProvider != "" {
			providerName = newProvider
		}
		if newModel != "" {
			modelName = newModel
		}
		p := setupProvider(providerName, cfg, modelName)
		if p == nil {
			return fmt.Errorf("provider %q not available", providerName)
		}
		ag.SetProvider(p)
		return nil
	}

	m := tui.New(ag, sess, providerName, modelName, setCore)
	pModel := tea.NewProgram(m, tea.WithAltScreen())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		pModel.Quit()
	}()

	if _, err := pModel.Run(); err != nil {
		log.Error("TUI error", "error", err)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func resolveProviderConfig(cfg *core.Config, providerName, modelFlag string) core.ProviderConfig {
	pCfg := core.ProviderConfig{Model: modelFlag}
	pCfg.APIKey = config.ResolveAPIKey(providerName, cfg)
	if pCfg.APIKey == "" {
		for _, pc := range cfg.Providers {
			if pc.APIKey != "" {
				pCfg.APIKey = pc.APIKey
				break
			}
		}
	}
	if pCfg.Model == "" {
		for _, pc := range cfg.Providers {
			if pc.Name == providerName && pc.Model != "" {
				pCfg.Model = pc.Model
				break
			}
		}
	}
	return pCfg
}

func setupProvider(providerName string, cfg *core.Config, modelFlag string) core.Provider {
	primary := provider.Get(providerName, resolveProviderConfig(cfg, providerName, modelFlag))
	if primary == nil {
		return nil
	}

	// Build a fallback chain: try primary first (with retry), then fall
	// through other providers if rate limited.
	fallbackOrder := []string{"openrouter", "nvidia", "groq", "openai", "anthropic", "gemini"}
	var fallbacks []core.Provider
	for _, fb := range fallbackOrder {
		if fb == providerName {
			continue // skip primary
		}
		p := provider.Get(fb, resolveProviderConfig(cfg, fb, ""))
		if p != nil {
			fallbacks = append(fallbacks, p)
		}
	}
	return provider.NewFallback(primary, fallbacks...)
}

func getTerminalWidth() int {
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		return w
	}
	return 80
}

// contentWidth is the number of columns available for answer text: the
// terminal width minus a small side margin so wrapped lines never touch the
// edge. It always fits the current terminal.
func contentWidth() int {
	w := getTerminalWidth() - 2
	if w < 20 {
		w = 20
	}
	return w
}

// systemPromptFor returns the system prompt to send. Short, simple queries get
// a trimmed prompt to save tokens; anything longer keeps the full prompt so
// answer quality is unaffected.
func systemPromptFor(prompt string) string {
	if len(prompt) <= 120 && !strings.ContainsAny(prompt, "\n") {
		return core.SystemPromptShort
	}
	return core.SystemPrompt
}

func runOnce(ag core.Agent, prompt string, w io.Writer) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &core.Request{
		Messages: []core.Message{
			{
				Role:    core.RoleSystem,
				Content: systemPromptFor(prompt),
			},
			{
				Role:    core.RoleUser,
				Content: prompt,
			},
		},
		Stream:    true,
		MaxTokens: 8192,
	}

	ch, err := ag.Run(ctx, req)
	if err != nil {
		return fmt.Errorf("agent run: %w", err)
	}

	// Stream the answer live between a top and bottom rule with open sides.
	width := contentWidth()
	fw := tui.NewFrameWriter(w, width, getTerminalWidth())
	defer fw.Close()

	for tok := range ch {
		if tok.Error != nil {
			if errors.Is(tok.Error, context.Canceled) {
				return nil
			}
			return tok.Error
		}
		if tok.Done {
			break
		}
		if tok.Content != "" {
			fw.Write(tok.Content)
		}
	}
	return nil
}
