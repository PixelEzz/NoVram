package cpu_test

import (
	"testing"

	"github.com/novram-project/novram/internal/cpu"
)

func TestDetect(t *testing.T) {
	info := cpu.Detect()

	if info.PhysicalCores <= 0 {
		t.Errorf("Expected PhysicalCores > 0, got %d", info.PhysicalCores)
	}

	if info.LogicalThreads <= 0 {
		t.Errorf("Expected LogicalThreads > 0, got %d", info.LogicalThreads)
	}

	if info.PhysicalCores > info.LogicalThreads {
		t.Errorf("PhysicalCores (%d) should not exceed LogicalThreads (%d)", info.PhysicalCores, info.LogicalThreads)
	}

	if info.ModelName == "" {
		t.Errorf("Expected non-empty ModelName")
	}

	summary := info.VectorSummary()
	if summary == "" {
		t.Errorf("Expected non-empty VectorSummary")
	}

	t.Logf("Detected CPU: %s", info.ModelName)
	t.Logf("Cores: %d physical / %d logical", info.PhysicalCores, info.LogicalThreads)
	t.Logf("Vector Flags: %s", summary)
	t.Logf("RAM: %.2f GB total, %.2f GB free",
		float64(info.TotalRAMBytes)/(1024*1024*1024),
		float64(info.FreeRAMBytes)/(1024*1024*1024))
	t.Logf("L3 Cache: %d KB", info.L3CacheKB)
}

func TestEvaluateModel(t *testing.T) {
	info := cpu.Detect()
	profile := cpu.StandardModelProfiles[1] // 3B
	quant := cpu.SupportedQuants[1]         // Q4_K_M

	rec := cpu.EvaluateModel(info, profile, quant, 4096)
	if rec.ModelWeightGB <= 0 {
		t.Errorf("Expected positive weight size, got %.2f", rec.ModelWeightGB)
	}
	if rec.KVCacheGB <= 0 {
		t.Errorf("Expected positive KV cache size, got %.2f", rec.KVCacheGB)
	}
	if rec.TotalRAMGB <= rec.ModelWeightGB {
		t.Errorf("Total RAM (%.2f) must exceed weight size (%.2f)", rec.TotalRAMGB, rec.ModelWeightGB)
	}
	if rec.EstimatedToks <= 0 {
		t.Errorf("Expected positive estimated tok/s, got %.2f", rec.EstimatedToks)
	}
}

func TestRecommendedBatchSize(t *testing.T) {
	info := cpu.Info{L3CacheKB: 16384}
	batch := cpu.RecommendedBatchSize(info)
	if batch != 512 {
		t.Errorf("Expected batch size 512 for 16MB L3 cache, got %d", batch)
	}

	infoLow := cpu.Info{L3CacheKB: 4096}
	batchLow := cpu.RecommendedBatchSize(infoLow)
	if batchLow != 256 {
		t.Errorf("Expected batch size 256 for 4MB L3 cache, got %d", batchLow)
	}
}
