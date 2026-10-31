package main

import (
	"fmt"
	"os"

	"github.com/novram-project/novram/internal/cpu"
	"github.com/novram-project/novram/internal/interceptor"
)

func runRecommend(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: novram recommend <model-name-or-alias>")
		fmt.Println("Example: novram recommend gpt-4o")
		fmt.Println("Example: novram recommend deepseek-r1")
		fmt.Println("Example: novram recommend llama-3.2-3b")
		os.Exit(1)
	}

	requestedModel := args[0]
	hw := cpu.Detect()
	decision := interceptor.InterceptAndOptimize(hw, requestedModel, 4096)

	fmt.Print(Banner)
	fmt.Println("🧠 NOVRAM OPTIMIZER INTERCEPTOR — MODEL RECOMMENDATION")
	fmt.Println("================================================================================")
	fmt.Printf("• Host CPU:             %s (%d physical cores, %s)\n", hw.ModelName, hw.PhysicalCores, hw.VectorSummary())
	fmt.Printf("• Available Memory:     %.2f GB (Total: %.2f GB)\n", decision.AvailableRAMGB, float64(hw.TotalRAMBytes)/(1024*1024*1024))
	fmt.Printf("• Requested Model:      %s\n", decision.OriginalModel)
	fmt.Println("--------------------------------------------------------------------------------")
	fmt.Printf("• Recommended Model:    %s\n", decision.SelectedModel)
	fmt.Printf("• Optimal Quantization: %s (Zero-Swap Guarantee)\n", decision.RecommendedQuant)
	fmt.Printf("• Recommended Context:  %d tokens\n", decision.RecommendedContext)
	fmt.Printf("• Estimated CPU Speed:  ~%.1f tokens/sec\n", decision.EstTokensPerSec)
	fmt.Printf("• Memory Working Set:   %.2f GB\n", decision.RequiredRAMGB)
	fmt.Printf("• Feasibility Status:   [%s]\n", decision.SafetyStatus)
	fmt.Printf("• Interceptor Note:     %s\n", decision.RecommendationNote)
	fmt.Println("================================================================================")
	fmt.Println("📥 DOWNLOAD URL:")
	fmt.Printf("   %s\n\n", decision.HuggingFaceDownload)
	fmt.Println("🚀 RUN WITH OPTIMAL SETTINGS:")
	fmt.Printf("   novram chat --model <path-to-%s-%s.gguf> --threads %d --ctx %d\n\n",
		decision.SelectedModel, decision.RecommendedQuant, hw.PhysicalCores, decision.RecommendedContext)
}
