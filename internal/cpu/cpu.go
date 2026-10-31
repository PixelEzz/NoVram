package cpu

import (
	"fmt"
	"runtime"
	"strings"
)

// Info contains comprehensive host hardware information relevant to CPU inference.
type Info struct {
	ModelName      string
	PhysicalCores  int
	LogicalThreads int
	L1DataCacheKB  int
	L2CacheKB      int
	L3CacheKB      int
	TotalRAMBytes  uint64
	FreeRAMBytes   uint64

	// Vector Instruction Support
	HasAVX    bool
	HasAVX2   bool
	HasAVX512 bool
	HasFMA    bool
	HasNEON   bool
}

// Detect gathers hardware topology, vector extensions, and memory statistics.
func Detect() Info {
	info := Info{
		LogicalThreads: runtime.NumCPU(),
	}

	// OS-specific core, memory, and cache detection
	detectOS(&info)

	// Architecture-specific instruction set and brand detection
	detectVectorAndBrand(&info)

	// Fallback safety: ensure physical cores is at least 1 and does not exceed logical threads
	if info.PhysicalCores <= 0 {
		if info.LogicalThreads > 1 {
			info.PhysicalCores = info.LogicalThreads / 2
		} else {
			info.PhysicalCores = 1
		}
	}
	if info.PhysicalCores > info.LogicalThreads {
		info.PhysicalCores = info.LogicalThreads
	}

	return info
}

// VectorSummary returns a comma-separated list of detected high-performance vector instructions.
func (i Info) VectorSummary() string {
	var list []string
	if i.HasAVX512 {
		list = append(list, "AVX-512")
	}
	if i.HasAVX2 {
		list = append(list, "AVX2")
	}
	if i.HasAVX && !i.HasAVX2 {
		list = append(list, "AVX")
	}
	if i.HasFMA {
		list = append(list, "FMA")
	}
	if i.HasNEON {
		list = append(list, "ARM NEON")
	}
	if len(list) == 0 {
		return "Basic SSE / Scalar"
	}
	return strings.Join(list, ", ")
}

// detectVectorAndBrand queries CPUID for x86_64 brand string and instruction set flags.
func detectVectorAndBrand(info *Info) {
	if runtime.GOARCH == "amd64" {
		// 1. Check max CPUID level
		maxID, _, _, _ := cpuid(0, 0)

		// 2. Query Leaf 1 for AVX, FMA, SSE flags
		if maxID >= 1 {
			_, _, ecx1, edx1 := cpuid(1, 0)
			info.HasAVX = (ecx1 & (1 << 28)) != 0
			info.HasFMA = (ecx1 & (1 << 12)) != 0
			_ = edx1
		}

		// 3. Query Leaf 7, Subleaf 0 for AVX2 and AVX-512 flags
		if maxID >= 7 {
			_, ebx7, _, _ := cpuid(7, 0)
			info.HasAVX2 = (ebx7 & (1 << 5)) != 0
			// AVX-512 Foundation (bit 16)
			info.HasAVX512 = (ebx7 & (1 << 16)) != 0
		}

		// 4. Query Extended Leaves 0x80000000..0x80000004 for Brand String
		maxExt, _, _, _ := cpuid(0x80000000, 0)
		if maxExt >= 0x80000004 {
			var brandBytes [48]byte
			off := 0
			for leaf := uint32(0x80000002); leaf <= 0x80000004; leaf++ {
				eax, ebx, ecx, edx := cpuid(leaf, 0)
				regs := []uint32{eax, ebx, ecx, edx}
				for _, r := range regs {
					brandBytes[off] = byte(r & 0xFF)
					brandBytes[off+1] = byte((r >> 8) & 0xFF)
					brandBytes[off+2] = byte((r >> 16) & 0xFF)
					brandBytes[off+3] = byte((r >> 24) & 0xFF)
					off += 4
				}
			}
			brand := strings.TrimSpace(string(brandBytes[:]))
			brand = strings.ReplaceAll(brand, "\x00", "")
			if brand != "" {
				info.ModelName = brand
			}
		}
	} else if runtime.GOARCH == "arm64" {
		info.HasNEON = true
		if info.ModelName == "" {
			info.ModelName = fmt.Sprintf("ARM64 (%s)", runtime.GOOS)
		}
	}

	if info.ModelName == "" {
		info.ModelName = fmt.Sprintf("Generic CPU (%s/%s)", runtime.GOOS, runtime.GOARCH)
	}
}
