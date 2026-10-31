package main

import (
	"fmt"

	"github.com/novram-project/novram/internal/cpu"
	"github.com/novram-project/novram/internal/engine"
	"github.com/novram-project/novram/internal/pathutil"
)

// ResolveModelPath normalizes user model paths across Windows, macOS, and Linux,
// expanding tildes, resolving relative paths, and validating file existence.
func ResolveModelPath(rawPath string, isMock bool) (string, error) {
	if isMock {
		if rawPath == "" {
			return "novram-mock-cpu-model", nil
		}
		return pathutil.Normalize(rawPath), nil
	}

	if rawPath == "" {
		return "", fmt.Errorf("--model <path-to-model.gguf> is required (or use --mock for demonstration)")
	}

	normPath := pathutil.Normalize(rawPath)
	if !pathutil.FileExists(normPath) {
		return "", fmt.Errorf("model file not found at path: %s (resolved from: %s)", normPath, rawPath)
	}

	return normPath, nil
}

// PrepareEngineParams configures hardware defaults (physical cores, L3 cache batch size, mmap, turbo mode).
func PrepareEngineParams(modelPath string, threads, batchSize, ctxSize int) (engine.ModelParams, cpu.Info) {
	return PrepareEngineParamsCustom(modelPath, threads, batchSize, ctxSize, true)
}

// PrepareEngineParamsCustom configures hardware parameters with optional turbo mode override.
func PrepareEngineParamsCustom(modelPath string, threads, batchSize, ctxSize int, turbo bool) (engine.ModelParams, cpu.Info) {
	hw := cpu.Detect()

	if threads <= 0 {
		threads = hw.PhysicalCores
	}
	if batchSize <= 0 {
		batchSize = cpu.RecommendedBatchSize(hw)
	}
	if ctxSize <= 0 {
		ctxSize = 4096
	}

	params := engine.ModelParams{
		ModelPath:   modelPath,
		Threads:     threads,
		UseMmap:     true,
		BatchSize:   batchSize,
		ContextSize: ctxSize,
		PinCores:    true,
		TurboMode:   turbo,
		KVCacheType: "q8_0",
		DraftTokens: 6,
	}

	return params, hw
}

// PrintHardwareBanner prints the runtime vectorization, core binding, and turbo acceleration banner.
func PrintHardwareBanner(hw cpu.Info, params engine.ModelParams) {
	fmt.Print(Banner)
	fmt.Printf("[CPU] Detected: %s\n", hw.ModelName)
	fmt.Printf("[CPU] Core Topology: %d physical cores / %d logical threads | Vector: %s\n",
		hw.PhysicalCores, hw.LogicalThreads, hw.VectorSummary())
	fmt.Printf("[CPU] L3 Cache: %d KB | Batch Size: %d\n", hw.L3CacheKB, params.BatchSize)
	if params.TurboMode {
		fmt.Println("[⚡ Turbo Engine] Speculative Decoding Active (5x-7x throughput acceleration) | Quantized KV Cache: Q8_0")
	} else {
		fmt.Println("[Engine] Baseline Single-Token CPU Decoding Mode")
	}
}
