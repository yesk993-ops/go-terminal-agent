package tool

import (
	"encoding/json"
	"sync"

	"github.com/agent/ai-terminal/internal/core"
)

type registry struct {
	mu    sync.RWMutex
	tools map[string]core.Tool
}

func NewRegistry() core.ToolRegistry {
	return &registry{
		tools: make(map[string]core.Tool),
	}
}

func (r *registry) Register(t core.Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[t.Name()] = t
}

func (r *registry) Get(name string) (core.Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

func (r *registry) List() []core.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]core.Tool, 0, len(r.tools))
	for _, t := range r.tools {
		list = append(list, t)
	}
	return list
}

func (r *registry) Definitions() []core.ToolDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	defs := make([]core.ToolDef, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, core.ToolDef{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: t.Schema(),
		})
	}
	return defs
}

func RegisterDefaultTools(reg core.ToolRegistry) {
	reg.Register(NewReadTool())
	reg.Register(NewWriteTool())
	reg.Register(NewEditTool())
	reg.Register(NewGrepTool())
	reg.Register(NewGlobTool())
	reg.Register(NewBashTool())
	reg.Register(NewWebSearchTool())
	reg.Register(NewWebFetchTool())
}

func schemaFor(props map[string]any, required []string) json.RawMessage {
	s := map[string]any{
		"type":       "object",
		"properties": props,
	}
	if len(required) > 0 {
		s["required"] = required
	}
	data, err := json.Marshal(s)
	if err != nil {
		return json.RawMessage(`{"type":"object","properties":{}}`)
	}
	return data
}
