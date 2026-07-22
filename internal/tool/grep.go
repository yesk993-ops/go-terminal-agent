package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/agent/ai-terminal/internal/core"
)

type grepTool struct{}

func NewGrepTool() core.Tool {
	return &grepTool{}
}

func (t *grepTool) Name() string { return "grep" }

func (t *grepTool) Description() string {
	return "Search file contents using a regular expression pattern. Returns matching file paths and line numbers."
}

func (t *grepTool) Schema() json.RawMessage {
	return schemaFor(map[string]any{
		"pattern": map[string]any{
			"type":        "string",
			"description": "The regular expression pattern to search for",
		},
		"path": map[string]any{
			"type":        "string",
			"description": "The directory to search in (optional, defaults to current)",
		},
		"include": map[string]any{
			"type":        "string",
			"description": "File pattern to include (e.g. '*.go', '*.{ts,tsx}') (optional)",
		},
	}, []string{"pattern"})
}

func (t *grepTool) Execute(ctx context.Context, args json.RawMessage) *core.ToolResult {
	var params struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path"`
		Include string `json:"include"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return &core.ToolResult{Status: core.StatusError, Error: "invalid arguments: " + err.Error()}
	}

	if params.Path == "" {
		params.Path = "."
	}

	re, err := regexp.Compile(params.Pattern)
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Error: "invalid regex pattern: " + err.Error()}
	}

	var results []string
	err = filepath.Walk(params.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}
			return nil
		}

		if params.Include != "" {
			match, _ := filepath.Match(params.Include, filepath.Base(path))
			if !match {
				return nil
			}
		}

		// Skip files larger than 4MB to prevent memory exhaustion
		if info.Size() > 4<<20 {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if re.MatchString(line) {
				results = append(results, fmt.Sprintf("%s:%d: %s", path, i+1, strings.TrimSpace(line)))
			}
		}
		return nil
	})

	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Error: fmt.Sprintf("search error: %v", err)}
	}

	if len(results) == 0 {
		return &core.ToolResult{Status: core.StatusSuccess, Output: "No matches found"}
	}

	output := strings.Join(results, "\n")
	if len(output) > 50000 {
		runes := []rune(output)
		output = string(runes[:50000]) + "\n... (truncated)"
	}

	return &core.ToolResult{
		Status: core.StatusSuccess,
		Output: fmt.Sprintf("Found %d matches:\n%s", len(results), output),
	}
}
