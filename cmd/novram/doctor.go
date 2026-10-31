package main

import (
	"fmt"

	"github.com/novram-project/novram/internal/cpu"
)

func runDoctor(args []string) {
	fmt.Print(Banner)
	fmt.Println("🏥 NOVRAM HARDWARE AUTO-TUNER & RECOMMENDER")
	fmt.Println("================================================================================")

	hw := cpu.Detect()

	totalRAMGB := float64(hw.TotalRAMBytes) / (1024 * 1024 * 1024)
	freeRAMGB := float64(hw.FreeRAMBytes) / (1024 * 1024 * 1024)
	if freeRAMGB <= 0 {
		freeRAMGB = totalRAMGB * 0.75
	}

	fmt.Printf("• CPU Model:         %s\n", hw.ModelName)
	fmt.Printf("• Core Topology:     %d Physical Cores | %d Logical Threads\n", hw.PhysicalCores, hw.LogicalThreads)
	fmt.Printf("                     (novram defaults inference threads strictly to %d physical cores\n", hw.PhysicalCores)
	fmt.Println("                      to prevent hyperthread cache thrashing and bus saturation)")
	fmt.Printf("• Vector Flags:      %s\n", hw.VectorSummary())
	fmt.Printf("• Cache Hierarchy:   L1: %d KB | L2: %d KB | L3: %d KB\n", hw.L1DataCacheKB, hw.L2CacheKB, hw.L3CacheKB)
	fmt.Printf("• System Memory:     Total: %.2f GB | Currently Available: %.2f GB\n", totalRAMGB, freeRAMGB)
	fmt.Printf("• Optimal Batch:     n_batch = %d (tuned for host L3 cache capacity)\n", cpu.RecommendedBatchSize(hw))
	fmt.Println("================================================================================")
	fmt.Println("\n📊 GGUF FEASIBILITY & PERFORMANCE MATRIX (Zero-Swap Guarantee):")
	fmt.Println("--------------------------------------------------------------------------------")
	fmt.Printf("%-24s %-8s %-6s %-9s %-8s %-10s %-12s\n", "Model Size", "Quant", "Ctx", "Weights", "KV Cache", "Total RAM", "Est Speed")
	fmt.Println("--------------------------------------------------------------------------------")

	var bestRecommendation *cpu.ModelRecommendation

	ctxSizes := []int{4096, 8192}
	for _, profile := range cpu.StandardModelProfiles {
		for _, quant := range cpu.SupportedQuants {
			for _, ctx := range ctxSizes {
				rec := cpu.EvaluateModel(hw, profile, quant, ctx)
				if ctx == 4096 {
					fmt.Printf("%-24s %-8s %-6d %-6.2f GB %-6.2f GB %s %-6.2f GB %-5.1f tok/s\n",
						rec.ModelName, rec.Quant, rec.ContextSize, rec.ModelWeightGB, rec.KVCacheGB,
						rec.StatusSymbol, rec.TotalRAMGB, rec.EstimatedToks)
				}

				if rec.Status == cpu.StatusOptimal && (bestRecommendation == nil || profile.Parameters > 3.0) {
					recCopy := rec
					bestRecommendation = &recCopy
				}
			}
		}
	}

	fmt.Println("--------------------------------------------------------------------------------")
	fmt.Println("Legend: 🟢 OPTIMAL (Zero swap, peak tok/s) | 🟡 TIGHT (Close other apps) | 🔴 SWAPS (High latency)")
	fmt.Println("================================================================================")

	if bestRecommendation != nil {
		fmt.Println("\n🎯 RECOMMENDED CONFIGURATION FOR THIS MACHINE:")
		fmt.Printf("   Model:       %s (%s)\n", bestRecommendation.ModelName, bestRecommendation.Quant)
		fmt.Printf("   Context:     %d tokens (KV cache: %.2f GB)\n", bestRecommendation.ContextSize, bestRecommendation.KVCacheGB)
		fmt.Printf("   Peak RAM:    %.2f GB (safely fits in %.2f GB available RAM)\n", bestRecommendation.TotalRAMGB, freeRAMGB)
		fmt.Printf("   Est. Speed:  ~%.1f tokens/sec on %d physical CPU cores\n", bestRecommendation.EstimatedToks, hw.PhysicalCores)
		fmt.Println("\n🚀 QUICKSTART RUN COMMAND:")
		fmt.Printf("   %s\n\n", bestRecommendation.RecommendedCmd)
	}
}
