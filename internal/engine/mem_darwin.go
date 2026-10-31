//go:build darwin
// +build darwin

package engine

import (
	"runtime"
)

// GetProcessMemoryUsage returns approximate process memory on Darwin.
func GetProcessMemoryUsage() uint64 {
	var m runtime.MemStats
	// Sample without forced GC
	runtime.ReadMemStats(&m)
	return m.Sys
}
