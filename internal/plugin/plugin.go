package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/agent/ai-terminal/internal/core"
	"github.com/agent/ai-terminal/internal/logger"
)

// Plugin is the interface every agent plugin must implement.
type Plugin interface {
	Name() string
	Version() string
	Init(reg core.ToolRegistry) error
}

// pluginExt returns the shared-library extension for the current OS.
// Go's plugin package only supports ELF .so on Linux/macOS; Windows is a no-op.
func pluginExt() string {
	switch runtime.GOOS {
	case "windows":
		return ".dll" // not actually loadable via plugin.Open, but listed for completeness
	case "darwin":
		return ".so"
	default:
		return ".so"
	}
}

// Load discovers and loads plugins from dir. On platforms where Go plugins are
// unsupported (notably Windows), Load is a no-op that returns nil.
func Load(dir string, reg core.ToolRegistry) error {
	if !pluginsSupported() {
		logger.L().Debug("plugins not supported on this platform", "goos", runtime.GOOS)
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read plugin dir: %w", err)
	}

	ext := pluginExt()
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ext {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		if err := openAndInit(path, reg); err != nil {
			logger.L().Warn("failed to load plugin", "path", path, "error", err)
			continue
		}
		logger.L().Info("loaded plugin", "path", path)
	}

	return nil
}

// FindPluginDir returns the first existing plugin directory, or "".
func FindPluginDir() string {
	var paths []string
	paths = append(paths, "./plugins")

	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		paths = append(paths, filepath.Join(home, ".config", "agent", "plugins"))
	}

	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			paths = append(paths, filepath.Join(appData, "agent", "plugins"))
		}
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			paths = append(paths, filepath.Join(local, "agent", "plugins"))
		}
	} else {
		paths = append(paths, "/usr/local/lib/agent/plugins")
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			paths = append(paths, filepath.Join(xdg, "agent", "plugins"))
		}
	}

	for _, p := range paths {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			return p
		}
	}
	return ""
}
