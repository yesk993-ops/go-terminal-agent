package provider

import (
	"context"

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

	temperature := 1.0
	if t, ok := req.Options["temperature"]; ok {
		temperature = toFloat64(t)
	}

	chatReq := map[string]any{
		"model":      p.model,
		"messages":   convertMessages(req.Messages),
		"stream":     true,
		"max_tokens": maxTokens,
	}

	for k, v := range req.Options {
		if k != "temperature" {
			chatReq[k] = v
		}
	}

	chatReq["temperature"] = temperature

	if len(req.Tools) > 0 {
		chatReq["tools"] = convertTools(req.Tools)
	}

	resp, err := p.doPost(ctx, p.baseURL+"/chat/completions", chatReq, nil)
	if err != nil {
		return nil, err
	}

	return streamOpenAICompatibleSSE(ctx, resp)
}
