package core

import "context"

type Agent interface {
	Run(ctx context.Context, req *Request) (<-chan Token, error)
	SetProvider(p Provider)
	SetTools(registry ToolRegistry)
}
