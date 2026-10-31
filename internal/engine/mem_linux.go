//go:build linux
// +build linux

package engine

import (
	"os"
	"strconv"
	"strings"
)

// GetProcessMemoryUsage returns the exact current RSS RAM bytes on Linux via /proc/self/statm.
func GetProcessMemoryUsage() uint64 {
	data, err := os.ReadFile("/proc/self/statm")
	if err != nil {
		return 0
	}

	fields := strings.Fields(string(data))
	if len(fields) >= 2 {
		residentPages, err := strconv.ParseUint(fields[1], 10, 64)
		if err == nil {
			return residentPages * uint64(os.Getpagesize())
		}
	}
	return 0
}
