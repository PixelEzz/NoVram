//go:build !windows && !linux && !darwin
// +build !windows,!linux,!darwin

package engine

import "runtime"

func GetProcessMemoryUsage() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Sys
}
