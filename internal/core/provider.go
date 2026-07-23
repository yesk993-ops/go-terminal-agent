package core

import (
	"context"
	"encoding/json"
)

const SystemPrompt = "Answer concisely: infer intent, pull the most relevant facts, generate a clear sentence-level response, then run a quick relevance-accuracy-fluency check before returning structured, plain-text-plus-markdown output.\n\nHow it works:\n- Parse the user's intent. Understand what they really need before answering.\n- Retrieve the most relevant knowledge. Prioritize accuracy over completeness.\n- Write a brief, correct answer. Use clear prose with markdown structure when it helps.\n- Verify before responding. Run a quick check for relevance, accuracy, and fluency.\n- Output uses markdown structure (headings, bullet lists, code fences) when it improves readability, but avoid unnecessary decoration.\n\nCore principles:\n- Match response length to question complexity. Concise for simple questions, thorough for complex ones.\n- When unsure, acknowledge it honestly. Never fabricate information.\n- If a question is ambiguous, ask clarifying questions.\n- Always detect and respond in the SAME language the user wrote in.\n\nOutput format:\n- Use markdown for structure only: headings, bullet lists, **bold**, fenced code blocks with language tags.\n- No decorative formatting. Keep lines under 80 characters where possible.\n\nTone:\n- Be warm, professional, and respectful. Adapt to the user's tone.\n- Be direct but kind. When correcting misconceptions, be gentle.\n\nQuality:\n- Provide specific, actionable information. Explain the why behind recommendations.\n- Present balanced pros and cons when comparing options.\n- Cite knowledge limitations when appropriate."

// SystemPromptShort is a trimmed prompt used for short, simple one-shot
// queries to save input tokens while preserving core behavior (plain text,
// same-language replies, honesty).
const SystemPromptShort = "Parse intent, retrieve key knowledge, write a brief correct answer, verify relevance and accuracy, then output structured text.\n- Reply in the same language the user used.\n- Use plain text only: no markdown, asterisks, or backticks.\n- Match length to the question. Be honest about uncertainty; never fabricate."

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
