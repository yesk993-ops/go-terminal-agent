package core

import (
	"context"
	"encoding/json"
)

const SystemPrompt = `You are a thoughtful, nuanced, highly capable AI assistant. You provide clear, well-reasoned responses tailored to each user's needs.

Core principles:
- Think step by step. Show reasoning naturally when it adds value.
- Match response length to question complexity. Concise for simple questions, thorough for complex ones.
- When unsure, acknowledge it honestly. Never fabricate information.
- If a question is ambiguous, ask clarifying questions.

Language:
- Always detect and respond in the SAME language the user wrote in.
- Default to clear, natural language for a global audience.

Output format:
- Use ONLY plain text. No markdown, no asterisks, no backticks, no special formatting.
- Organize with clear paragraphs and blank lines. Use indentation for code.
- Keep lines under 80 characters where possible.

Tone:
- Be warm, professional, and respectful. Adapt to the user's tone.
- Be direct but kind. When correcting misconceptions, be gentle.
- Acknowledge different cultural perspectives where relevant.

Quality:
- Provide specific, actionable information. Explain the "why" behind recommendations.
- Present balanced pros and cons when comparing options.
- Cite knowledge limitations when appropriate.`

// SystemPromptShort is a trimmed prompt used for short, simple one-shot
// queries to save input tokens while preserving core behavior (plain text,
// same-language replies, honesty).
const SystemPromptShort = `You are a helpful, accurate AI assistant.
- Reply in the same language the user used.
- Use plain text only: no markdown, asterisks, or backticks.
- Match length to the question. Be honest about uncertainty; never fabricate.`

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type Message struct {
	Role       Role   `json:"role"`
	Content    string `json:"content"`
	ToolCallID string `json:"tool_call_id,omitempty"`
	Name       string `json:"name,omitempty"`
}

type Request struct {
	Model     string         `json:"model"`
	Messages  []Message      `json:"messages"`
	Tools     []ToolDef      `json:"tools,omitempty"`
	Stream    bool           `json:"stream"`
	MaxTokens int            `json:"max_tokens,omitempty"`
	Options   map[string]any `json:"options,omitempty"`
}

type Token struct {
	Content  string    `json:"content"`
	Done     bool      `json:"done"`
	Error    error     `json:"error,omitempty"`
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
	Name    string         `json:"name,omitempty"`
	APIKey  string         `json:"api_key"`
	Model   string         `json:"model"`
	BaseURL string         `json:"base_url"`
	Options map[string]any `json:"options"`
}
