package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/novram-project/novram/internal/engine"
	"github.com/novram-project/novram/internal/server"
)

func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	modelPathFlag := fs.String("model", "", "Path to GGUF model file (shorthand -m)")
	fs.StringVar(modelPathFlag, "m", "", "Path to GGUF model file")
	port := fs.Int("port", 8080, "HTTP server port (shorthand -p)")
	fs.IntVar(port, "p", 8080, "HTTP server port")
	host := fs.String("host", "0.0.0.0", "HTTP server host address")
	threads := fs.Int("threads", 0, "Number of inference threads (default: physical CPU cores)")
	fs.IntVar(threads, "t", 0, "Number of inference threads")
	ctxSize := fs.Int("ctx", 4096, "Context window length (shorthand -c)")
	fs.IntVar(ctxSize, "c", 4096, "Context window length")
	batchSize := fs.Int("batch", 0, "Batch size for prompt processing (default: tuned for L3 cache)")
	fs.IntVar(batchSize, "b", 0, "Batch size for prompt processing")
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

	if err := eng.Load(params); err != nil {
		fmt.Fprintf(os.Stderr, "Engine load failed: %v\n", err)
		os.Exit(1)
	}
	defer eng.Close()

	srv := server.NewServer(eng, *host, *port)
	if err := srv.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Server failed: %v\n", err)
		os.Exit(1)
	}
}
