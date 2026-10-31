//go:build windows
// +build windows

package engine

import (
	"syscall"
	"unsafe"
)

var (
	modPsapi                  = syscall.NewLazyDLL("psapi.dll")
	procGetProcessMemoryInfo  = modPsapi.NewProc("GetProcessMemoryInfo")
	modKernel32Mem            = syscall.NewLazyDLL("kernel32.dll")
	procGetCurrentProcess     = modKernel32Mem.NewProc("GetCurrentProcess")
)

type processMemoryCounters struct {
	cb                         uint32
	pageFaultCount             uint32
	peakWorkingSetSize         uintptr
	workingSetSize             uintptr
	quotaPeakPagedPoolUsage    uintptr
	quotaPagedPoolUsage        uintptr
	quotaPeakNonPagedPoolUsage uintptr
	quotaNonPagedPoolUsage     uintptr
	pagefileUsage              uintptr
	peakPagefileUsage          uintptr
}

// GetProcessMemoryUsage returns the exact current RSS (Working Set) RAM bytes without STW GC pauses.
func GetProcessMemoryUsage() uint64 {
	hProc, _, _ := procGetCurrentProcess.Call()
	if hProc == 0 {
		return 0
	}

	var pmc processMemoryCounters
	pmc.cb = uint32(unsafe.Sizeof(pmc))
	r1, _, _ := procGetProcessMemoryInfo.Call(hProc, uintptr(unsafe.Pointer(&pmc)), uintptr(pmc.cb))
	if r1 != 0 {
		return uint64(pmc.workingSetSize)
	}
	return 0
}
