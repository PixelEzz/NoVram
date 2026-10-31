package interceptor_test

import (
	"strings"
	"testing"

	"github.com/novram-project/novram/internal/cpu"
	"github.com/novram-project/novram/internal/interceptor"
)

func TestFindModel(t *testing.T) {
	// Exact match
	m1, found1 := interceptor.FindModel("llama-3.2-3b")
	if !found1 || m1.ID != "llama-3.2-3b" {
		t.Errorf("Expected to find llama-3.2-3b, got: %+v", m1)
	}

	// Alias matches from IDE requests
	m2, found2 := interceptor.FindModel("gpt-4o")
	if !found2 || m2.ID == "" {
		t.Errorf("Expected to map gpt-4o alias to a catalog model")
	}

	m3, found3 := interceptor.FindModel("deepseek-r1")
	if !found3 || !strings.Contains(m3.ID, "deepseek") {
		t.Errorf("Expected deepseek-r1 to map to deepseek entry")
	}

	m4, found4 := interceptor.FindModel("qwen-coder")
	if !found4 || !strings.Contains(m4.ID, "qwen") {
		t.Errorf("Expected qwen-coder to map to qwen entry")
	}
}

func TestInterceptAndOptimize(t *testing.T) {
	hw := cpu.Detect()
	decision := interceptor.InterceptAndOptimize(hw, "gpt-4o", 2048)

	if decision.OriginalModel != "gpt-4o" {
		t.Errorf("Expected OriginalModel gpt-4o, got %s", decision.OriginalModel)
	}
	if decision.SelectedModel == "" {
		t.Errorf("Expected non-empty SelectedModel")
	}
	if decision.RecommendedQuant == "" {
		t.Errorf("Expected recommended quantization")
	}
	if decision.EstTokensPerSec <= 0 {
		t.Errorf("Expected positive estimated tok/s")
	}
	if decision.RecommendationNote == "" {
		t.Errorf("Expected non-empty RecommendationNote")
	}

	t.Logf("Decision for %s:", decision.OriginalModel)
	t.Logf("  Selected: %s (%s)", decision.SelectedModel, decision.RecommendedQuant)
	t.Logf("  Status: %s", decision.SafetyStatus)
	t.Logf("  Note: %s", decision.RecommendationNote)
	t.Logf("  HuggingFace: %s", decision.HuggingFaceDownload)
}
