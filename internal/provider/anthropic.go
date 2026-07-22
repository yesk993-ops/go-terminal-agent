package provider

import (
	"context"

	"github.com/agent/ai-terminal/internal/core"
)

type anthropicProvider struct {
	baseProvider
}

func init() {
	Register("anthropic", func(cfg core.ProviderConfig) core.Provider {
		return &anthropicProvider{
			baseProvider: newBaseProvider("anthropic", cfg.APIKey, cfg.Model, cfg.BaseURL),
		}
	})
}

func (p *anthropicProvider) Stream(ctx context.Context, req *core.Request) (<-chan core.Token, error) {
	messages := make([]chatMessage, 0, len(req.Messages))
	var systemContent string

	for _, m := range req.Messages {
		if m.Role == core.RoleSystem {
			if systemContent != "" {
				systemContent += "\n" + m.Content
			} else {
				systemContent = m.Content
			}
			continue
		}
		var content any = m.Content
		messages = append(messages, chatMessage{Role: string(m.Role), Content: content})
	}

	chatReq := map[string]any{
		"model":      p.model,
		"messages":   messages,
		"stream":     true,
		"max_tokens": req.MaxTokens,
	}

	if systemContent != "" {
		chatReq["system"] = systemContent
	}

	headers := map[string]string{
		"anthropic-version": "2023-06-01",
		"x-api-key":         p.apiKey,
	}

	resp, err := p.doPost(ctx, p.baseURL+"/messages", chatReq, headers)
	if err != nil {
		return nil, err
	}

	return streamAnthropicSSE(ctx, resp)
}
