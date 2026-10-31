package pathutil

import (
	"os"
	"path/filepath"
	"strings"
)

// Normalize resolves a user-supplied path into a clean, platform-native absolute path.
// It handles forward slashes, backslashes, tilde expansions (~), and environment variables.
func Normalize(path string) string {
	if path == "" {
		return ""
	}

	// 1. Expand environment variables (e.g. $HOME on Unix, %USERPROFILE% on Windows)
	expanded := os.ExpandEnv(path)

	// 2. Expand tilde (~) prefix to home directory
	if strings.HasPrefix(expanded, "~/") || strings.HasPrefix(expanded, "~\\") || expanded == "~" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			if expanded == "~" {
				expanded = homeDir
			} else {
				expanded = filepath.Join(homeDir, expanded[2:])
			}
		}
	}

	// 3. Convert slashes to the host OS format
	nativePath := filepath.FromSlash(expanded)

	// 4. Resolve clean absolute path
	absPath, err := filepath.Abs(nativePath)
	if err != nil {
		return filepath.Clean(nativePath)
	}
	return filepath.Clean(absPath)
}

// FileExists returns true if the path exists and is a regular file.
func FileExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(Normalize(path))
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// NormalizeNewlines normalizes CRLF and CR line endings into standard LF (\n).
func NormalizeNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}
