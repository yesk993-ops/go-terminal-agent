package tool

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/agent/ai-terminal/internal/core"
)

func TestBashToolEcho(t *testing.T) {
	r := NewRegistry()
	RegisterDefaultTools(r)

	tool, ok := r.Get("bash")
	if !ok {
		t.Fatal("expected bash tool")
	}

	result := tool.Execute(context.Background(), json.RawMessage(`{"command": "echo hello"}`))
	if result.Status == core.StatusError {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if result.Output != "hello\n" && result.Output != "hello" {
		t.Fatalf("expected 'hello', got %q", result.Output)
	}
}

func TestBashToolDestructiveDetection(t *testing.T) {
	r := NewRegistry()
	RegisterDefaultTools(r)

	tool, ok := r.Get("bash")
	if !ok {
		t.Fatal("expected bash tool")
	}

	result := tool.Execute(context.Background(), json.RawMessage(`{"command": "rm -rf /tmp/test"}`))
	if result.Status != core.StatusPending {
		t.Fatalf("expected pending status for destructive command, got %s", result.Status)
	}
}

func TestBashToolNotFound(t *testing.T) {
	result := (&bashTool{}).Execute(context.Background(), json.RawMessage(`{"command": "nonexistent_command_xyz_123"}`))
	if result.Status != core.StatusError {
		t.Fatalf("expected error for nonexistent command, got %s", result.Status)
	}
}
