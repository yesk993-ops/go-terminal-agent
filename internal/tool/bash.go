package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/agent/ai-terminal/internal/core"
)

type bashTool struct{}

func NewBashTool() core.Tool {
	return &bashTool{}
}

func (t *bashTool) Name() string { return "bash" }

func (t *bashTool) Description() string {
	return "Execute a shell command and return the output. For destructive actions, user confirmation is required."
}

func (t *bashTool) Schema() json.RawMessage {
	return schemaFor(map[string]any{
		"command": map[string]any{
			"type":        "string",
			"description": "The shell command to execute",
		},
		"workdir": map[string]any{
			"type":        "string",
			"description": "Working directory for the command (optional)",
		},
		"timeout": map[string]any{
			"type":        "integer",
			"description": "Timeout in milliseconds (optional, default 30000)",
		},
	}, []string{"command"})
}

var destructivePatterns = []string{
	"rm ", "/rm", "rm -rf", "rm -r", "rm -f", "rmdir", "dd ", "/dd", "mkfs", "format",
	":(){ :|:& };:", "/dev/sd", "/dev/nvme", "/dev/mmcblk",
	"chmod 0", "chmod 644 /", "chmod 777 /",
	"chown ", "reboot", "shutdown", "poweroff", "halt",
	">|", "sudo ", "sudo\t", "pkexec",
}

func isDestructive(cmd string) bool {
	lower := strings.ToLower(strings.TrimSpace(cmd))
	for _, p := range destructivePatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

func (t *bashTool) Execute(ctx context.Context, args json.RawMessage) *core.ToolResult {
	var params struct {
		Command string `json:"command"`
		Workdir string `json:"workdir"`
		Timeout int    `json:"timeout"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return &core.ToolResult{Status: core.StatusError, Error: "invalid arguments: " + err.Error()}
	}

	if isDestructive(params.Command) {
		return &core.ToolResult{
			Status: core.StatusPending,
			Output: fmt.Sprintf("Destructive action detected. Command requires confirmation:\n$ %s", params.Command),
		}
	}

	if params.Timeout <= 0 {
		params.Timeout = 30000
	}

	cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(params.Timeout)*time.Millisecond)
	defer cancel()

	shell := "sh"
	flag := "-c"

	cmd := exec.CommandContext(cmdCtx, shell, flag, params.Command)

	if params.Workdir != "" {
		cmd.Dir = params.Workdir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	var output strings.Builder
	if stdout.Len() > 0 {
		output.WriteString(stdout.String())
	}
	if stderr.Len() > 0 {
		if output.Len() > 0 {
			output.WriteString("\n")
		}
		output.WriteString("stderr: " + stderr.String())
	}

	if err != nil {
		if cmdCtx.Err() == context.DeadlineExceeded {
			return &core.ToolResult{
				Status: core.StatusError,
				Output: output.String(),
				Error:  fmt.Sprintf("command timed out after %dms", params.Timeout),
			}
		}
		if cmdCtx.Err() == context.Canceled {
			return &core.ToolResult{
				Status: core.StatusError,
				Output: output.String(),
				Error:  "command was cancelled",
			}
		}
		return &core.ToolResult{
			Status: core.StatusError,
			Output: output.String(),
			Error:  fmt.Sprintf("command failed: %v", err),
		}
	}

	return &core.ToolResult{
		Status: core.StatusSuccess,
		Output: output.String(),
	}
}
