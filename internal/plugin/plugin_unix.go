//go:build !windows

package plugin

import (
	"fmt"
	"plugin"

	"github.com/agent/ai-terminal/internal/core"
)

func pluginsSupported() bool { return true }

func openAndInit(path string, reg core.ToolRegistry) error {
	p, err := plugin.Open(path)
	if err != nil {
		return fmt.Errorf("open plugin: %w", err)
	}

	sym, err := p.Lookup("PluginInstance")
	if err != nil {
		return fmt.Errorf("lookup PluginInstance: %w", err)
	}

	plug, ok := sym.(Plugin)
	if !ok {
		return fmt.Errorf("PluginInstance does not implement plugin.Plugin interface")
	}

	if err := plug.Init(reg); err != nil {
		return fmt.Errorf("plugin init: %w", err)
	}
	return nil
}
