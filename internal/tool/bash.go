package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
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
	return "Execute a shell command and return the output. For destructive actions, user confirmation is required. On Windows uses cmd.exe (or PowerShell if available); on Unix uses sh."
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

// destructivePatterns covers both Unix and Windows destructive operations.
var destructivePatterns = []string{
	// Unix
	"rm ", "/rm", "rm -rf", "rm -r", "rm -f", "rmdir", "dd ", "/dd", "mkfs", "format",
	":(){ :|:& };:", "/dev/sd", "/dev/nvme", "/dev/mmcblk",
	"chmod 0", "chmod 644 /", "chmod 777 /",
	"chown ", "reboot", "shutdown", "poweroff", "halt",
	">|", "sudo ", "sudo\t", "pkexec",
	// Windows
	"del /", "del /f", "del /s", "rd /s", "rmdir /s", "format ",
	"diskpart", "remove-item ", "ri -recurse", "ri -r",
	"clear-disk", "reset-computer", "stop-computer", "restart-computer",
	"format-volume", "clear-content c:", "erase ",
	"reg delete", "net user", "takeown ",
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

// shellCommand returns the platform shell executable and the flag used to pass
// a command string. Preference order on Windows:
//  1. ComSpec (usually cmd.exe)
//  2. powershell.exe
//  3. pwsh.exe (PowerShell Core)
//  4. bare "cmd"
//
// On Unix: sh -c
func shellCommand(command string) (name string, args []string) {
	if runtime.GOOS == "windows" {
		// Prefer cmd.exe via ComSpec — always present on Windows.
		if comspec := os.Getenv("ComSpec"); comspec != "" {
			return comspec, []string{"/C", command}
		}
		// Fall back to PowerShell if ComSpec is unset (rare).
		if pwsh, err := exec.LookPath("powershell.exe"); err == nil {
			return pwsh, []string{"-NoProfile", "-NonInteractive", "-Command", command}
		}
		if pwsh, err := exec.LookPath("pwsh.exe"); err == nil {
			return pwsh, []string{"-NoProfile", "-NonInteractive", "-Command", command}
		}
		if pwsh, err := exec.LookPath("pwsh"); err == nil {
			return pwsh, []string{"-NoProfile", "-NonInteractive", "-Command", command}
		}
		return "cmd", []string{"/C", command}
	}
	return "sh", []string{"-c", command}
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

	shell, shellArgs := shellCommand(params.Command)
	cmd := exec.CommandContext(cmdCtx, shell, shellArgs...)

	if params.Workdir != "" {
		cmd.Dir = params.Workdir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	var output strings.Builder
	outStr := stdout.String()
	errStr := stderr.String()

	// Truncate excessively large outputs to prevent memory issues.
	const maxOutput = 50000
	if len(outStr) > maxOutput {
		outStr = outStr[:maxOutput] + "\n... (stdout truncated)"
	}
	if len(errStr) > maxOutput {
		errStr = errStr[:maxOutput] + "\n... (stderr truncated)"
	}

	if len(outStr) > 0 {
		output.WriteString(outStr)
	}
	if len(errStr) > 0 {
		if output.Len() > 0 {
			output.WriteString("\n")
		}
		output.WriteString("stderr: " + errStr)
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
