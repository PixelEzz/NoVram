package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/novram-project/novram/internal/engine"
	"github.com/novram-project/novram/internal/pathutil"
)

func runChat(args []string) {
	fs := flag.NewFlagSet("chat", flag.ExitOnError)
	modelPathFlag := fs.String("model", "", "Path to GGUF model file (shorthand -m)")
	fs.StringVar(modelPathFlag, "m", "", "Path to GGUF model file")
	threads := fs.Int("threads", 0, "Number of inference threads (default: physical CPU cores)")
	fs.IntVar(threads, "t", 0, "Number of inference threads")
	ctxSize := fs.Int("ctx", 4096, "Context window length (shorthand -c)")
	fs.IntVar(ctxSize, "c", 4096, "Context window length")
	batchSize := fs.Int("batch", 0, "Batch size for prompt processing (default: tuned for L3 cache)")
	fs.IntVar(batchSize, "b", 0, "Batch size for prompt processing")
	temp := fs.Float64("temp", 0.7, "Sampling temperature")
	topP := fs.Float64("top-p", 0.9, "Top-p sampling")
	systemPrompt := fs.String("system", "You are an expert AI assistant running on CPU via novram. Provide thorough, comprehensive, and detailed answers with complete code examples.", "System prompt")
	batchThreads := fs.Int("threads-batch", 0, "Number of threads for prompt batch processing (default: logical threads)")
	fs.IntVar(batchThreads, "tb", 0, "Number of threads for prompt batch processing")
	flashAttn := fs.Bool("flash-attn", true, "Enable Flash Attention on CPU")
	fs.BoolVar(flashAttn, "fa", true, "Enable Flash Attention on CPU")
	turbo := fs.Bool("turbo", true, "Enable speculative acceleration, quantized KV cache, and Flash Attention")
	maxTokens := fs.Int("max-tokens", 4096, "Maximum tokens to generate per response (shorthand -n)")
	fs.IntVar(maxTokens, "n", 4096, "Maximum tokens to generate per response")
	minP := fs.Float64("min-p", 0.05, "Min-P dynamic sampling threshold (0.0 = disabled)")
	useMock := fs.Bool("mock", false, "Use fast in-memory mock engine (for testing without GGUF weights)")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	if *modelPathFlag == "" && fs.NArg() > 0 {
		*modelPathFlag = fs.Arg(0)
	}

	modelPath, err := ResolveModelPath(*modelPathFlag, *useMock)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	params, hw := PrepareEngineParamsCustom(modelPath, *threads, *batchSize, *ctxSize, *turbo)
	if *batchThreads > 0 {
		params.BatchThreads = *batchThreads
	}
	params.FlashAttention = *flashAttn
	params.MinP = float32(*minP)
	PrintHardwareBanner(hw, params)

	var eng engine.Engine
	if *useMock {
		eng = engine.NewMockEngine()
		fmt.Println("[Engine] Running in MOCK SIMULATION mode (for tests & demonstration)")
	} else {
		eng = engine.NewLlamaEngine()
	}

	fmt.Printf("[Engine] Loading model with %d threads (physical cores), mmap=true, n_batch=%d...\n",
		params.Threads, params.BatchSize)

	if err := eng.Load(params); err != nil {
		fmt.Fprintf(os.Stderr, "Engine load failed: %v\n", err)
		os.Exit(1)
	}
	defer eng.Close()

	fmt.Println("\nModel loaded successfully into memory address space.")
	fmt.Println("Type your message and press Enter. Commands: /quit, /exit, /clear")
	fmt.Println()

	// Set up interrupt handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	scanner := bufio.NewScanner(os.Stdin)
	var conversationHistory strings.Builder
	if *systemPrompt != "" {
		conversationHistory.WriteString(fmt.Sprintf("System: %s\n", *systemPrompt))
	}

	for {
		fmt.Print("novram> ")
		if !scanner.Scan() {
			break
		}

		rawLine := scanner.Text()
		line := strings.TrimSpace(pathutil.NormalizeNewlines(rawLine))
		if line == "" {
			continue
		}

		if line == "/exit" || line == "/quit" {
			fmt.Println("Exiting novram. Goodbye!")
			break
		}
		if line == "/clear" {
			conversationHistory.Reset()
			if *systemPrompt != "" {
				conversationHistory.WriteString(fmt.Sprintf("System: %s\n", *systemPrompt))
			}
			fmt.Println("Conversation history cleared.")
			continue
		}

		conversationHistory.WriteString(fmt.Sprintf("User: %s\nAssistant: ", line))

		genParams := engine.GenParams{
			Prompt:       conversationHistory.String(),
			SystemPrompt: *systemPrompt,
			Temperature:  float32(*temp),
			TopP:         float32(*topP),
			MinP:         float32(*minP),
			MaxTokens:    *maxTokens,
			TurboMode:    *turbo,
		}

		ctx, cancel := context.WithCancel(context.Background())

		var assistantResponse strings.Builder
		stats, err := eng.Generate(ctx, genParams, func(token string) bool {
			fmt.Print(token)
			assistantResponse.WriteString(token)
			return true
		})
		cancel()

		fmt.Println()
		if err != nil {
			fmt.Printf("\n[Error during generation: %v]\n", err)
		} else {
			conversationHistory.WriteString(assistantResponse.String() + "\n")
			fmt.Printf("%s\n\n", stats.Summary())
		}
	}
}
