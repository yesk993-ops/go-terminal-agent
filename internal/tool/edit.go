package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/agent/ai-terminal/internal/core"
)

type editTool struct{}

func NewEditTool() core.Tool {
	return &editTool{}
}

func (t *editTool) Name() string { return "edit" }

func (t *editTool) Description() string {
	return "Edit a file by performing an exact string replacement. This is a destructive action."
}

func (t *editTool) Schema() json.RawMessage {
	return schemaFor(map[string]any{
		"file_path": map[string]any{
			"type":        "string",
			"description": "The absolute path to the file to edit",
		},
		"old_string": map[string]any{
			"type":        "string",
			"description": "The existing text to replace",
		},
		"new_string": map[string]any{
			"type":        "string",
			"description": "The new text to insert",
		},
	}, []string{"file_path", "old_string", "new_string"})
}

func (t *editTool) Execute(ctx context.Context, args json.RawMessage) *core.ToolResult {
	var params struct {
		FilePath  string `json:"file_path"`
		OldString string `json:"old_string"`
		NewString string `json:"new_string"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return &core.ToolResult{Status: core.StatusError, Error: "invalid arguments: " + err.Error()}
	}

	safePath, err := resolveSafePath(params.FilePath)
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Error: err.Error()}
	}

	data, err := os.ReadFile(safePath)
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Error: fmt.Sprintf("read file: %v", err)}
	}

	content := string(data)
	if !strings.Contains(content, params.OldString) {
		return &core.ToolResult{Status: core.StatusError, Error: "old_string not found in file"}
	}

	newContent := strings.Replace(content, params.OldString, params.NewString, 1)

	if err := os.WriteFile(safePath, []byte(newContent), 0644); err != nil {
		return &core.ToolResult{Status: core.StatusError, Error: fmt.Sprintf("write file: %v", err)}
	}

	return &core.ToolResult{
		Status: core.StatusSuccess,
		Output: fmt.Sprintf("Edited %s (replaced %d chars with %d chars)", safePath, len(params.OldString), len(params.NewString)),
	}
}
