package main

import (
	"fmt"
	"os"
)

const (
	Version = "0.1.0"
	Banner  = `
 ███╗   ██╗ ██████╗ ██╗   ██╗██████╗  █████╗ ███╗   ███╗
 ████╗  ██║██╔═══██╗██║   ██║██╔══██╗██╔══██╗████╗ ████║
 ██╔██╗ ██║██║   ██║██║   ██║██████╔╝███████║██╔████╔██║
 ██║╚██╗██║██║   ██║╚██╗ ██╔╝██╔══██╗██╔══██║██║╚██╔╝██║
 ██║ ╚████║╚██████╔╝ ╚████╔╝ ██║  ██║██║  ██║██║ ╚═╝ ██║
 ╚═╝  ╚═══╝ ╚═════╝   ╚═══╝  ╚═╝  ╚═╝╚═╝  ╚═╝╚═╝     ╚═╝
  Ultra-Fast, Zero-Dependency CPU-Only LLM Runner & Micro-Server
`
)

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "chat":
		runChat(args)
	case "serve":
		runServe(args)
	case "doctor":
		runDoctor(args)
	case "bench":
		runBench(args)
	case "version", "--version", "-v":
		fmt.Printf("novram version %s (cpu-only)\n", Version)
	case "help", "--help", "-h":
		printHelp()
	default:
		fmt.Printf("Unknown command '%s'. Run 'novram --help' for usage.\n", cmd)
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Print(Banner)
	fmt.Printf("novram v%s — High-performance CPU-only GGUF inference engine\n\n", Version)
	fmt.Println("USAGE:")
	fmt.Println("  novram <command> [options]")
	fmt.Println("\nCOMMANDS:")
	fmt.Println("  chat    Start an interactive terminal chat session with live tok/s stats")
	fmt.Println("  serve   Launch an OpenAI-compatible micro-server (/v1/chat/completions)")
	fmt.Println("  doctor  Inspect CPU vector flags, RAM, and recommend optimal GGUF quantization")
	fmt.Println("  bench   Benchmark raw prompt evaluation and token generation on CPU")
	fmt.Println("  version Print novram version")
	fmt.Println("\nRUN 'novram <command> --help' FOR MORE INFORMATION ON A SPECIFIC COMMAND.")
}
