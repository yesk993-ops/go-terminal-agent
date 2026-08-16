package config

import (
	"os"
	"path/filepath"
	"runtime"
)

// userHome returns the current user's home directory in a cross-platform way.
// Prefers os.UserHomeDir (works on Windows, macOS, Linux) and falls back to
// HOME / USERPROFILE environment variables.
func userHome() string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return home
	}
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	// Windows fallback when UserHomeDir is unavailable.
	if home := os.Getenv("USERPROFILE"); home != "" {
		return home
	}
	return ""
}

// defaultConfigDir returns the platform-appropriate config directory for the agent.
//
//	Windows: %APPDATA%\agent  (e.g. C:\Users\<user>\AppData\Roaming\agent)
//	macOS/Linux: ~/.config/agent  (or $XDG_CONFIG_HOME/agent)
func defaultConfigDir() string {
	home := userHome()

	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, appName)
		}
		if home != "" {
			return filepath.Join(home, "AppData", "Roaming", appName)
		}
		return filepath.Join(os.TempDir(), appName)
	}

	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, appName)
	}
	if home != "" {
		return filepath.Join(home, ".config", appName)
	}
	return filepath.Join(os.TempDir(), appName)
}

// defaultSessionPath returns the platform-appropriate session storage directory.
//
//	Windows: %LOCALAPPDATA%\agent\sessions
//	macOS/Linux: ~/.local/share/agent
func defaultSessionPath() string {
	home := userHome()

	if runtime.GOOS == "windows" {
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			return filepath.Join(local, appName, "sessions")
		}
		if home != "" {
			return filepath.Join(home, "AppData", "Local", appName, "sessions")
		}
		return filepath.Join(os.TempDir(), appName, "sessions")
	}

	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, appName)
	}
	if home != "" {
		return filepath.Join(home, ".local", "share", appName)
	}
	return filepath.Join(os.TempDir(), appName, "sessions")
}

// configSearchPaths returns directories to search for config.yaml, in priority order.
func configSearchPaths() []string {
	paths := []string{"."}

	cfgDir := defaultConfigDir()
	if cfgDir != "" {
		paths = append(paths, cfgDir)
	}

	// Legacy / alternate locations kept for compatibility.
	home := userHome()
	if home != "" {
		// Always check ~/.config/agent even on Windows (Git Bash / WSL users).
		legacy := filepath.Join(home, ".config", appName)
		if legacy != cfgDir {
			paths = append(paths, legacy)
		}
	}

	if runtime.GOOS != "windows" {
		paths = append(paths, filepath.Join("/etc", appName))
	}

	return paths
}

// DefaultConfigFile returns the full path to the default config.yaml location.
func DefaultConfigFile() string {
	return filepath.Join(defaultConfigDir(), configFile)
}
