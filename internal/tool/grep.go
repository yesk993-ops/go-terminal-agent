package tool

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/agent/ai-terminal/internal/core"
)

const (
	maxGrepFileSize = 4 << 20
	maxGrepMatches  = 5000
	maxGrepOutput   = 50000
)

var errGrepLimitReached = errors.New("grep result limit reached")

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
	if params.Include != "" {
		if _, err := filepath.Match(params.Include, "example"); err != nil {
			return &core.ToolResult{Status: core.StatusError, Error: "invalid include pattern: " + err.Error()}
		}
	}

	var output strings.Builder
	matches := 0
	truncated := false
	err = filepath.WalkDir(params.Path, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			if strings.HasPrefix(entry.Name(), ".") && entry.Name() != "." {
				return filepath.SkipDir
			}
			return nil
		}
		if matches >= maxGrepMatches || output.Len() >= maxGrepOutput {
			truncated = true
			return filepath.SkipAll
		}

		if params.Include != "" {
			match, _ := filepath.Match(params.Include, filepath.Base(path))
			if !match {
				return nil
			}
		}

		info, err := entry.Info()
		if err != nil || info.Size() > maxGrepFileSize {
			return nil
		}
		if err := grepFile(ctx, path, re, &output, &matches); err != nil {
			if errors.Is(err, errGrepLimitReached) {
				truncated = true
				return filepath.SkipAll
			}
			if err == context.Canceled || err == context.DeadlineExceeded {
				return err
			}
			return nil
		}
		if matches >= maxGrepMatches || output.Len() >= maxGrepOutput {
			truncated = true
			return filepath.SkipAll
		}
		return nil
	})

	if err != nil {
		if err == context.Canceled || err == context.DeadlineExceeded {
			return &core.ToolResult{Status: core.StatusError, Error: fmt.Sprintf("search cancelled: %v", err)}
		}
		return &core.ToolResult{Status: core.StatusError, Error: fmt.Sprintf("search error: %v", err)}
	}

	if matches == 0 {
		return &core.ToolResult{Status: core.StatusSuccess, Output: "No matches found"}
	}
	if truncated {
		output.WriteString("... (search truncated; narrow the path or pattern for more results)\n")
	}
	return &core.ToolResult{
		Status: core.StatusSuccess,
		Output: fmt.Sprintf("Found %d matches%s:\n%s", matches, map[bool]string{true: " (partial)", false: ""}[truncated], output.String()),
	}
}

func grepFile(ctx context.Context, path string, re *regexp.Regexp, output *strings.Builder, matches *int) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		lineNo++
		line := scanner.Text()
		if !re.MatchString(line) {
			continue
		}
		formatted := fmt.Sprintf("%s:%d: %s\n", path, lineNo, strings.TrimSpace(line))
		if *matches >= maxGrepMatches || output.Len()+len(formatted) > maxGrepOutput {
			return errGrepLimitReached
		}
		output.WriteString(formatted)
		(*matches)++
	}
	return scanner.Err()
}
