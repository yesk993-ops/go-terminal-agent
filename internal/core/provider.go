package core

import (
	"context"
	"encoding/json"
)

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type Message struct {
	Role       Role            `json:"role"`
	Content    string          `json:"content"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
	Name       string          `json:"name,omitempty"`
}

type Request struct {
	Model     string            `json:"model"`
	Messages  []Message         `json:"messages"`
	Tools     []ToolDef         `json:"tools,omitempty"`
	Stream    bool              `json:"stream"`
	MaxTokens int               `json:"max_tokens,omitempty"`
	Options   map[string]any    `json:"options,omitempty"`
}

type Token struct {
	Content string `json:"content"`
	Done    bool   `json:"done"`
	Error   error  `json:"error,omitempty"`
	ToolCall *ToolCall `json:"tool_call,omitempty"`
}

type ToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type ToolCall struct {
	ID   string          `json:"id"`
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

type Provider interface {
	Name() string
	Stream(ctx context.Context, req *Request) (<-chan Token, error)
}

type ProviderConfig struct {
	Name    string            `json:"name,omitempty"`
	APIKey  string            `json:"api_key"`
	Model   string            `json:"model"`
	BaseURL string            `json:"base_url"`
	Options map[string]any    `json:"options"`
}
