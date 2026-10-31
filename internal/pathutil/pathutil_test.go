package pathutil_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/novram-project/novram/internal/pathutil"
)

func TestNormalize(t *testing.T) {
	// Relative path resolution
	res := pathutil.Normalize("./models/test.gguf")
	if !filepath.IsAbs(res) {
		t.Errorf("Expected absolute path, got: %s", res)
	}

	// Mixed slashes
	mixed := pathutil.Normalize("models/sub/../test.gguf")
	if strings.Contains(mixed, "..") {
		t.Errorf("Expected path without '..', got: %s", mixed)
	}

	// Tilde expansion
	homeDir, err := os.UserHomeDir()
	if err == nil {
		tildePath := pathutil.Normalize("~/test-model.gguf")
		if !strings.HasPrefix(tildePath, homeDir) {
			t.Errorf("Expected tilde path to start with %s, got: %s", homeDir, tildePath)
		}
	}
}

func TestNormalizeNewlines(t *testing.T) {
	crlf := "Line 1\r\nLine 2\r\nLine 3"
	normalized := pathutil.NormalizeNewlines(crlf)
	if strings.Contains(normalized, "\r") {
		t.Errorf("Expected no carriage returns in normalized output")
	}
	if normalized != "Line 1\nLine 2\nLine 3" {
		t.Errorf("Unexpected normalization result: %q", normalized)
	}
}

func TestFileExists(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "novram_test_*.tmp")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	if !pathutil.FileExists(tmpFile.Name()) {
		t.Errorf("Expected FileExists to return true for existing file: %s", tmpFile.Name())
	}

	if pathutil.FileExists(tmpFile.Name() + ".nonexistent") {
		t.Errorf("Expected FileExists to return false for nonexistent file")
	}
}
