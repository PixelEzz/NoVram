package cpu

import (
	"fmt"
)

// ModelProfile represents an archetype of popular LLMs.
type ModelProfile struct {
	Name       string
	Parameters float64 // in billions
	Layers     int
	KVHeads    int
	HeadDim    int
}

// StandardModelProfiles covers the most popular CPU-viable models.
var StandardModelProfiles = []ModelProfile{
	{Name: "1.5B (Qwen2.5 / DeepSeek-R1)", Parameters: 1.54, Layers: 28, KVHeads: 2, HeadDim: 64},
	{Name: "3B (Llama 3.2)", Parameters: 3.21, Layers: 28, KVHeads: 8, HeadDim: 128},
	{Name: "7B / 8B (Llama 3.1 / Qwen2.5)", Parameters: 8.03, Layers: 32, KVHeads: 8, HeadDim: 128},
	{Name: "14B (Qwen 2.5)", Parameters: 14.7, Layers: 48, KVHeads: 8, HeadDim: 128},
}

// QuantType defines quantization characteristics.
type QuantType struct {
	Name string
	BPW  float64 // bits per weight
	Desc string
}

var SupportedQuants = []QuantType{
	{Name: "IQ3_M", BPW: 3.30, Desc: "Max memory saving, lowest RAM footprint"},
	{Name: "Q4_K_M", BPW: 4.50, Desc: "Golden ratio: near-lossless perplexity & high AVX2 speed"},
	{Name: "Q5_K_M", BPW: 5.50, Desc: "Higher fidelity, moderate RAM increase"},
	{Name: "Q8_0", BPW: 8.50, Desc: "Exact precision reference, highest RAM requirement"},
}

// FeasibilityStatus describes whether a model fits in host RAM.
type FeasibilityStatus string

const (
	StatusOptimal FeasibilityStatus = "OPTIMAL" // <= 65% of available RAM
	StatusTight   FeasibilityStatus = "TIGHT"   // 65% - 85% of available RAM
	StatusSwaps   FeasibilityStatus = "SWAPS"   // > 85% of available RAM
)

// ModelRecommendation contains detailed memory requirements and execution recommendation.
type ModelRecommendation struct {
	ModelName      string
	Quant          string
	ContextSize    int
	ModelWeightGB  float64
	KVCacheGB      float64
	TotalRAMGB     float64
	Status         FeasibilityStatus
	StatusSymbol   string
	EstimatedToks  float64
	RecommendedCmd string
}

// EvaluateModel computes the exact memory footprint and suitability.
func EvaluateModel(info Info, profile ModelProfile, quant QuantType, ctxSize int) ModelRecommendation {
	// 1. Model weights in GB (with 8% tensor metadata overhead)
	weightBytes := (profile.Parameters * 1e9 * quant.BPW / 8.0) * 1.08
	weightGB := weightBytes / (1024 * 1024 * 1024)

	// 2. KV Cache size in GB (fp16: 2 bytes per element, 2 for K and V)
	// Elements per token = 2 * Layers * KVHeads * HeadDim
	elementsPerToken := 2.0 * float64(profile.Layers) * float64(profile.KVHeads) * float64(profile.HeadDim)
	kvBytes := elementsPerToken * float64(ctxSize) * 2.0
	kvGB := kvBytes / (1024 * 1024 * 1024)

	// 3. Scratchpad / runtime working buffer (~0.5 GB)
	scratchGB := 0.5
	totalRAMGB := weightGB + kvGB + scratchGB

	// 4. Memory evaluation against system RAM
	availRAMGB := float64(info.FreeRAMBytes) / (1024 * 1024 * 1024)
	if availRAMGB <= 0 {
		availRAMGB = float64(info.TotalRAMBytes) / (1024 * 1024 * 1024) * 0.75
	}

	ratio := totalRAMGB / availRAMGB
	var status FeasibilityStatus
	var symbol string

	if ratio <= 0.65 {
		status = StatusOptimal
		symbol = "🟢"
	} else if ratio <= 0.85 {
		status = StatusTight
		symbol = "🟡"
	} else {
		status = StatusSwaps
		symbol = "🔴"
	}

	// 5. Estimated tokens/sec heuristic based on CPU cores and vector flags
	speedMultiplier := 1.0
	if info.HasAVX512 {
		speedMultiplier = 1.35
	} else if info.HasAVX2 {
		speedMultiplier = 1.0
	} else if info.HasNEON {
		speedMultiplier = 1.15
	} else {
		speedMultiplier = 0.4
	}

	// Memory bandwidth bound speed estimate
	baseToks := 0.0
	if profile.Parameters <= 2.0 {
		baseToks = 48.0 * speedMultiplier
	} else if profile.Parameters <= 4.0 {
		baseToks = 28.0 * speedMultiplier
	} else if profile.Parameters <= 9.0 {
		baseToks = 14.5 * speedMultiplier
	} else {
		baseToks = 7.5 * speedMultiplier
	}

	// Scale slightly with physical cores (diminishing returns beyond 8 cores due to memory bus saturation)
	coreFactor := 1.0
	if info.PhysicalCores <= 4 {
		coreFactor = 0.75 + float64(info.PhysicalCores)*0.06
	} else if info.PhysicalCores <= 8 {
		coreFactor = 1.0 + float64(info.PhysicalCores-4)*0.05
	} else {
		coreFactor = 1.2
	}
	estToks := baseToks * coreFactor

	cmd := fmt.Sprintf("novram chat --model <path-to-%s-%s.gguf> --ctx %d --threads %d",
		sanitizeName(profile.Name), quant.Name, ctxSize, info.PhysicalCores)

	return ModelRecommendation{
		ModelName:      profile.Name,
		Quant:          quant.Name,
		ContextSize:    ctxSize,
		ModelWeightGB:  weightGB,
		KVCacheGB:      kvGB,
		TotalRAMGB:     totalRAMGB,
		Status:         status,
		StatusSymbol:   symbol,
		EstimatedToks:  estToks,
		RecommendedCmd: cmd,
	}
}

// RecommendedBatchSize returns the optimal n_batch for prompt processing based on L3 cache.
func RecommendedBatchSize(info Info) int {
	if info.L3CacheKB >= 32768 {
		return 1024
	} else if info.L3CacheKB >= 16384 {
		return 512
	} else if info.L3CacheKB >= 8192 {
		return 512
	}
	return 256
}

func sanitizeName(s string) string {
	res := ""
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '.' || c == '-' {
			res += string(c)
		} else if c == ' ' {
			res += "-"
		}
	}
	return res
}
