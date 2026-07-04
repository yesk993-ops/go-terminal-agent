package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agent/ai-terminal/internal/core"
)

type globTool struct{}

func NewGlobTool() core.Tool {
	return &globTool{}
}

func (t *globTool) Name() string { return "glob" }

func (t *globTool) Description() string {
	return "Find files matching a glob pattern (e.g. '**/*.go', 'src/**/*.ts')."
}

func (t *globTool) Schema() json.RawMessage {
	return schemaFor(map[string]any{
		"pattern": map[string]any{
			"type":        "string",
			"description": "The glob pattern to match (e.g. '**/*.go')",
		},
		"path": map[string]any{
			"type":        "string",
			"description": "The directory to search in (optional, defaults to current)",
		},
	}, []string{"pattern"})
}

func (t *globTool) Execute(ctx context.Context, args json.RawMessage) *core.ToolResult {
	var params struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return &core.ToolResult{Status: core.StatusError, Error: "invalid arguments: " + err.Error()}
	}

	if params.Path == "" {
		params.Path = "."
	}

	var results []string
	err := filepath.Walk(params.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}
			return nil
		}
		match, err := filepath.Match(params.Pattern, filepath.Base(path))
		if err != nil {
			return nil
		}
		if match {
			results = append(results, path)
		}
		return nil
	})

	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Error: fmt.Sprintf("glob error: %v", err)}
	}

	if len(results) == 0 {
		return &core.ToolResult{Status: core.StatusSuccess, Output: "No files found matching pattern"}
	}

	output := strings.Join(results, "\n")
	return &core.ToolResult{
		Status: core.StatusSuccess,
		Output: fmt.Sprintf("Found %d files:\n%s", len(results), output),
	}
}
