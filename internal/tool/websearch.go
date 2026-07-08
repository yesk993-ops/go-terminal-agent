package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/agent/ai-terminal/internal/core"
)

type webSearchTool struct {
	client *http.Client
}

func NewWebSearchTool() core.Tool {
	return &webSearchTool{
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (t *webSearchTool) Name() string { return "web_search" }

func (t *webSearchTool) Description() string {
	return "Search the web for information. Returns search results with snippets and URLs."
}

func (t *webSearchTool) Schema() json.RawMessage {
	return schemaFor(map[string]any{
		"query": map[string]any{
			"type":        "string",
			"description": "The search query",
		},
		"count": map[string]any{
			"type":        "integer",
			"description": "Number of results to return (optional, default 5)",
		},
	}, []string{"query"})
}

func (t *webSearchTool) Execute(ctx context.Context, args json.RawMessage) *core.ToolResult {
	var params struct {
		Query string `json:"query"`
		Count int    `json:"count"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return &core.ToolResult{Status: core.StatusError, Error: "invalid arguments: " + err.Error()}
	}

	if params.Count <= 0 {
		params.Count = 5
	}
	if params.Count > 20 {
		params.Count = 20
	}

	result, err := t.searchDDG(ctx, params.Query, params.Count)
	if err != nil {
		return &core.ToolResult{
			Status: core.StatusError,
			Error:  fmt.Sprintf("web search failed: %v", err),
		}
	}

	return &core.ToolResult{
		Status: core.StatusSuccess,
		Output: result,
	}
}

func (t *webSearchTool) searchDDG(ctx context.Context, query string, count int) (string, error) {
	u := fmt.Sprintf("https://api.duckduckgo.com/?q=%s&format=json&no_html=1&skip_disambig=1",
		url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "go-terminal-agent/1.0")

	resp, err := t.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var ddg struct {
		AbstractText string `json:"AbstractText"`
		AbstractURL  string `json:"AbstractURL"`
		Answer       string `json:"Answer"`
		Results      []struct {
			Text     string `json:"Text"`
			FirstURL string `json:"FirstURL"`
		} `json:"Results"`
		RelatedTopics []struct {
			Text     string `json:"Text"`
			FirstURL string `json:"FirstURL"`
			Topics   []struct {
				Text     string `json:"Text"`
				FirstURL string `json:"FirstURL"`
			} `json:"Topics"`
		} `json:"RelatedTopics"`
	}

	if err := json.Unmarshal(body, &ddg); err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Web search results for: %s\n\n", query))

	if ddg.Answer != "" {
		b.WriteString("Answer: ")
		b.WriteString(ddg.Answer)
		b.WriteString("\n\n")
	}

	if ddg.AbstractText != "" {
		b.WriteString("Summary: ")
		b.WriteString(ddg.AbstractText)
		if ddg.AbstractURL != "" {
			b.WriteString("\nSource: ")
			b.WriteString(ddg.AbstractURL)
		}
		b.WriteString("\n\n")
	}

	n := 0
	for _, r := range ddg.Results {
		if n >= count {
			break
		}
		n++
		b.WriteString(fmt.Sprintf("%d. %s\n   %s\n\n", n, r.Text, r.FirstURL))
	}

	for _, rt := range ddg.RelatedTopics {
		if n >= count {
			break
		}
		if rt.Text != "" {
			n++
			b.WriteString(fmt.Sprintf("%d. %s\n   %s\n\n", n, rt.Text, rt.FirstURL))
		}
		for _, t2 := range rt.Topics {
			if n >= count {
				break
			}
			n++
			b.WriteString(fmt.Sprintf("%d. %s\n   %s\n\n", n, t2.Text, t2.FirstURL))
		}
	}

	if n == 0 {
		b.WriteString("No results found.")
	}

	return b.String(), nil
}
