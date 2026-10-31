package engine_test

import (
	"testing"

	"github.com/novram-project/novram/internal/engine"
)

func TestPromptLookupSpeculator(t *testing.T) {
	speculator := engine.NewPromptLookupSpeculator(2, 4, 5)

	prompt := "func calculateMetrics(items []int) float64 { var total float64; for _, item := range items { total += float64(item) } return total }"
	currentGen := "func calculateMetrics(items []int)"

	candidates := speculator.SpeculateCandidates(prompt, currentGen)
	if len(candidates) == 0 {
		t.Fatalf("Expected candidate tokens to be speculated from prompt n-gram match")
	}

	t.Logf("Speculated candidates: %v", candidates)
	if candidates[0] != " float64" {
		t.Errorf("Expected first candidate to be ' float64', got %q", candidates[0])
	}
}

func TestComputeSpeedup(t *testing.T) {
	// Baseline: 100 tokens, 0 accepted draft -> 1.0x
	s1 := engine.ComputeSpeedup(100, 0)
	if s1 != 1.0 {
		t.Errorf("Expected 1.0x speedup, got %.2f", s1)
	}

	// 100 tokens with 80 accepted draft tokens (20 forward passes) -> 5.0x speedup!
	s2 := engine.ComputeSpeedup(100, 80)
	if s2 != 5.0 {
		t.Errorf("Expected 5.0x speedup, got %.2f", s2)
	}
}
