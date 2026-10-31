package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/novram-project/novram/internal/cpu"
	"github.com/novram-project/novram/internal/engine"
)

func runBench(args []string) {
	fs := flag.NewFlagSet("bench", flag.ExitOnError)
	modelPath := fs.String("model", "", "Path to GGUF model file (shorthand -m)")
	fs.StringVar(modelPath, "m", "", "Path to GGUF model file")
	threads := fs.Int("threads", 0, "Number of inference threads (default: physical cores)")
	fs.IntVar(threads, "t", 0, "Number of inference threads")
	tokens := fs.Int("tokens", 128, "Number of tokens to generate for benchmark")
	useMock := fs.Bool("mock", false, "Run benchmark in mock simulation mode")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	hw := cpu.Detect()
	if *threads <= 0 {
		*threads = hw.PhysicalCores
	}

	fmt.Print(Banner)
	fmt.Println("⚡ NOVRAM CPU BENCHMARK TOOL")
	fmt.Println("================================================================================")
	fmt.Printf("• CPU:             %s\n", hw.ModelName)
	fmt.Printf("• Vector Flags:    %s\n", hw.VectorSummary())
	fmt.Printf("• Threads:         %d physical cores\n", *threads)
	fmt.Printf("• Target Tokens:   %d\n", *tokens)
	fmt.Println("================================================================================")

	var eng engine.Engine
	if *useMock || *modelPath == "" {
		eng = engine.NewMockEngine()
		fmt.Println("[Bench] Running with in-memory engine benchmark...")
	} else {
		eng = engine.NewLlamaEngine()
	}

	params := engine.ModelParams{
		ModelPath:   *modelPath,
		Threads:     *threads,
		UseMmap:     true,
		BatchSize:   cpu.RecommendedBatchSize(hw),
		ContextSize: 4096,
		PinCores:    true,
	}

	t0 := time.Now()
	if err := eng.Load(params); err != nil {
		fmt.Fprintf(os.Stderr, "Load error: %v\n", err)
		os.Exit(1)
	}
	defer eng.Close()
	loadDuration := time.Since(t0)

	fmt.Printf("• Model Load Time: %d ms (mmap zero-copy)\n", loadDuration.Milliseconds())
	fmt.Println("\nRunning warm-up and generation benchmark...")

	prompt := "Explain the importance of CPU cache locality and SIMD vectorization in deep learning inference."
	genParams := engine.GenParams{
		Prompt:      prompt,
		Temperature: 0.2,
		MaxTokens:   *tokens,
	}

	var sb strings.Builder
	stats, err := eng.Generate(context.Background(), genParams, func(tok string) bool {
		sb.WriteString(tok)
		return true
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "Benchmark failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n================================================================================")
	fmt.Println("📈 BENCHMARK RESULTS:")
	fmt.Println("--------------------------------------------------------------------------------")
	fmt.Printf("• Time to First Token (TTFT): %d ms\n", stats.TimeToFirstTokenMs)
	fmt.Printf("• Generation Throughput:      %.2f tokens/second\n", stats.TokensPerSec)
	fmt.Printf("• Total Tokens Generated:     %d tokens\n", stats.CompletionTokens)
	fmt.Printf("• Total Inference Time:       %d ms\n", stats.TotalTimeMs)
	fmt.Printf("• Memory Working Set:         %.2f GB\n", float64(stats.PeakRAMBytes)/(1024*1024*1024))
	fmt.Println("================================================================================")
}
