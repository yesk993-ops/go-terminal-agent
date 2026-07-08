package core

import (
	"context"
	"encoding/json"
)

type ToolResultStatus string

const (
	StatusSuccess ToolResultStatus = "success"
	StatusError   ToolResultStatus = "error"
	StatusPending ToolResultStatus = "pending"
)

type ToolResult struct {
	Status ToolResultStatus `json:"status"`
	Output string           `json:"output"`
	Error  string           `json:"error,omitempty"`
	Data   json.RawMessage  `json:"data,omitempty"`
}

type Tool interface {
	Name() string
	Description() string
	Schema() json.RawMessage
	Execute(ctx context.Context, args json.RawMessage) *ToolResult
}

type ToolRegistry interface {
	Register(tool Tool)
	Get(name string) (Tool, bool)
	List() []Tool
	Definitions() []ToolDef
}
