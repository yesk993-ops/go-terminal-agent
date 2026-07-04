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

	"golang.org/x/term"

	"github.com/agent/ai-terminal/internal/agent"
	"github.com/agent/ai-terminal/internal/cache"
	"github.com/agent/ai-terminal/internal/config"
	"github.com/agent/ai-terminal/internal/core"
	"github.com/agent/ai-terminal/internal/logger"
	"github.com/agent/ai-terminal/internal/provider"
	"github.com/agent/ai-terminal/internal/session"
	"github.com/agent/ai-terminal/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
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
		ag := agent.New(p, nil)
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
	return provider.Get(providerName, resolveProviderConfig(cfg, providerName, modelFlag))
}

func stripMarkdown(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if i+1 < len(s) && s[i] == '*' && s[i+1] == '*' {
			if i > 0 && s[i-1] >= '0' && s[i-1] <= '9' {
				b.WriteByte(s[i])
				continue
			}
			if i+2 < len(s) && s[i+2] >= '0' && s[i+2] <= '9' {
				b.WriteByte(s[i])
				continue
			}
			j := i + 2
			matched := false
			for j+1 < len(s) && !(s[j] == '*' && s[j+1] == '*') {
				b.WriteByte(s[j])
				j++
			}
			if j+1 < len(s) {
				i = j + 1
				matched = true
			}
			if matched {
				continue
			}
		}
		if s[i] == '*' && (i == 0 || s[i-1] == ' ' || s[i-1] == '\n') {
			if i+1 < len(s) && s[i+1] == ' ' {
				b.WriteByte(s[i])
				continue
			}
			j := i + 1
			matched := false
			for j < len(s) && s[j] != '*' && s[j] != '\n' {
				b.WriteByte(s[j])
				j++
			}
			if j < len(s) && s[j] == '*' {
				i = j
				matched = true
			}
			if matched {
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func stripEmojis(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 0x2600 && r <= 0x27BF,
			r >= 0x1F300 && r <= 0x1FAFF,
			r >= 0x2702 && r <= 0x27B0,
			r >= 0x24C2 && r <= 0x1F251,
			r == 0x200D, r == 0xFE0F, r == 0x2139,
			r >= 0x2B05 && r <= 0x2B55,
			r >= 0x2934 && r <= 0x2935,
			r >= 0x25AA && r <= 0x25FE:
			continue
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func wrapLine(line string, maxWidth int) string {
	if len(line) <= maxWidth {
		return line
	}

	words := strings.Fields(line)
	if len(words) == 0 {
		return line
	}

	var out strings.Builder
	var cur strings.Builder
	curLen := 0

	flush := func() {
		if cur.Len() > 0 {
			trimmed := strings.TrimRight(cur.String(), " ")
			if out.Len() > 0 {
				out.WriteByte('\n')
			}
			out.WriteString(trimmed)
			cur.Reset()
			curLen = 0
		}
	}

	for _, word := range words {
		if curLen+len(word)+1 > maxWidth && curLen > 0 {
			flush()
		}
		if curLen > 0 {
			cur.WriteByte(' ')
			curLen++
		}
		cur.WriteString(word)
		curLen += len(word)
	}
	flush()

	return out.String()
}

func terminalWidth(fd int) int {
	w, _, err := term.GetSize(fd)
	if err != nil || w < 40 {
		return 80
	}
	return w - 1
}

func renderAnswer(s string) string {
	s = stripEmojis(s)
	s = strings.TrimSpace(s)

	width := terminalWidth(int(os.Stdout.Fd()))

	lines := strings.Split(s, "\n")
	var wrapped []string
	for _, l := range lines {
		line := l
		if strings.HasPrefix(line, "### ") {
			line = "\033[35m" + strings.TrimPrefix(line, "### ") + "\033[39m"
		} else if strings.HasPrefix(line, "## ") {
			line = "\033[35m" + strings.TrimPrefix(line, "## ") + "\033[39m"
		} else if strings.HasPrefix(line, "# ") {
			line = "\033[35m" + strings.TrimPrefix(line, "# ") + "\033[39m"
		} else {
			line = stripMarkdown(line)
		}
		w := wrapLine(line, width)
		if strings.Contains(w, "\n") {
			wrapped = append(wrapped, strings.Split(w, "\n")...)
		} else {
			wrapped = append(wrapped, w)
		}
	}

	var out strings.Builder
	for _, l := range wrapped {
		out.WriteString(l + "\n")
	}
	return out.String()
}

func runOnce(ag core.Agent, prompt string, w io.Writer) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &core.Request{
		Messages: []core.Message{
			{
				Role:    core.RoleSystem,
				Content: "You are a highly capable AI assistant. Provide accurate, well-structured answers with depth and clarity. Break down complex topics step by step. Use concrete examples. When uncertain, acknowledge limitations rather than guessing. Be thorough — cover what matters, not just surface-level information. Keep responses self-contained and practical.",
			},
			{
				Role:    core.RoleUser,
				Content: prompt,
			},
		},
		Stream:    true,
		MaxTokens: 4096,
	}

	ch, err := ag.Run(ctx, req)
	if err != nil {
		return fmt.Errorf("agent run: %w", err)
	}

	var buf strings.Builder
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
			buf.WriteString(tok.Content)
		}
	}

	rendered := renderAnswer(buf.String())
	fmt.Fprint(w, rendered)
	return nil
}
