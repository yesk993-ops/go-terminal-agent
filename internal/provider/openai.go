package provider

import (
	"context"

	"github.com/agent/ai-terminal/internal/core"
)

type openAIProvider struct {
	baseProvider
}

func init() {
	Register("openai", func(cfg core.ProviderConfig) core.Provider {
		return &openAIProvider{
			baseProvider: newBaseProvider("openai", cfg.APIKey, cfg.Model, cfg.BaseURL),
		}
	})
}

func (p *openAIProvider) Stream(ctx context.Context, req *core.Request) (<-chan core.Token, error) {
	temperature := 0.7
	if t, ok := req.Options["temperature"]; ok {
		temperature = toFloat64(t)
	}

	chatReq := map[string]any{
		"model":       p.model,
		"messages":    convertMessages(req.Messages),
		"stream":      true,
		"max_tokens":  req.MaxTokens,
		"temperature": temperature,
	}

	for k, v := range req.Options {
		if k != "temperature" {
			chatReq[k] = v
		}
	}

	if len(req.Tools) > 0 {
		chatReq["tools"] = convertTools(req.Tools)
	}

	resp, err := p.doPost(ctx, p.baseURL+"/chat/completions", chatReq, nil)
	if err != nil {
		return nil, err
	}

	return streamOpenAICompatibleSSE(ctx, resp)
}
