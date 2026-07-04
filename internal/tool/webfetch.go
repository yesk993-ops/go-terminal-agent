package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/agent/ai-terminal/internal/core"
)

type webFetchTool struct{}

func NewWebFetchTool() core.Tool {
	return &webFetchTool{}
}

func (t *webFetchTool) Name() string { return "web_fetch" }

func (t *webFetchTool) Description() string {
	return "Fetch the content of a URL. Useful for reading documentation, articles, or API responses."
}

func (t *webFetchTool) Schema() json.RawMessage {
	return schemaFor(map[string]any{
		"url": map[string]any{
			"type":        "string",
			"description": "The URL to fetch",
		},
		"timeout": map[string]any{
			"type":        "integer",
			"description": "Timeout in seconds (optional, default 30)",
		},
	}, []string{"url"})
}

func (t *webFetchTool) Execute(ctx context.Context, args json.RawMessage) *core.ToolResult {
	var params struct {
		URL     string `json:"url"`
		Timeout int    `json:"timeout"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return &core.ToolResult{Status: core.StatusError, Error: "invalid arguments: " + err.Error()}
	}

	if params.Timeout <= 0 {
		params.Timeout = 30
	}

	fetchCtx, cancel := context.WithTimeout(ctx, time.Duration(params.Timeout)*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, params.URL, nil)
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Error: fmt.Sprintf("create request: %v", err)}
	}

	req.Header.Set("User-Agent", "GoAgent/1.0")
	req.Header.Set("Accept", "text/html,text/plain,*/*")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Error: fmt.Sprintf("fetch URL: %v", err)}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Error: fmt.Sprintf("read body: %v", err)}
	}

	if resp.StatusCode != http.StatusOK {
		return &core.ToolResult{
			Status: core.StatusError,
			Output: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)),
			Error:  fmt.Sprintf("unexpected status code: %d", resp.StatusCode),
		}
	}

	content := string(body)
	if len(content) > 100000 {
		runes := []rune(content)
		content = string(runes[:100000]) + "\n... (truncated at 100KB)"
	}

	return &core.ToolResult{
		Status: core.StatusSuccess,
		Output: fmt.Sprintf("Content from %s (%d bytes):\n%s", params.URL, len(body), content),
	}
}
