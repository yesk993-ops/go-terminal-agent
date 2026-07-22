package tool

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var allowedPaths []string

func init() {
	// Allow the current working directory and its children by default.
	if cwd, err := os.Getwd(); err == nil {
		allowedPaths = append(allowedPaths, cwd)
	}
	home, err := os.UserHomeDir()
	if err == nil {
		allowedPaths = append(allowedPaths, home)
	}

	// Allow /tmp for temporary file operations.
	allowedPaths = append(allowedPaths, os.TempDir())
}

// resolveSafePath validates that the given filePath is within an allowed
// directory tree after canonicalisation, preventing path traversal attacks.
// It returns the cleaned absolute path or an error.
func resolveSafePath(filePath string) (string, error) {
	cleaned := filepath.Clean(filePath)
	abs, err := filepath.Abs(cleaned)
	if err != nil {
		return "", fmt.Errorf("cannot resolve path: %w", err)
	}

	// Find the deepest existing ancestor and resolve symlinks on it.
	// This handles both existing files and paths that will be created.
	existing := abs
	for {
		real, err := filepath.EvalSymlinks(existing)
		if err == nil {
			// Rebuild the full path by appending the non-existing suffix.
			rel, _ := filepath.Rel(existing, abs)
			if rel == "." {
				rel = ""
			}
			resolved := filepath.Join(real, rel)
			if !isPathAllowed(resolved) {
				return "", fmt.Errorf("access denied: path %q is outside allowed directories", filePath)
			}
			return resolved, nil
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("cannot resolve path: %w", err)
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			// Reached root and it doesn't exist — allow if within permitted paths.
			if !isPathAllowed(abs) {
				return "", fmt.Errorf("access denied: path %q is outside allowed directories", filePath)
			}
			return abs, nil
		}
		existing = parent
	}
}

// isPathAllowed returns true if the resolved path is within any allowed
// directory tree.
func isPathAllowed(resolved string) bool {
	for _, allowed := range allowedPaths {
		allowed = filepath.Clean(allowed)
		if strings.HasPrefix(resolved, allowed+string(filepath.Separator)) ||
			resolved == allowed {
			return true
		}
	}
	return false
}
