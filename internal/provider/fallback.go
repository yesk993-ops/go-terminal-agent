package provider

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/agent/ai-terminal/internal/core"
	"github.com/agent/ai-terminal/internal/logger"
)

const transientRetryDelay = 250 * time.Millisecond

type providerHealth struct {
	unavailableUntil time.Time
}

// fallbackProvider tries configured providers in order. Definite quota or
// rate-limit failures move to the next provider immediately; a short retry is
// reserved for genuinely transient upstream failures.
type fallbackProvider struct {
	providers []core.Provider
	healthMu  sync.Mutex
	health    map[string]providerHealth
}

func NewFallback(primary core.Provider, fallbacks ...core.Provider) core.Provider {
	all := make([]core.Provider, 0, 1+len(fallbacks))
	all = append(all, primary)
	all = append(all, fallbacks...)
	return &fallbackProvider{
		providers: all,
		health:    make(map[string]providerHealth),
	}
}

func (f *fallbackProvider) Name() string {
	names := make([]string, len(f.providers))
	for i, p := range f.providers {
		names[i] = p.Name()
	}
	return strings.Join(names, "→")
}

func (f *fallbackProvider) Stream(ctx context.Context, req *core.Request) (<-chan core.Token, error) {
	var lastErr error
	for idx, prov := range f.providers {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if f.isCoolingDown(prov.Name()) {
			continue
		}
		if idx > 0 {
			logger.L().Warn("falling back", "to", prov.Name())
		}

		ch, err := f.tryWithRetry(ctx, prov, req)
		if err == nil {
			return f.withFallbackNotice(ctx, idx, prov, ch), nil
		}
		lastErr = err

		if isDefinitiveUnavailable(err) {
			f.markUnavailable(prov.Name(), cooldownFor(err))
			logger.L().Warn("provider unavailable, trying next configured provider", "provider", prov.Name(), "error", err)
			continue
		}
		if !isRetryableError(err) {
			return nil, err
		}

		logger.L().Warn("temporary provider failure, trying next configured provider", "provider", prov.Name(), "error", err)
	}

	if lastErr != nil {
		return nil, fmt.Errorf("all providers unavailable: %w", lastErr)
	}
	return nil, fmt.Errorf("all providers unavailable")
}

func (f *fallbackProvider) withFallbackNotice(ctx context.Context, idx int, current core.Provider, in <-chan core.Token) <-chan core.Token {
	if idx == 0 {
		return in
	}
	out := make(chan core.Token, 64)
	go func() {
		defer close(out)
		if !sendFallbackToken(ctx, out, core.Token{
			Content: fmt.Sprintf("\n[Switching to %s after another provider was unavailable...]\n", current.Name()),
		}) {
			return
		}
		for {
			select {
			case tok, ok := <-in:
				if !ok {
					return
				}
				if !sendFallbackToken(ctx, out, tok) {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

func sendFallbackToken(ctx context.Context, out chan<- core.Token, tok core.Token) bool {
	select {
	case out <- tok:
		return true
	case <-ctx.Done():
		return false
	}
}

func (f *fallbackProvider) tryWithRetry(ctx context.Context, prov core.Provider, req *core.Request) (<-chan core.Token, error) {
	for attempt := 0; attempt < 2; attempt++ {
		ch, err := prov.Stream(ctx, req)
		if err == nil {
			return ch, nil
		}
		if isDefinitiveUnavailable(err) || !isRetryableError(err) || attempt == 1 {
			return nil, err
		}

		logger.L().Warn("temporary provider error, retrying once", "provider", prov.Name(), "error", err)
		select {
		case <-time.After(transientRetryDelay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return nil, fmt.Errorf("provider retry loop exited unexpectedly")
}

func (f *fallbackProvider) isCoolingDown(name string) bool {
	f.healthMu.Lock()
	defer f.healthMu.Unlock()
	health, ok := f.health[name]
	if !ok {
		return false
	}
	if time.Now().Before(health.unavailableUntil) {
		return true
	}
	delete(f.health, name)
	return false
}

func (f *fallbackProvider) markUnavailable(name string, delay time.Duration) {
	if delay <= 0 {
		delay = time.Minute
	}
	// Avoid turning an untrusted Retry-After value into an hours-long local
	// outage. The next user request will re-check after this bounded window.
	if delay > 5*time.Minute {
		delay = 5 * time.Minute
	}
	f.healthMu.Lock()
	f.health[name] = providerHealth{unavailableUntil: time.Now().Add(delay)}
	f.healthMu.Unlock()
}

func cooldownFor(err error) time.Duration {
	if httpErr, ok := asHTTPError(err); ok && httpErr.RetryAfter > 0 {
		return httpErr.RetryAfter
	}
	return time.Minute
}

// isDefinitiveUnavailable identifies failures where retrying the same
// provider delays the answer but cannot make progress.
func isDefinitiveUnavailable(err error) bool {
	if httpErr, ok := asHTTPError(err); ok {
		return httpErr.StatusCode == 402 || httpErr.StatusCode == 429
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "insufficient_quota") ||
		strings.Contains(msg, "insufficient credits") ||
		strings.Contains(msg, "rate_limit") ||
		strings.Contains(msg, "rate limit") ||
		strings.Contains(msg, "too many requests")
}

// isRetryableError returns true for temporary upstream and network failures.
func isRetryableError(err error) bool {
	if httpErr, ok := asHTTPError(err); ok {
		return httpErr.StatusCode == 408 || httpErr.StatusCode == 425 || httpErr.StatusCode >= 500
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "upstream") ||
		strings.Contains(msg, "temporarily") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "connection reset")
}
