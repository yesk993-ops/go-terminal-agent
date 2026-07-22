package provider

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/agent/ai-terminal/internal/core"
	"github.com/agent/ai-terminal/internal/logger"
)

var (
	mu        sync.RWMutex
	registry  = make(map[string]Factory)
	providers = make(map[string]core.Provider)
)

type Factory func(cfg core.ProviderConfig) core.Provider

func Register(name string, factory Factory) {
	mu.Lock()
	defer mu.Unlock()
	registry[name] = factory
}

func Get(name string, cfg core.ProviderConfig) core.Provider {
	// The client holds credentials and a base URL, so model alone is not a
	// sufficient identity. Hashing the key avoids retaining it verbatim in a
	// global map key while preventing accidental client reuse across accounts.
	keyHash := sha256.Sum256([]byte(cfg.APIKey))
	cacheKey := name + ":" + cfg.Model + ":" + cfg.BaseURL + ":" + hex.EncodeToString(keyHash[:])

	mu.RLock()
	if p, ok := providers[cacheKey]; ok {
		mu.RUnlock()
		return p
	}
	mu.RUnlock()

	mu.Lock()
	defer mu.Unlock()
	if p, ok := providers[cacheKey]; ok {
		return p
	}

	factory, ok := registry[name]
	if !ok {
		return nil
	}

	p := factory(cfg)
	providers[cacheKey] = p
	return p
}

func ListAvailable() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

type baseProvider struct {
	name    string
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

// httpError preserves status and Retry-After information so fallback policy
// can react immediately to a known rate limit instead of parsing strings and
// repeatedly sleeping on a provider that cannot serve the request.
type httpError struct {
	StatusCode int
	Body       string
	RetryAfter time.Duration
}

func (e *httpError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("API error (status %d)", e.StatusCode)
	}
	return fmt.Sprintf("API error (status %d): %s", e.StatusCode, e.Body)
}

func retryAfterFromHeader(value string) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(value); err == nil {
		if delay := time.Until(when); delay > 0 {
			return delay
		}
	}
	return 0
}

func asHTTPError(err error) (*httpError, bool) {
	var httpErr *httpError
	if errors.As(err, &httpErr) {
		return httpErr, true
	}
	return nil, false
}

func newBaseProvider(name, apiKey, model, baseURL string) baseProvider {
	if baseURL == "" {
		baseURL = defaultBaseURL(name)
	}
	if model == "" {
		model = defaultModel(name)
	}
	return baseProvider{
		name:    name,
		apiKey:  apiKey,
		model:   model,
		baseURL: strings.TrimRight(baseURL, "/"),
		client: &http.Client{
			Timeout: 120 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        20,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
				DisableCompression:  false,
				ForceAttemptHTTP2:   true,
			},
		},
	}
}

func (b *baseProvider) Name() string { return b.name }

func defaultBaseURL(name string) string {
	switch name {
	case "openai":
		return "https://api.openai.com/v1"
	case "anthropic":
		return "https://api.anthropic.com/v1"
	case "gemini":
		return "https://generativelanguage.googleapis.com/v1beta"
	case "groq":
		return "https://api.groq.com/openai/v1"
	case "nvidia":
		return "https://integrate.api.nvidia.com/v1"
	case "openrouter":
		return "https://openrouter.ai/api/v1"
	default:
		return ""
	}
}

func defaultModel(name string) string {
	switch name {
	case "openai":
		return "gpt-4o"
	case "anthropic":
		return "claude-sonnet-4-20250514"
	case "gemini":
		return "gemini-2.5-pro"
	case "groq":
		return "llama-3.3-70b-versatile"
	case "nvidia":
		return "meta/llama-3.1-8b-instruct"
	case "openrouter":
		return "openrouter/auto"
	default:
		return ""
	}
}

type openAIToolCallDelta struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function *struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type openAIDelta struct {
	Content   string                `json:"content"`
	ToolCalls []openAIToolCallDelta `json:"tool_calls"`
}

type openAIChoice struct {
	Delta        openAIDelta `json:"delta"`
	FinishReason *string     `json:"finish_reason"`
}

type openAIChunk struct {
	Choices []openAIChoice `json:"choices"`
}

type accumulatedToolCall struct {
	id      string
	name    string
	argsBuf strings.Builder
}

func watchStreamCancellation(ctx context.Context, body io.Closer) func() {
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = body.Close()
		case <-done:
		}
	}()
	return func() { close(done) }
}

func sendStreamToken(ctx context.Context, tokenCh chan<- core.Token, token core.Token) bool {
	select {
	case tokenCh <- token:
		return true
	case <-ctx.Done():
		return false
	}
}

func streamOpenAICompatibleSSE(ctx context.Context, resp *http.Response) (<-chan core.Token, error) {
	tokenCh := make(chan core.Token, 64)

	go func() {
		defer close(tokenCh)
		defer resp.Body.Close()
		stopWatching := watchStreamCancellation(ctx, resp.Body)
		defer stopWatching()

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)

		var pending []*accumulatedToolCall

		for scanner.Scan() {
			line := scanner.Text()

			if err := ctx.Err(); err != nil {
				sendStreamToken(ctx, tokenCh, core.Token{Error: core.ErrContextCancelled, Done: true})
				return
			}

			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				if !flushPendingToolCalls(ctx, &pending, tokenCh) {
					return
				}
				sendStreamToken(ctx, tokenCh, core.Token{Done: true})
				return
			}

			var chunk openAIChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				logger.L().Debug("SSE parse error", "error", err, "data", data[:min(len(data), 200)])
				continue
			}

			if len(chunk.Choices) == 0 {
				continue
			}

			choice := chunk.Choices[0]

			for _, tc := range choice.Delta.ToolCalls {
				accumulateToolCall(&pending, tc)
			}

			if choice.FinishReason != nil && *choice.FinishReason != "" {
				if *choice.FinishReason == "tool_calls" && !flushPendingToolCalls(ctx, &pending, tokenCh) {
					return
				}
				sendStreamToken(ctx, tokenCh, core.Token{Done: true})
				return
			}

			if choice.Delta.Content != "" {
				if len(pending) > 0 && !flushPendingToolCalls(ctx, &pending, tokenCh) {
					return
				}
				if !sendStreamToken(ctx, tokenCh, core.Token{Content: choice.Delta.Content}) {
					return
				}
			}
		}

		if !flushPendingToolCalls(ctx, &pending, tokenCh) {
			return
		}
		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			logger.L().Debug("SSE scan error", "error", err)
			sendStreamToken(ctx, tokenCh, core.Token{Error: err, Done: true})
		}
	}()

	return tokenCh, nil
}

func accumulateToolCall(pending *[]*accumulatedToolCall, tc openAIToolCallDelta) {
	if tc.ID != "" {
		atc := &accumulatedToolCall{id: tc.ID}
		if tc.Function != nil {
			atc.name = tc.Function.Name
			atc.argsBuf.WriteString(tc.Function.Arguments)
		}
		*pending = append(*pending, atc)
		return
	}

	if len(*pending) > 0 && tc.Function != nil {
		last := (*pending)[len(*pending)-1]
		last.argsBuf.WriteString(tc.Function.Arguments)
		if tc.Function.Name != "" {
			last.name = tc.Function.Name
		}
	}
}

func flushPendingToolCalls(ctx context.Context, pending *[]*accumulatedToolCall, tokenCh chan<- core.Token) bool {
	for _, atc := range *pending {
		if atc.id == "" || atc.name == "" {
			continue
		}
		if !sendStreamToken(ctx, tokenCh, core.Token{
			ToolCall: &core.ToolCall{
				ID:   atc.id,
				Name: atc.name,
				Args: []byte(atc.argsBuf.String()),
			},
		}) {
			return false
		}
	}
	*pending = nil
	return true
}

func streamGeminiSSE(ctx context.Context, resp *http.Response) (<-chan core.Token, error) {
	tokenCh := make(chan core.Token, 64)

	go func() {
		defer close(tokenCh)
		defer resp.Body.Close()
		stopWatching := watchStreamCancellation(ctx, resp.Body)
		defer stopWatching()

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)

		for scanner.Scan() {
			line := scanner.Text()

			if err := ctx.Err(); err != nil {
				sendStreamToken(ctx, tokenCh, core.Token{Error: core.ErrContextCancelled, Done: true})
				return
			}

			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				sendStreamToken(ctx, tokenCh, core.Token{Done: true})
				return
			}

			var chunk struct {
				Candidates []struct {
					Content struct {
						Parts []struct {
							Text string `json:"text"`
						} `json:"parts"`
					} `json:"content"`
					FinishReason string `json:"finishReason"`
				} `json:"candidates"`
			}

			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			if len(chunk.Candidates) == 0 {
				continue
			}

			candidate := chunk.Candidates[0]
			if candidate.FinishReason != "" {
				sendStreamToken(ctx, tokenCh, core.Token{Done: true})
				return
			}

			for _, part := range candidate.Content.Parts {
				if part.Text != "" && !sendStreamToken(ctx, tokenCh, core.Token{Content: part.Text}) {
					return
				}
			}
		}

		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			logger.L().Debug("Gemini SSE scan error", "error", err)
			sendStreamToken(ctx, tokenCh, core.Token{Error: err, Done: true})
		}
	}()

	return tokenCh, nil
}

func streamAnthropicSSE(ctx context.Context, resp *http.Response) (<-chan core.Token, error) {
	tokenCh := make(chan core.Token, 64)

	go func() {
		defer close(tokenCh)
		defer resp.Body.Close()
		stopWatching := watchStreamCancellation(ctx, resp.Body)
		defer stopWatching()

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)

		for scanner.Scan() {
			line := scanner.Text()

			if err := ctx.Err(); err != nil {
				sendStreamToken(ctx, tokenCh, core.Token{Error: core.ErrContextCancelled, Done: true})
				return
			}

			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			var event struct {
				Type  string `json:"type"`
				Delta struct {
					Text string `json:"text"`
				} `json:"delta"`
				ContentBlock struct {
					Text string `json:"text"`
				} `json:"content_block"`
			}

			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			switch event.Type {
			case "content_block_delta":
				if event.Delta.Text != "" && !sendStreamToken(ctx, tokenCh, core.Token{Content: event.Delta.Text}) {
					return
				}
			case "content_block_start":
				if event.ContentBlock.Text != "" && !sendStreamToken(ctx, tokenCh, core.Token{Content: event.ContentBlock.Text}) {
					return
				}
			case "message_stop":
				sendStreamToken(ctx, tokenCh, core.Token{Done: true})
				return
			}
		}

		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			logger.L().Debug("Anthropic SSE error", "error", err)
			sendStreamToken(ctx, tokenCh, core.Token{Error: err, Done: true})
			return
		}
		if ctx.Err() == nil {
			sendStreamToken(ctx, tokenCh, core.Token{Done: true})
		}
	}()

	return tokenCh, nil
}

type chatMessage struct {
	Role       string `json:"role"`
	Content    any    `json:"content"`
	ToolCallID string `json:"tool_call_id,omitempty"`
	Name       string `json:"name,omitempty"`
}

func convertMessages(msgs []core.Message) []chatMessage {
	result := make([]chatMessage, 0, len(msgs))
	for _, m := range msgs {
		cm := chatMessage{
			Role:       string(m.Role),
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
			Name:       m.Name,
		}
		result = append(result, cm)
	}
	return result
}

func toFloat64(v any) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case json.Number:
		f, _ := val.Float64()
		return f
	default:
		return 0.7
	}
}

func convertTools(tools []core.ToolDef) []map[string]any {
	out := make([]map[string]any, 0, len(tools))
	for _, t := range tools {
		var schema map[string]any
		if err := json.Unmarshal(t.InputSchema, &schema); err != nil {
			schema = map[string]any{}
		}
		out = append(out, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        t.Name,
				"description": t.Description,
				"parameters":  schema,
			},
		})
	}
	return out
}

func (b *baseProvider) doPost(ctx context.Context, url string, body any, extraHeaders map[string]string) (*http.Response, error) {
	return doPostClient(ctx, url, b.apiKey, body, extraHeaders, b.client)
}

func doPost(ctx context.Context, url, apiKey string, body any, extraHeaders map[string]string) (*http.Response, error) {
	return doPostClient(ctx, url, apiKey, body, extraHeaders, http.DefaultClient)
}

func doPostClient(ctx context.Context, url, apiKey string, body any, extraHeaders map[string]string, client *http.Client) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("User-Agent", "go-terminal-agent/1.0")

	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		retryAfter := retryAfterFromHeader(resp.Header.Get("Retry-After"))
		resp.Body.Close()
		return nil, &httpError{
			StatusCode: resp.StatusCode,
			Body:       string(respBody),
			RetryAfter: retryAfter,
		}
	}

	return resp, nil
}
