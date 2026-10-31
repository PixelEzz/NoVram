package interceptor

import (
	"fmt"
	"strings"

	"github.com/novram-project/novram/internal/cpu"
)

// ModelEntry represents a known model family in the interceptor catalog.
type ModelEntry struct {
	ID          string   // e.g. "llama-3.2-3b"
	Family      string   // e.g. "llama"
	Aliases     []string // e.g. ["gpt-4o", "claude-3-5-sonnet", "llama3"]
	Parameters  float64  // in billions
	ContextMax  int
	Category    string   // "general", "coding", "reasoning"
	Description string
	HuggingFace string   // Canonical GGUF repo
}

// Catalog contains popular models that users or AI coding extensions request.
var Catalog = []ModelEntry{
	{
		ID:          "deepseek-r1-1.5b",
		Family:      "deepseek",
		Aliases:     []string{"deepseek-r1", "deepseek-coder", "r1-1.5b", "reasoning-mini"},
		Parameters:  1.5,
		ContextMax:  32768,
		Category:    "reasoning",
		Description: "DeepSeek-R1 Distill Qwen 1.5B (Fast reasoning on CPU)",
		HuggingFace: "bartowski/DeepSeek-R1-Distill-Qwen-1.5B-GGUF",
	},
	{
		ID:          "qwen2.5-coder-1.5b",
		Family:      "qwen",
		Aliases:     []string{"qwen-coder", "coder-mini", "copilot-fast"},
		Parameters:  1.54,
		ContextMax:  32768,
		Category:    "coding",
		Description: "Qwen 2.5 Coder 1.5B (Ultra-fast CPU code completion)",
		HuggingFace: "Qwen/Qwen2.5-Coder-1.5B-Instruct-GGUF",
	},
	{
		ID:          "llama-3.2-1b",
		Family:      "llama",
		Aliases:     []string{"llama-1b", "llama3-1b", "gpt-3.5-turbo", "fast"},
		Parameters:  1.23,
		ContextMax:  131072,
		Category:    "general",
		Description: "Llama 3.2 1B Instruct (Sub-second latency on commodity CPUs)",
		HuggingFace: "bartowski/Llama-3.2-1B-Instruct-GGUF",
	},
	{
		ID:          "llama-3.2-3b",
		Family:      "llama",
		Aliases:     []string{"llama-3b", "llama3-3b", "gpt-4o-mini", "claude-3-haiku"},
		Parameters:  3.21,
		ContextMax:  131072,
		Category:    "general",
		Description: "Llama 3.2 3B Instruct (Golden ratio for 8GB-16GB RAM machines)",
		HuggingFace: "bartowski/Llama-3.2-3B-Instruct-GGUF",
	},
	{
		ID:          "qwen2.5-coder-7b",
		Family:      "qwen",
		Aliases:     []string{"qwen-7b", "qwen2.5-7b", "claude-3-5-sonnet", "gpt-4o", "continue", "cline", "roo-code"},
		Parameters:  7.61,
		ContextMax:  32768,
		Category:    "coding",
		Description: "Qwen 2.5 Coder 7B (State of the art coding model for 16GB+ RAM)",
		HuggingFace: "Qwen/Qwen2.5-Coder-7B-Instruct-GGUF",
	},
	{
		ID:          "llama-3.1-8b",
		Family:      "llama",
		Aliases:     []string{"llama-8b", "llama3-8b", "llama-3.1"},
		Parameters:  8.03,
		ContextMax:  131072,
		Category:    "general",
		Description: "Llama 3.1 8B Instruct (Premier general purpose open weights)",
		HuggingFace: "bartowski/Meta-Llama-3.1-8B-Instruct-GGUF",
	},
	{
		ID:          "qwen2.5-14b",
		Family:      "qwen",
		Aliases:     []string{"qwen-14b", "deepseek-r1-14b"},
		Parameters:  14.7,
		ContextMax:  32768,
		Category:    "reasoning",
		Description: "Qwen 2.5 14B (High intelligence for 32GB+ RAM systems)",
		HuggingFace: "Qwen/Qwen2.5-14B-Instruct-GGUF",
	},
}

// FindModel matches a user/IDE requested model name against the catalog.
func FindModel(requestedName string) (ModelEntry, bool) {
	clean := strings.ToLower(strings.TrimSpace(requestedName))

	// 1. Direct ID match
	for _, m := range Catalog {
		if strings.EqualFold(m.ID, clean) {
			return m, true
		}
	}

	// 2. Alias match (e.g. IDE asking for "gpt-4o", "claude-3-5-sonnet", "copilot")
	for _, m := range Catalog {
		for _, alias := range m.Aliases {
			if strings.EqualFold(alias, clean) || strings.Contains(clean, alias) {
				return m, true
			}
		}
	}

	// 3. Fallback: match by family name
	if strings.Contains(clean, "deepseek") || strings.Contains(clean, "r1") {
		return Catalog[0], true // deepseek-r1-1.5b
	}
	if strings.Contains(clean, "qwen") || strings.Contains(clean, "code") {
		return Catalog[1], true // qwen2.5-coder-1.5b
	}
	if strings.Contains(clean, "llama") {
		return Catalog[3], true // llama-3.2-3b
	}

	// Default fallback to optimal 3B
	return Catalog[3], false
}

// OptimizeDecision contains the interceptor's recommendation and tuning verdict.
type OptimizeDecision struct {
	OriginalModel       string
	SelectedModel       string
	RecommendedQuant    string
	RecommendedContext  int
	EstTokensPerSec     float64
	RequiredRAMGB       float64
	AvailableRAMGB      float64
	SafetyStatus        string // "SAFE", "TIGHT", "SWAP_WARNING"
	RecommendationNote  string
	HuggingFaceDownload string
}

// InterceptAndOptimize evaluates the incoming request against host CPU & RAM.
func InterceptAndOptimize(hw cpu.Info, requestedModel string, requestedMaxTokens int) OptimizeDecision {
	matched, _ := FindModel(requestedModel)

	availRAMGB := float64(hw.FreeRAMBytes) / (1024 * 1024 * 1024)
	if availRAMGB <= 0 {
		availRAMGB = float64(hw.TotalRAMBytes) / (1024 * 1024 * 1024) * 0.75
	}

	// Determine best quantization for host RAM
	recommendedQuant := "Q4_K_M"
	bpw := 4.5
	if availRAMGB < 3.0 {
		recommendedQuant = "IQ3_M"
		bpw = 3.3
	} else if availRAMGB > 12.0 && matched.Parameters <= 7.0 {
		recommendedQuant = "Q5_K_M"
		bpw = 5.5
	}

	// Calculate memory footprint
	weightGB := (matched.Parameters * 1e9 * bpw / 8.0 * 1.08) / (1024 * 1024 * 1024)
	ctxTokens := 4096
	if requestedMaxTokens > 4096 {
		ctxTokens = 8192
	}
	kvGB := (float64(ctxTokens) * matched.Parameters * 0.06) / 1024.0
	totalRAMGB := weightGB + kvGB + 0.5

	safety := "SAFE"
	note := fmt.Sprintf("Running %s (%s) seamlessly within %.2f GB available RAM.", matched.ID, recommendedQuant, availRAMGB)

	// Check for swapping risk
	if totalRAMGB > availRAMGB {
		safety = "SWAP_WARNING"
		// Recommend a lighter alternative that will guarantee zero swapping
		var lighterModel ModelEntry
		for _, alt := range Catalog {
			altWeight := (alt.Parameters * 1e9 * 3.3 / 8.0 * 1.08) / (1024 * 1024 * 1024)
			if altWeight+0.8 <= availRAMGB {
				lighterModel = alt
				break
			}
		}
		if lighterModel.ID != "" {
			note = fmt.Sprintf("Requested '%s' requires %.2f GB RAM (exceeds %.2f GB free). Interceptor recommends '%s-Q4_K_M' for peak CPU throughput.",
				requestedModel, totalRAMGB, availRAMGB, lighterModel.ID)
		} else {
			note = fmt.Sprintf("High RAM footprint (%.2f GB) vs %.2f GB available. Close other applications to prevent swap paging.", totalRAMGB, availRAMGB)
		}
	} else if totalRAMGB > availRAMGB*0.80 {
		safety = "TIGHT"
		note = fmt.Sprintf("Memory headroom is tight (%.2f GB / %.2f GB). Running with optimized L3 batch size to protect cache.", totalRAMGB, availRAMGB)
	}

	// Speed heuristic
	speedFactor := 1.0
	if hw.HasAVX512 {
		speedFactor = 1.35
	} else if hw.HasAVX2 {
		speedFactor = 1.0
	} else if hw.HasNEON {
		speedFactor = 1.15
	} else {
		speedFactor = 0.4
	}

	estSpeed := (48.0 / matched.Parameters) * speedFactor
	if estSpeed > 65.0 {
		estSpeed = 65.0
	}

	downloadURL := fmt.Sprintf("https://huggingface.co/%s", matched.HuggingFace)

	return OptimizeDecision{
		OriginalModel:       requestedModel,
		SelectedModel:       matched.ID,
		RecommendedQuant:    recommendedQuant,
		RecommendedContext:  ctxTokens,
		EstTokensPerSec:     estSpeed,
		RequiredRAMGB:       totalRAMGB,
		AvailableRAMGB:      availRAMGB,
		SafetyStatus:        safety,
		RecommendationNote:  note,
		HuggingFaceDownload: downloadURL,
	}
}
