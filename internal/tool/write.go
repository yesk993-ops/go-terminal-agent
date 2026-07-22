package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/agent/ai-terminal/internal/core"
)

type writeTool struct{}

func NewWriteTool() core.Tool {
	return &writeTool{}
}

func (t *writeTool) Name() string { return "write" }

func (t *writeTool) Description() string {
	return "Write content to a file. Creates the file if it doesn't exist. This is a destructive action."
}

func (t *writeTool) Schema() json.RawMessage {
	return schemaFor(map[string]any{
		"file_path": map[string]any{
			"type":        "string",
			"description": "The absolute path to the file to write",
		},
		"content": map[string]any{
			"type":        "string",
			"description": "The content to write to the file",
		},
	}, []string{"file_path", "content"})
}

func (t *writeTool) Execute(ctx context.Context, args json.RawMessage) *core.ToolResult {
	var params struct {
		FilePath string `json:"file_path"`
		Content  string `json:"content"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return &core.ToolResult{Status: core.StatusError, Error: "invalid arguments: " + err.Error()}
	}

	safePath, err := resolveSafePath(params.FilePath)
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Error: err.Error()}
	}

	dir := filepath.Dir(safePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return &core.ToolResult{Status: core.StatusError, Error: fmt.Sprintf("create directory: %v", err)}
	}

	if err := os.WriteFile(safePath, []byte(params.Content), 0644); err != nil {
		return &core.ToolResult{Status: core.StatusError, Error: fmt.Sprintf("write file: %v", err)}
	}

	return &core.ToolResult{
		Status: core.StatusSuccess,
		Output: fmt.Sprintf("Written %d bytes to %s", len(params.Content), safePath),
	}
}
