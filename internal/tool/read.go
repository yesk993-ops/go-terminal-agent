package tool

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

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

	safePath, err := resolveSafePath(params.FilePath)
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Error: err.Error()}
	}

	if params.Offset <= 0 && params.Limit <= 0 {
		data, err := os.ReadFile(safePath)
		if err != nil {
			return &core.ToolResult{Status: core.StatusError, Error: fmt.Sprintf("read file: %v", err)}
		}
		return &core.ToolResult{Status: core.StatusSuccess, Output: string(data)}
	}

	content, err := readLineRange(ctx, safePath, params.Offset, params.Limit)
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Error: err.Error()}
	}
	return &core.ToolResult{Status: core.StatusSuccess, Output: content}
}

// readLineRange scans only the requested portion instead of allocating and
// splitting an entire file when the caller asks for a small window.
func readLineRange(ctx context.Context, path string, offset, limit int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}
	defer f.Close()

	start := offset
	if start < 1 {
		start = 1
	}
	end := int(^uint(0) >> 1)
	if limit > 0 {
		end = start + limit - 1
	}

	reader := bufio.NewReaderSize(f, 64*1024)
	var out strings.Builder
	lineNo := 0
	selected := 0
	lastEndedWithNewline := false
	appendLine := func(line string) {
		if lineNo < start || lineNo > end {
			return
		}
		line = strings.TrimSuffix(line, "\n")
		if selected > 0 {
			out.WriteByte('\n')
		}
		out.WriteString(line)
		selected++
	}
	for {
		if err := ctx.Err(); err != nil {
			return "", fmt.Errorf("read file: %w", err)
		}
		line, readErr := reader.ReadString('\n')
		if len(line) > 0 {
			lastEndedWithNewline = strings.HasSuffix(line, "\n")
			lineNo++
			appendLine(line)
			if lineNo >= end {
				break
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				// splitLines, the historical implementation, considers a final
				// newline to create one trailing empty logical line. Preserve that
				// exact range behavior without reading the whole file.
				if len(line) == 0 && lastEndedWithNewline {
					lineNo++
					appendLine("")
				}
				break
			}
			return "", fmt.Errorf("read file: %w", readErr)
		}
	}
	return out.String(), nil
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
	return strings.Join(lines, "\n")
}
