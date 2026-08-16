package config

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// loadShellEnv reads API keys (and other exports) from common shell / profile
// files so keys set in the user's profile are picked up even when the process
// was not launched from an interactive shell.
//
// On Unix it parses .bashrc / .zshrc / .profile / .bash_profile.
// On Windows it also checks PowerShell profiles and a plain agent.env file
// next to the config directory.
func loadShellEnv() map[string]string {
	keys := make(map[string]string)
	home := userHome()
	if home == "" {
		return keys
	}

	// Shared plain-text env file: KEY=value per line (works everywhere).
	// Placed beside the config so Windows users can drop keys without PowerShell.
	for _, p := range []string{
		filepath.Join(defaultConfigDir(), "agent.env"),
		filepath.Join(home, ".config", "agent", "agent.env"),
		filepath.Join(home, ".agent.env"),
	} {
		mergeEnvFile(keys, p, parseDotEnv)
	}

	if runtime.GOOS == "windows" {
		// PowerShell profiles: $env:NAME = "value"  or  $env:NAME="value"
		docs := filepath.Join(home, "Documents")
		for _, p := range []string{
			filepath.Join(docs, "WindowsPowerShell", "Microsoft.PowerShell_profile.ps1"),
			filepath.Join(docs, "PowerShell", "Microsoft.PowerShell_profile.ps1"),
			filepath.Join(home, "Documents", "WindowsPowerShell", "profile.ps1"),
		} {
			mergeEnvFile(keys, p, parsePowerShellEnv)
		}
		// Also honour Unix-style rc files (Git Bash / WSL-style homes on Windows).
		for _, name := range []string{".bashrc", ".bash_profile", ".profile", ".zshrc"} {
			mergeEnvFile(keys, filepath.Join(home, name), parseBashExport)
		}
		return keys
	}

	for _, name := range []string{".bashrc", ".zshrc", ".profile", ".bash_profile"} {
		mergeEnvFile(keys, filepath.Join(home, name), parseBashExport)
	}
	return keys
}

// loadBashrcEnv is kept as a thin alias so existing call sites compile.
// Prefer loadShellEnv for new code.
func loadBashrcEnv() map[string]string {
	return loadShellEnv()
}

func mergeEnvFile(dst map[string]string, path string, parse func(string) (string, string, bool)) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// Raise the token limit a bit for long profile files.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if k, v, ok := parse(line); ok && k != "" && v != "" {
			dst[k] = v
		}
	}
}

func parseBashExport(line string) (string, string, bool) {
	if !strings.HasPrefix(line, "export ") {
		return "", "", false
	}
	line = strings.TrimPrefix(line, "export ")
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	key := strings.TrimSpace(parts[0])
	val := strings.Trim(parts[1], "\"' ")
	return key, val, true
}

func parseDotEnv(line string) (string, string, bool) {
	// Support optional "export " prefix.
	line = strings.TrimPrefix(line, "export ")
	if strings.HasPrefix(line, "set ") {
		// Windows cmd: set KEY=value
		line = strings.TrimPrefix(line, "set ")
	}
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	key := strings.TrimSpace(parts[0])
	// Skip lines that look like shell syntax rather than KEY=value.
	if key == "" || strings.ContainsAny(key, " \t$") {
		return "", "", false
	}
	val := strings.TrimSpace(parts[1])
	val = strings.Trim(val, "\"'")
	return key, val, true
}

func parsePowerShellEnv(line string) (string, string, bool) {
	// Match: $env:NAME = "value"   $env:NAME='value'   $env:NAME="value"
	lower := strings.ToLower(line)
	if !strings.Contains(lower, "$env:") {
		return "", "", false
	}
	// Strip comments.
	if idx := strings.Index(line, "#"); idx >= 0 {
		line = line[:idx]
	}
	line = strings.TrimSpace(line)

	// Find $env:
	idx := strings.Index(strings.ToLower(line), "$env:")
	if idx < 0 {
		return "", "", false
	}
	rest := line[idx+5:] // after $env:
	// Split on '='
	parts := strings.SplitN(rest, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	key := strings.TrimSpace(parts[0])
	val := strings.TrimSpace(parts[1])
	val = strings.Trim(val, "\"' ")
	if key == "" || val == "" {
		return "", "", false
	}
	return key, val, true
}
