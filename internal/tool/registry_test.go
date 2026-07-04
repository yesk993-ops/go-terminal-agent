package tool

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/agent/ai-terminal/internal/core"
)

func TestRegistry(t *testing.T) {
	r := NewRegistry()
	RegisterDefaultTools(r)

	tools := r.List()
	if len(tools) == 0 {
		t.Fatal("expected tools to be registered")
	}

	defs := r.Definitions()
	if len(defs) == 0 {
		t.Fatal("expected tool definitions")
	}

	foundNames := make(map[string]bool)
	for _, def := range defs {
		foundNames[def.Name] = true
	}

	expected := []string{"read", "write", "edit", "grep", "glob", "bash", "web_search", "web_fetch"}
	for _, name := range expected {
		if !foundNames[name] {
			t.Fatalf("expected tool %q to be registered", name)
		}
	}
}

func TestReadTool(t *testing.T) {
	r := NewRegistry()
	RegisterDefaultTools(r)

	tool, ok := r.Get("read")
	if !ok {
		t.Fatal("expected read tool")
	}

	result := tool.Execute(context.Background(), json.RawMessage(`{"file_path": "registry_test.go"}`))
	if result.Status == core.StatusError {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if result.Output == "" {
		t.Fatal("expected non-empty output")
	}
}

func TestGlobTool(t *testing.T) {
	r := NewRegistry()
	RegisterDefaultTools(r)

	tool, ok := r.Get("glob")
	if !ok {
		t.Fatal("expected glob tool")
	}

	result := tool.Execute(context.Background(), json.RawMessage(`{"pattern": "*.go", "path": "."}`))
	if result.Status == core.StatusError {
		t.Fatalf("unexpected error: %s", result.Error)
	}
}

func TestToolNotFound(t *testing.T) {
	r := NewRegistry()
	_, ok := r.Get("nonexistent")
	if ok {
		t.Fatal("expected nonexistent tool to not be found")
	}
}
