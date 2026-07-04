package provider

import (
	"context"
	"encoding/json"

	"github.com/agent/ai-terminal/internal/core"
)

type groqProvider struct {
	baseProvider
}

func init() {
	Register("groq", func(cfg core.ProviderConfig) core.Provider {
		return &groqProvider{
			baseProvider: newBaseProvider("groq", cfg.APIKey, cfg.Model, cfg.BaseURL),
		}
	})
}

func (p *groqProvider) Stream(ctx context.Context, req *core.Request) (<-chan core.Token, error) {
	chatReq := map[string]any{
		"model":       p.model,
		"messages":    convertMessages(req.Messages),
		"stream":      true,
		"max_tokens":  req.MaxTokens,
		"temperature": 0.7,
	}

	if len(req.Tools) > 0 {
		tools := make([]map[string]any, 0, len(req.Tools))
		for _, t := range req.Tools {
			var schema map[string]any
			_ = json.Unmarshal(t.InputSchema, &schema)
			tools = append(tools, map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        t.Name,
					"description": t.Description,
					"parameters":  schema,
				},
			})
		}
		chatReq["tools"] = tools
	}

	resp, err := p.doPost(ctx, p.baseURL+"/chat/completions", chatReq, nil)
	if err != nil {
		return nil, err
	}

	return streamOpenAICompatibleSSE(ctx, resp)
}
