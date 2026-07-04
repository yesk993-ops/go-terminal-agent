package tool

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/agent/ai-terminal/internal/core"
)

type webSearchTool struct{}

func NewWebSearchTool() core.Tool {
	return &webSearchTool{}
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

	result := fmt.Sprintf("Web search for: %s\n(Results would appear here with a real search API)", params.Query)

	return &core.ToolResult{
		Status: core.StatusSuccess,
		Output: result,
	}
}
