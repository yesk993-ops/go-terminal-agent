package provider

import (
	"context"
	"strings"

	"github.com/agent/ai-terminal/internal/core"
	"github.com/agent/ai-terminal/internal/logger"
)

type fallbackProvider struct {
	primary   core.Provider
	secondary core.Provider
}

func NewFallback(primary, secondary core.Provider) core.Provider {
	return &fallbackProvider{
		primary:   primary,
		secondary: secondary,
	}
}

func (f *fallbackProvider) Name() string {
	return f.primary.Name() + "+" + f.secondary.Name()
}

func (f *fallbackProvider) Stream(ctx context.Context, req *core.Request) (<-chan core.Token, error) {
	ch, err := f.primary.Stream(ctx, req)
	if err == nil {
		return ch, nil
	}

	if isRateLimitError(err) {
		logger.L().Warn("primary provider rate limited, falling back",
			"primary", f.primary.Name(),
			"error", err.Error())

		return f.secondary.Stream(ctx, req)
	}

	return nil, err
}

func isRateLimitError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "429") ||
		strings.Contains(msg, "rate_limit") ||
		strings.Contains(msg, "rate limit") ||
		strings.Contains(msg, "too many requests") ||
		strings.Contains(msg, "insufficient_quota")
}
