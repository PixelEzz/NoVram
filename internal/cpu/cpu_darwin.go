//go:build darwin
// +build darwin

package cpu

import (
	"encoding/binary"
	"strings"
	"syscall"
)

func detectOS(info *Info) {
	// 1. CPU Brand String (e.g. "Apple M3 Pro" or Intel Core i9)
	if brand, err := sysctlString("machdep.cpu.brand_string"); err == nil && brand != "" {
		info.ModelName = brand
	}

	// 2. Total physical RAM (hw.memsize - 64-bit)
	if mem, err := sysctlUint64("hw.memsize"); err == nil && mem > 0 {
		info.TotalRAMBytes = mem
		// Approximation: on macOS, available RAM is roughly ~60% under normal load
		info.FreeRAMBytes = uint64(float64(mem) * 0.6)
	}

	// 3. Physical Cores (hw.physicalcpu)
	if cores, err := sysctlUint32("hw.physicalcpu"); err == nil && cores > 0 {
		info.PhysicalCores = int(cores)
	}

	// 4. Logical CPU threads (hw.logicalcpu)
	if logical, err := sysctlUint32("hw.logicalcpu"); err == nil && logical > 0 {
		info.LogicalThreads = int(logical)
	}

	// 5. Cache sizes
	if l3, err := sysctlUint64("hw.l3cachesize"); err == nil && l3 > 0 {
		info.L3CacheKB = int(l3 / 1024)
	}
	if l2, err := sysctlUint64("hw.l2cachesize"); err == nil && l2 > 0 {
		info.L2CacheKB = int(l2 / 1024)
	}
	if l1, err := sysctlUint64("hw.l1dcachesize"); err == nil && l1 > 0 {
		info.L1DataCacheKB = int(l1 / 1024)
	}

	// For Apple Silicon (which does not report unified system level cache as traditional L3),
	// fallback L3 cache recommendation heuristic to performance cluster L2 cache
	if info.L3CacheKB == 0 && info.L2CacheKB > 0 {
		info.L3CacheKB = info.L2CacheKB
	}
}

func sysctlString(name string) (string, error) {
	s, err := syscall.Sysctl(name)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(strings.TrimSpace(s), "\x00"), nil
}

func sysctlUint32(name string) (uint32, error) {
	s, err := syscall.Sysctl(name)
	if err != nil {
		return 0, err
	}
	b := []byte(s)
	if len(b) >= 4 {
		return binary.LittleEndian.Uint32(b[:4]), nil
	}
	return 0, syscall.EINVAL
}

func sysctlUint64(name string) (uint64, error) {
	s, err := syscall.Sysctl(name)
	if err != nil {
		return 0, err
	}
	b := []byte(s)
	if len(b) >= 8 {
		return binary.LittleEndian.Uint64(b[:8]), nil
	} else if len(b) >= 4 {
		return uint64(binary.LittleEndian.Uint32(b[:4])), nil
	}
	return 0, syscall.EINVAL
}
