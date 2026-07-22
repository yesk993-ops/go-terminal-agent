package provider

import (
	"context"
	"fmt"

	"github.com/agent/ai-terminal/internal/core"
)

type geminiProvider struct {
	baseProvider
	apiKey string
}

func init() {
	Register("gemini", func(cfg core.ProviderConfig) core.Provider {
		// Gemini uses X-Goog-Api-Key header, not Bearer auth.
		// Pass the API key through extra headers to avoid double-auth.
		return &geminiProvider{
			baseProvider: newBaseProvider("gemini", "", cfg.Model, cfg.BaseURL),
			apiKey:       cfg.APIKey,
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

	url := fmt.Sprintf("%s/models/%s:streamGenerateContent?alt=sse",
		p.baseURL, p.model)

	headers := map[string]string{
		"X-Goog-Api-Key": p.apiKey,
	}

	resp, err := p.doPost(ctx, url, chatReq, headers)
	if err != nil {
		return nil, err
	}

	return streamGeminiSSE(ctx, resp)
}
