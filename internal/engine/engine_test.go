package engine_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/novram-project/novram/internal/engine"
)

func TestMockEngine(t *testing.T) {
	eng := engine.NewMockEngine()

	params := engine.DefaultModelParams("test-model.gguf")
	if params.Threads <= 0 {
		t.Errorf("Expected Threads > 0, got %d", params.Threads)
	}
	if !params.UseMmap {
		t.Errorf("Expected UseMmap to be true by default")
	}
	if params.BatchSize <= 0 {
		t.Errorf("Expected BatchSize > 0, got %d", params.BatchSize)
	}

	err := eng.Load(params)
	if err != nil {
		t.Fatalf("Failed to load mock engine: %v", err)
	}

	info := eng.ModelInfo()
	if !info.IsMock {
		t.Errorf("Expected IsMock to be true")
	}

	var generatedTokens []string
	genParams := engine.GenParams{
		Prompt:      "Hello novram",
		Temperature: 0.7,
		MaxTokens:   50,
	}

	stats, err := eng.Generate(context.Background(), genParams, func(token string) bool {
		generatedTokens = append(generatedTokens, token)
		return true
	})

	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	fullText := strings.Join(generatedTokens, "")
	if len(fullText) == 0 {
		t.Errorf("Expected non-empty generated text")
	}

	if stats.CompletionTokens <= 0 {
		t.Errorf("Expected CompletionTokens > 0, got %d", stats.CompletionTokens)
	}

	if stats.TokensPerSec <= 0 {
		t.Errorf("Expected TokensPerSec > 0, got %.2f", stats.TokensPerSec)
	}

	summary := stats.Summary()
	if !strings.Contains(summary, "tok/s") {
		t.Errorf("Expected summary to contain 'tok/s', got: %s", summary)
	}

	t.Logf("Generated text: %s", fullText)
	t.Logf("Stats Summary: %s", summary)

	// Test Context Cancellation
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, errCancel := eng.Generate(ctx, genParams, func(token string) bool {
		return true
	})
	if errCancel == nil && ctx.Err() != nil {
		t.Errorf("Expected context cancellation to return error")
	}

	if err := eng.Close(); err != nil {
		t.Errorf("Failed to close engine: %v", err)
	}
}
