package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/agent/ai-terminal/internal/core"
	"github.com/agent/ai-terminal/internal/logger"
)

// fallbackProvider tries a list of providers in order.
// Each provider gets up to 3 retries with backoff on rate-limit errors
// before falling through to the next provider in the chain.
type fallbackProvider struct {
	providers []core.Provider
}

func NewFallback(primary core.Provider, fallbacks ...core.Provider) core.Provider {
	all := make([]core.Provider, 0, 1+len(fallbacks))
	all = append(all, primary)
	all = append(all, fallbacks...)
	return &fallbackProvider{providers: all}
}

func (f *fallbackProvider) Name() string {
	names := make([]string, len(f.providers))
	for i, p := range f.providers {
		names[i] = p.Name()
	}
	return strings.Join(names, "→")
}

func (f *fallbackProvider) Stream(ctx context.Context, req *core.Request) (<-chan core.Token, error) {
	for idx, prov := range f.providers {
		// Check context before trying each provider.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if idx > 0 {
			prevName := f.providers[idx-1].Name()
			logger.L().Warn("falling back",
				"from", prevName,
				"to", prov.Name())
		}

		// Retry up to 3 times on rate-limit errors
		ch, err := f.tryWithRetry(ctx, prov, req)
		if err == nil {
			if idx == 0 {
				return ch, nil
			}
			// Wrap with status message; handle context cancellation.
			out := make(chan core.Token, 64)
			go func(prev, curr string, in <-chan core.Token) {
				defer close(out)
				select {
				case out <- core.Token{
					Content: fmt.Sprintf("\n[Unavailable: %s, switching to %s...]\n", prev, curr),
				}:
				case <-ctx.Done():
					return
				}
				for {
					select {
					case tok, ok := <-in:
						if !ok {
							return
						}
						select {
						case out <- tok:
						case <-ctx.Done():
							return
						}
					case <-ctx.Done():
						return
					}
				}
			}(f.providers[idx-1].Name(), prov.Name(), ch)
			return out, nil
		}

		// If it's not a retryable error, don't try other providers
		if !isRetryableError(err) {
			return nil, err
		}

		// Rate limited — continue to next provider
		logger.L().Warn("rate limited, trying next provider",
			"provider", prov.Name(),
			"error", err.Error())
	}

	return nil, fmt.Errorf("all providers unavailable")
}

func (f *fallbackProvider) tryWithRetry(ctx context.Context, prov core.Provider, req *core.Request) (<-chan core.Token, error) {
	var lastErr error
	for attempt := 0; attempt <= 3; attempt++ {
		ch, err := prov.Stream(ctx, req)
		if err == nil {
			return ch, nil
		}
		lastErr = err
		if !isRetryableError(err) {
			return nil, err
		}
		if attempt < 3 {
			backoff := time.Duration(1+attempt*2) * time.Second
			logger.L().Warn("retryable error, retrying",
				"provider", prov.Name(),
				"attempt", attempt+1,
				"backoff", backoff,
				"error", err.Error())
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}
	return nil, lastErr
}

// isRetryableError returns true for errors that should trigger a fallback
// to the next provider: rate limits, quota exhaustion, insufficient credits,
// and temporary upstream failures.
func isRetryableError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "429") ||
		strings.Contains(msg, "402") ||
		strings.Contains(msg, "rate_limit") ||
		strings.Contains(msg, "rate limit") ||
		strings.Contains(msg, "too many requests") ||
		strings.Contains(msg, "insufficient_quota") ||
		strings.Contains(msg, "insufficient credits") ||
		strings.Contains(msg, "upstream") ||
		strings.Contains(msg, "temporarily")
}
