package agent

import (
	"context"
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

	if a.session != nil {
		for _, m := range req.Messages {
			a.session.Append(m)
		}
	}

	outCh := make(chan core.Token, 128)

	go func() {
		defer close(outCh)
		a.runLoop(ctx, provider, tools, req, outCh)
	}()

	return outCh, nil
}

func (a *DefaultAgent) runLoop(ctx context.Context, provider core.Provider, tools core.ToolRegistry, req *core.Request, outCh chan core.Token) {
	currentReq := copyRequest(req)

	for turn := 0; turn <= maxToolCalls; turn++ {
		cacheKey := a.cacheKey(currentReq)
		if a.cache != nil {
			if cached, ok := a.cache.Get(cacheKey); ok {
				outCh <- core.Token{Content: cached}
				outCh <- core.Token{Done: true}
				return
			}
		}

		tokenCh, err := provider.Stream(ctx, currentReq)
		if err != nil {
			outCh <- core.Token{Error: fmt.Errorf("provider stream: %w", err), Done: true}
			return
		}

		var fullContent strings.Builder
		var toolCall *core.ToolCall

		buf := make([]core.Token, 0, 64)

		for token := range tokenCh {
			if token.Error != nil {
				outCh <- token
				return
			}

			if token.ToolCall != nil {
				toolCall = token.ToolCall
				continue
			}

			if token.Content != "" {
				fullContent.WriteString(token.Content)
			}

			buf = append(buf, token)

			if token.Done {
				break
			}
		}

		content := fullContent.String()
		isToolCall := (toolCall != nil && tools != nil) || (parseTextToolCall(content, tools) != nil && tools != nil)

		if isToolCall {
			var tc *core.ToolCall
			if toolCall != nil {
				tc = toolCall
			} else {
				tc = parseTextToolCall(content, tools)
			}
			a.onToolCall(ctx, tc, content, tools, currentReq, outCh)
			continue
		}

		for _, t := range buf {
			if t.Content != "" {
				outCh <- t
			}
		}

		if a.session != nil {
			a.session.Append(core.Message{
				Role:    core.RoleAssistant,
				Content: content,
			})
		}
		if a.cache != nil && content != "" {
			a.cache.Set(cacheKey, content, 0)
		}

		outCh <- core.Token{Done: true}
		return
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

func (a *DefaultAgent) cacheKey(req *core.Request) string {
	key := req.Model + "|"
	for _, m := range req.Messages {
		key += string(m.Role) + ":" + m.Content + "|"
	}
	return key
}

func (a *DefaultAgent) onToolCall(ctx context.Context, tc *core.ToolCall, content string, tools core.ToolRegistry, req *core.Request, outCh chan core.Token) {
	logger.L().Debug("executing tool", "tool", tc.Name, "args", string(tc.Args))

	result := a.executeTool(ctx, tc, tools)

	statusIcon := "✓"
	if result.Status == core.StatusError {
		statusIcon = "✗"
	}
	outCh <- core.Token{
		Content: fmt.Sprintf("\n\n[%s Tool: %s]\n", statusIcon, tc.Name),
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
