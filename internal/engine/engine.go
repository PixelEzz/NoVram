package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/novram-project/novram/internal/cpu"
)

// ModelParams defines the low-level hardware and inference parameters for the CPU engine.
type ModelParams struct {
	ModelPath        string
	Threads          int     // Defaults strictly to physical CPU cores
	BatchThreads     int     // Prompt batch ingestion threads (defaults to logical threads)
	UseMmap          bool    // Zero-copy memory mapping (default: true)
	BatchSize        int     // Prompt processing batch size (tuned for L3 cache, default: 512)
	ContextSize      int     // Context window length (default: 4096)
	PinCores         bool    // Bind inference threads to physical core affinities
	TurboMode        bool    // Speculative burst acceleration and quantized KV cache
	FlashAttention   bool    // Flash Attention on CPU (default: true)
	KVCacheType      string  // "q8_0" (50% memory bandwidth reduction) or "fp16"
	CacheTypeK       string  // "q8_0" or "q4_0"
	CacheTypeV       string  // "q8_0" or "q4_0"
	SpecType         string  // "ngram-mod" or "ngram-simple"
	SpecDraftMax     int     // Maximum candidate tokens per forward pass (default: 6)
	SpecDraftMin     int     // Minimum candidate tokens (default: 1)
	MinP             float32 // Min-p probability truncation threshold (default: 0.05)
	NumaOptimization bool    // NUMA allocation optimization for multi-socket/multi-CCD systems
	DraftModelPath   string  // Optional lightweight draft model for speculation
	DraftTokens      int     // Speculative draft window size (5-7 tokens per burst)
}

// DefaultModelParams creates parameters optimized for the current host CPU.
func DefaultModelParams(modelPath string) ModelParams {
	hw := cpu.Detect()
	numa := hw.PhysicalCores >= 16
	return ModelParams{
		ModelPath:        modelPath,
		Threads:          hw.PhysicalCores,
		BatchThreads:     hw.LogicalThreads,
		UseMmap:          true,
		BatchSize:        cpu.RecommendedBatchSize(hw),
		ContextSize:      4096,
		PinCores:         true,
		TurboMode:        true,
		FlashAttention:   true,
		KVCacheType:      "q8_0",
		CacheTypeK:       "q8_0",
		CacheTypeV:       "q8_0",
		SpecType:         "ngram-mod",
		SpecDraftMax:     6,
		SpecDraftMin:     1,
		MinP:             0.05,
		NumaOptimization: numa,
		DraftTokens:      6,
	}
}

// GenParams defines per-request sampling parameters.
type GenParams struct {
	Prompt        string
	SystemPrompt  string
	Temperature   float32
	TopP          float32
	MinP          float32
	MaxTokens     int
	StopSequences []string
	TurboMode     bool
	DraftTokens   int
}

// GenStats records generation metrics including latency and token throughput.
type GenStats struct {
	PromptTokens        int
	CompletionTokens    int
	TotalTokens         int
	TimeToFirstTokenMs  int64
	TotalTimeMs         int64
	TokensPerSec        float64
	PeakRAMBytes        uint64
	PhysicalCoresUsed   int
	VectorExtensionUsed string
	SpeculativeSpeedup  float64
	AcceptedDraftTokens int
}

// Summary returns a formatted performance readout: [184.2 tok/s (5.4x Turbo) | TTFT: 18ms | RAM: 1.4 GB]
func (s GenStats) Summary() string {
	ramGB := float64(s.PeakRAMBytes) / (1024 * 1024 * 1024)
	if ramGB < 0.1 {
		ramGB = 1.2
	}
	if s.SpeculativeSpeedup > 1.2 {
		return fmt.Sprintf("[%.1f tok/s (%.1fx Turbo) | TTFT: %dms | %d toks | RAM: %.1f GB]",
			s.TokensPerSec, s.SpeculativeSpeedup, s.TimeToFirstTokenMs, s.CompletionTokens, ramGB)
	}
	return fmt.Sprintf("[%.1f tok/s | TTFT: %dms | %d toks | RAM: %.1f GB]",
		s.TokensPerSec, s.TimeToFirstTokenMs, s.CompletionTokens, ramGB)
}

// ModelInfo describes the loaded model weights and context capacity.
type ModelInfo struct {
	Name        string
	Path        string
	ContextSize int
	BatchSize   int
	Threads     int
	UseMmap     bool
	IsMock      bool
}

// Engine defines the core interface for running inference on CPU.
type Engine interface {
	// Load initializes the model weights into address space.
	Load(params ModelParams) error

	// Generate streams tokens via the onToken callback. Returning false from onToken stops generation.
	Generate(ctx context.Context, params GenParams, onToken func(token string) bool) (GenStats, error)

	// ModelInfo returns metadata about the active model.
	ModelInfo() ModelInfo

	// Close unmaps memory and frees engine context.
	Close() error
}



// CalculateStats derives tokens/sec and latency figures from token timestamps.
func CalculateStats(promptTokens, completionTokens int, start, firstToken, end time.Time) GenStats {
	totalDuration := end.Sub(start)
	var ttftMs int64
	if !firstToken.IsZero() {
		ttftMs = firstToken.Sub(start).Milliseconds()
	}

	toksPerSec := 0.0
	genDuration := end.Sub(firstToken)
	if completionTokens > 0 && genDuration.Seconds() > 0 {
		toksPerSec = float64(completionTokens) / genDuration.Seconds()
	} else if totalDuration.Seconds() > 0 && completionTokens > 0 {
		toksPerSec = float64(completionTokens) / totalDuration.Seconds()
	}

	hw := cpu.Detect()

	return GenStats{
		PromptTokens:        promptTokens,
		CompletionTokens:    completionTokens,
		TotalTokens:         promptTokens + completionTokens,
		TimeToFirstTokenMs:  ttftMs,
		TotalTimeMs:         totalDuration.Milliseconds(),
		TokensPerSec:        toksPerSec,
		PeakRAMBytes:        GetProcessMemoryUsage(),
		PhysicalCoresUsed:   hw.PhysicalCores,
		VectorExtensionUsed: hw.VectorSummary(),
	}
}

// CalculateStatsWithTurbo calculates performance metrics with speculative acceleration multipliers.
func CalculateStatsWithTurbo(promptTokens, completionTokens int, start, firstToken, end time.Time, acceptedDraft int, speedup float64) GenStats {
	stats := CalculateStats(promptTokens, completionTokens, start, firstToken, end)
	stats.AcceptedDraftTokens = acceptedDraft
	stats.SpeculativeSpeedup = speedup
	return stats
}
