package provider

import (
	"context"
	"encoding/json"

	"github.com/agent/ai-terminal/internal/core"
)

type nvidiaProvider struct {
	baseProvider
}

func init() {
	Register("nvidia", func(cfg core.ProviderConfig) core.Provider {
		return &nvidiaProvider{
			baseProvider: newBaseProvider("nvidia", cfg.APIKey, cfg.Model, cfg.BaseURL),
		}
	})
}

func (p *nvidiaProvider) Stream(ctx context.Context, req *core.Request) (<-chan core.Token, error) {
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 16384
	}
	chatReq := map[string]any{
		"model":       p.model,
		"messages":    convertMessages(req.Messages),
		"stream":      true,
		"max_tokens":  maxTokens,
		"temperature": 1.0,
		"top_p":       1.0,
	}
	for k, v := range req.Options {
		chatReq[k] = v
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
