package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/agent/ai-terminal/internal/core"
)

type readTool struct{}

func NewReadTool() core.Tool {
	return &readTool{}
}

func (t *readTool) Name() string { return "read" }

func (t *readTool) Description() string {
	return "Read the contents of a file. Returns the file content or an error if the file doesn't exist."
}

func (t *readTool) Schema() json.RawMessage {
	return schemaFor(map[string]any{
		"file_path": map[string]any{
			"type":        "string",
			"description": "The absolute path to the file to read",
		},
		"offset": map[string]any{
			"type":        "integer",
			"description": "Line number to start reading from (1-indexed, optional)",
		},
		"limit": map[string]any{
			"type":        "integer",
			"description": "Maximum number of lines to read (optional)",
		},
	}, []string{"file_path"})
}

func (t *readTool) Execute(ctx context.Context, args json.RawMessage) *core.ToolResult {
	var params struct {
		FilePath string `json:"file_path"`
		Offset   int    `json:"offset"`
		Limit    int    `json:"limit"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return &core.ToolResult{Status: core.StatusError, Error: "invalid arguments: " + err.Error()}
	}

	data, err := os.ReadFile(params.FilePath)
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Error: fmt.Sprintf("read file: %v", err)}
	}

	content := string(data)

	if params.Offset > 0 || params.Limit > 0 {
		lines := splitLines(content)
		start := params.Offset
		if start < 1 {
			start = 1
		}
		end := len(lines)
		if params.Limit > 0 {
			end = start + params.Limit - 1
			if end > len(lines) {
				end = len(lines)
			}
		}
		if start > len(lines) {
			return &core.ToolResult{Status: core.StatusSuccess, Output: ""}
		}
		content = joinLines(lines[start-1 : end])
	}

	return &core.ToolResult{
		Status: core.StatusSuccess,
		Output: content,
	}
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start <= len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func joinLines(lines []string) string {
	result := ""
	for i, line := range lines {
		if i > 0 {
			result += "\n"
		}
		result += line
	}
	return result
}
