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
	systemPrompt := fs.String("system", "You are a helpful, ultra-fast AI assistant running on CPU via novram.", "System prompt")
	fs.StringVar(systemPrompt, "s", "You are a helpful, ultra-fast AI assistant running on CPU via novram.", "System prompt")
	useMock := fs.Bool("mock", false, "Use fast in-memory mock engine (for testing without GGUF weights)")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	modelPath, err := ResolveModelPath(*modelPathFlag, *useMock)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	params, hw := PrepareEngineParams(modelPath, *threads, *batchSize, *ctxSize)
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
			MaxTokens:    1024,
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
