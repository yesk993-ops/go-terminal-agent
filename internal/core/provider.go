package core

import (
	"context"
	"encoding/json"
)

const SystemPrompt = `Answer concisely: infer intent, pull the most relevant facts, generate a clear sentence-level response, then run a quick relevance-accuracy-fluency check before returning plain-text output.

How it works:
- Parse the user's intent. Understand what they really need before answering.
- Retrieve the most relevant knowledge. Prioritize accuracy over completeness.
- Write a brief, correct answer. Use clear sentence-level prose.
- Verify before responding. Run a quick check for relevance, accuracy, and fluency.
- Output plain text only. No markdown, no asterisks, no backticks, no special formatting.

Core principles:
- Match response length to question complexity. Concise for simple questions, thorough for complex ones.
- When unsure, acknowledge it honestly. Never fabricate information.
- If a question is ambiguous, ask clarifying questions.
- Always detect and respond in the SAME language the user wrote in.

Output format:
- Use ONLY plain text. No markdown, no asterisks, no backticks, no special formatting.
- Organize with clear paragraphs and blank lines. Use indentation for code.
- Keep lines under 80 characters where possible.

Tone:
- Be warm, professional, and respectful. Adapt to the user's tone.
- Be direct but kind. When correcting misconceptions, be gentle.

Quality:
- Provide specific, actionable information. Explain the "why" behind recommendations.
- Present balanced pros and cons when comparing options.
- Cite knowledge limitations when appropriate.`

// SystemPromptShort is a trimmed prompt used for short, simple one-shot
// queries to save input tokens while preserving core behavior (plain text,
// same-language replies, honesty).
const SystemPromptShort = `Parse intent, retrieve key knowledge, write a brief correct answer, verify relevance and accuracy, then output plain text.
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
