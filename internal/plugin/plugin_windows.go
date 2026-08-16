//go:build windows

package plugin

import "github.com/agent/ai-terminal/internal/core"

// Go's plugin package is not supported on Windows (no ELF .so loading).
// pluginsSupported reports false so Load becomes a no-op.

func pluginsSupported() bool { return false }

func openAndInit(_ string, _ core.ToolRegistry) error {
	return nil
}
