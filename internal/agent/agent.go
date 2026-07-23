package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/agent/ai-terminal/internal/core"
	"github.com/agent/ai-terminal/internal/logger"
)

const maxToolCalls = 3

type DefaultAgent struct {
	mu       sync.RWMutex
	provider core.Provider
	tools    core.ToolRegistry
	cache    core.Cache
	session  core.Session
}

type Option func(*DefaultAgent)

func WithCache(c core.Cache) Option {
	return func(a *DefaultAgent) {
		a.cache = c
	}
}

func WithSession(s core.Session) Option {
	return func(a *DefaultAgent) {
		a.session = s
	}
}

func New(provider core.Provider, tools core.ToolRegistry, opts ...Option) *DefaultAgent {
	a := &DefaultAgent{
		provider: provider,
		tools:    tools,
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

func (a *DefaultAgent) SetProvider(p core.Provider) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.provider = p
}

func (a *DefaultAgent) SetTools(r core.ToolRegistry) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.tools = r
}

func (a *DefaultAgent) Run(ctx context.Context, req *core.Request) (<-chan core.Token, error) {
	a.mu.RLock()
	provider := a.provider
	tools := a.tools
	a.mu.RUnlock()

	if provider == nil {
		return nil, fmt.Errorf("no provider configured")
	}
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}

	// The caller owns request history. In particular, interactive callers send
	// prior session messages with each request, so appending every request
	// message here would duplicate the conversation on every turn. The agent
	// persists only messages it creates itself (assistant replies and tool
	// results).
	outCh := make(chan core.Token, 128)

	go func() {
		defer close(outCh)
		a.runLoop(ctx, provider, tools, req, outCh)
	}()

	return outCh, nil
}

func (a *DefaultAgent) runLoop(ctx context.Context, provider core.Provider, tools core.ToolRegistry, req *core.Request, outCh chan core.Token) {
	currentReq := copyRequest(req)

	var userQuery string
	for i := len(currentReq.Messages) - 1; i >= 0; i-- {
		if currentReq.Messages[i].Role == core.RoleUser {
			userQuery = currentReq.Messages[i].Content
			break
		}
	}

	for turn := 0; turn <= maxToolCalls; turn++ {
		cacheKey := a.cacheKey(provider, currentReq)
		if a.cache != nil {
			if cached, ok := a.cache.Get(cacheKey); ok {
				a.send(ctx, outCh, core.Token{Content: cached})
				a.send(ctx, outCh, core.Token{Done: true})
				return
			}
		}

		tokenCh, err := provider.Stream(ctx, currentReq)
		if err != nil {
			a.send(ctx, outCh, core.Token{Error: fmt.Errorf("provider stream: %w", err), Done: true})
			return
		}

		var fullContent strings.Builder
		var toolCall *core.ToolCall

		// Native or text-parsed tool calls require a complete response before
		// execution. Normal chat requests have no tools, so they take the fast
		// path and are forwarded as soon as tokens arrive.
		bufferForTools := tools != nil
		buf := make([]core.Token, 0, 64)

		for token := range tokenCh {
			if token.Error != nil {
				a.send(ctx, outCh, token)
				return
			}

			if token.ToolCall != nil {
				toolCall = token.ToolCall
				continue
			}

			if token.Content != "" {
				fullContent.WriteString(token.Content)
				if bufferForTools {
					buf = append(buf, token)
				} else if !a.send(ctx, outCh, token) {
					return
				}
			}

			if token.Done {
				break
			}
		}

		content := fullContent.String()
		parsedToolCall := parseTextToolCall(content, tools)
		isToolCall := tools != nil && (toolCall != nil || parsedToolCall != nil)

		// If we've reached the max tool call limit, treat the response as final.
		if turn == maxToolCalls && isToolCall {
			for _, t := range buf {
				if t.Content != "" && !a.send(ctx, outCh, t) {
					return
				}
			}
			a.finishResponse(ctx, outCh, cacheKey, content, userQuery)
			return
		}

		if isToolCall {
			tc := toolCall
			if tc == nil {
				tc = parsedToolCall
			}
			if !a.onToolCall(ctx, tc, tools, currentReq, outCh) {
				return
			}
			continue
		}

		if bufferForTools {
			for _, t := range buf {
				if t.Content != "" && !a.send(ctx, outCh, t) {
					return
				}
			}
		}

		a.finishResponse(ctx, outCh, cacheKey, content, userQuery)
		return
	}
}

func (a *DefaultAgent) finishResponse(ctx context.Context, outCh chan<- core.Token, cacheKey, content, query string) {
	content = core.TrimToSentenceBoundary(content)
	qc := core.CheckResponseQuality(content, query)
	if !qc.Passed {
		logger.L().Warn("response quality check failed",
			"score", qc.RelevanceScore,
			"issues", qc.Issues,
			"suggestions", qc.Suggestions,
		)
	}

	if a.session != nil && content != "" {
		a.session.Append(core.Message{
			Role:    core.RoleAssistant,
			Content: content,
		})
	}
	if a.cache != nil && content != "" {
		a.cache.Set(cacheKey, content, 0)
	}
	a.send(ctx, outCh, core.Token{Done: true})
}

func (a *DefaultAgent) send(ctx context.Context, outCh chan<- core.Token, token core.Token) bool {
	select {
	case outCh <- token:
		return true
	case <-ctx.Done():
		return false
	}
}

func (a *DefaultAgent) executeTool(ctx context.Context, tc *core.ToolCall, reg core.ToolRegistry) *core.ToolResult {
	tool, ok := reg.Get(tc.Name)
	if !ok {
		return &core.ToolResult{
			Status: core.StatusError,
			Error:  fmt.Sprintf("tool %q not found", tc.Name),
		}
	}

	logger.L().Debug("executing tool", "tool", tc.Name, "args", string(tc.Args))
	result := tool.Execute(ctx, tc.Args)
	logger.L().Debug("tool result", "tool", tc.Name, "status", result.Status)

	return result
}

type cacheRequest struct {
	Model     string         `json:"model"`
	Messages  []core.Message `json:"messages"`
	Tools     []core.ToolDef `json:"tools,omitempty"`
	MaxTokens int            `json:"max_tokens"`
	Options   map[string]any `json:"options,omitempty"`
}

// cacheKey produces an unambiguous, bounded cache key. Provider and model are
// included so a reply from one quality/configuration profile is never reused
// for another.
func (a *DefaultAgent) cacheKey(provider core.Provider, req *core.Request) string {
	identity := cacheRequest{
		Model:     req.Model,
		Messages:  req.Messages,
		Tools:     req.Tools,
		MaxTokens: req.MaxTokens,
		Options:   req.Options,
	}
	encoded, err := json.Marshal(identity)
	if err != nil {
		// Options are expected to be JSON-compatible. Preserve a safe fallback
		// if a third-party caller supplies an unsupported value.
		encoded = []byte(fmt.Sprintf("%q|%d|%v", req.Model, req.MaxTokens, req.Messages))
	}
	sum := sha256.Sum256(encoded)
	return provider.Name() + ":" + hex.EncodeToString(sum[:])
}

func (a *DefaultAgent) onToolCall(ctx context.Context, tc *core.ToolCall, tools core.ToolRegistry, req *core.Request, outCh chan<- core.Token) bool {
	logger.L().Debug("executing tool", "tool", tc.Name, "args", string(tc.Args))

	result := a.executeTool(ctx, tc, tools)

	statusIcon := "✓"
	if result.Status == core.StatusError {
		statusIcon = "✗"
	}
	if !a.send(ctx, outCh, core.Token{
		Content: fmt.Sprintf("\n\n[%s Tool: %s]\n", statusIcon, tc.Name),
	}) {
		return false
	}

	toolResult := fmt.Sprintf("Tool %s returned:\n%s", tc.Name, result.Output)
	if result.Error != "" {
		toolResult = fmt.Sprintf("Tool %s failed: %s", tc.Name, result.Error)
	}

	toolMsg := core.Message{
		Role:    core.RoleUser,
		Content: toolResult,
	}

	req.Messages = append(req.Messages, toolMsg)

	if a.session != nil {
		a.session.Append(toolMsg)
	}
	return true
}

func parseTextToolCall(content string, tools core.ToolRegistry) *core.ToolCall {
	if tools == nil {
		return nil
	}

	for _, t := range tools.List() {
		name := t.Name()

		patterns := []struct {
			prefix string
			suffix string
		}{
			{"<function(" + name, "</function>"},
			{"<function_" + name + ">", "</function>"},
			{"<function." + name + ">", "</function>"},
			{"<" + name + ">", "</" + name + ">"},
		}

		for _, p := range patterns {
			idx := strings.Index(content, p.prefix)
			if idx < 0 {
				continue
			}
			start := idx + len(p.prefix)
			end := strings.LastIndex(content, p.suffix)
			if end > start {
				argsStr := content[start:end]
				argsStr = strings.TrimRight(argsStr, ") ")
				argsStr = strings.TrimSpace(argsStr)

				if !strings.HasPrefix(argsStr, "{") {
					braceIdx := strings.Index(argsStr, "{")
					if braceIdx >= 0 {
						argsStr = argsStr[braceIdx:]
					}
				}

				return &core.ToolCall{
					ID:   "text_fcall",
					Name: name,
					Args: []byte(argsStr),
				}
			}
		}
	}

	return nil
}

func copyRequest(req *core.Request) *core.Request {
	msgs := make([]core.Message, len(req.Messages))
	copy(msgs, req.Messages)

	tools := make([]core.ToolDef, len(req.Tools))
	copy(tools, req.Tools)

	opts := make(map[string]any, len(req.Options))
	for k, v := range req.Options {
		opts[k] = v
	}

	return &core.Request{
		Model:     req.Model,
		Messages:  msgs,
		Tools:     tools,
		Stream:    true,
		MaxTokens: req.MaxTokens,
		Options:   opts,
	}
}

var _ core.Agent = (*DefaultAgent)(nil)
