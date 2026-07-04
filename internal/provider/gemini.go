package provider

import (
	"context"
	"fmt"

	"github.com/agent/ai-terminal/internal/core"
)

type geminiProvider struct {
	baseProvider
}

func init() {
	Register("gemini", func(cfg core.ProviderConfig) core.Provider {
		return &geminiProvider{
			baseProvider: newBaseProvider("gemini", cfg.APIKey, cfg.Model, cfg.BaseURL),
		}
	})
}

type geminiContent struct {
	Role  string `json:"role"`
	Parts []struct {
		Text string `json:"text"`
	} `json:"parts"`
}

func (p *geminiProvider) Stream(ctx context.Context, req *core.Request) (<-chan core.Token, error) {
	contents := make([]geminiContent, 0, len(req.Messages))
	for _, m := range req.Messages {
		if m.Role == core.RoleSystem {
			continue
		}
		role := "user"
		if m.Role == core.RoleAssistant {
			role = "model"
		}
		contents = append(contents, geminiContent{
			Role: role,
			Parts: []struct {
				Text string `json:"text"`
			}{{Text: m.Content}},
		})
	}

	var systemInstruction string
	for _, m := range req.Messages {
		if m.Role == core.RoleSystem {
			systemInstruction = m.Content
			break
		}
	}

	chatReq := map[string]any{
		"contents": contents,
	}

	if systemInstruction != "" {
		chatReq["system_instruction"] = map[string]any{
			"parts": []map[string]any{{"text": systemInstruction}},
		}
	}

	url := fmt.Sprintf("%s/models/%s:streamGenerateContent?alt=sse&key=%s",
		p.baseURL, p.model, p.apiKey)

	resp, err := p.doPost(ctx, url, chatReq, nil)
	if err != nil {
		return nil, err
	}

	return streamGeminiSSE(ctx, resp)
}
