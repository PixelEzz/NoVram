//go:build darwin
// +build darwin

package cpu

import (
	"syscall"
	"unsafe"
)

func detectOS(info *Info) {
	// 1. Total physical RAM (hw.memsize)
	if val, err := syscall.SysctlUint64("hw.memsize"); err == nil && val > 0 {
		info.TotalRAMBytes = val
		// Approximation: 60% free if vm_stat not parsed
		info.FreeRAMBytes = uint64(float64(val) * 0.6)
	}

	// 2. Physical Cores (hw.physicalcpu)
	if val, err := syscall.SysctlUint32("hw.physicalcpu"); err == nil && val > 0 {
		info.PhysicalCores = int(val)
	}

	// 3. Cache sizes
	if val, err := syscall.SysctlUint32("hw.l3cachesize"); err == nil && val > 0 {
		info.L3CacheKB = int(val / 1024)
	}
	if val, err := syscall.SysctlUint32("hw.l2cachesize"); err == nil && val > 0 {
		info.L2CacheKB = int(val / 1024)
	}
	if val, err := syscall.SysctlUint32("hw.l1dcachesize"); err == nil && val > 0 {
		info.L1DataCacheKB = int(val / 1024)
	}
}
