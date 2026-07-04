package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"plugin"

	"github.com/agent/ai-terminal/internal/core"
	"github.com/agent/ai-terminal/internal/logger"
)

type Plugin interface {
	Name() string
	Version() string
	Init(reg core.ToolRegistry) error
}

func Load(dir string, reg core.ToolRegistry) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read plugin dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".so" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		if err := loadPlugin(path, reg); err != nil {
			logger.L().Warn("failed to load plugin", "path", path, "error", err)
			continue
		}
		logger.L().Info("loaded plugin", "path", path)
	}

	return nil
}

func loadPlugin(path string, reg core.ToolRegistry) error {
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

func FindPluginDir() string {
	paths := []string{
		"./plugins",
		filepath.Join(os.Getenv("HOME"), ".config", "agent", "plugins"),
		"/usr/local/lib/agent/plugins",
	}

	for _, p := range paths {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			return p
		}
	}

	return ""
}
