//go:build windows
// +build windows

package cpu

import (
	"syscall"
	"unsafe"
)

var (
	modKernel32                       = syscall.NewLazyDLL("kernel32.dll")
	procGlobalMemoryStatusEx          = modKernel32.NewProc("GlobalMemoryStatusEx")
	procGetLogicalProcessorInfo       = modKernel32.NewProc("GetLogicalProcessorInformation")
)

type memoryStatusEx struct {
	cbSize                  uint32
	dwMemoryLoad            uint32
	ullTotalPhys            uint64
	ullAvailPhys            uint64
	ullTotalPageFile        uint64
	ullAvailPageFile        uint64
	ullTotalVirtual         uint64
	ullAvailVirtual         uint64
	ullAvailExtendedVirtual uint64
}

type cacheDescriptor struct {
	level         uint8
	associativity uint8
	lineSize      uint16
	size          uint32
	cacheType     uint32
}

type systemLogicalProcessorInformation struct {
	processorMask uintptr
	relationship  uint32
	padding       uint32
	data          [16]byte
}

const (
	relationProcessorCore = 0
	relationCache         = 2
)

func detectOS(info *Info) {
	// 1. Memory stats via GlobalMemoryStatusEx
	var mem memoryStatusEx
	mem.cbSize = uint32(unsafe.Sizeof(mem))
	r1, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&mem)))
	if r1 != 0 {
		info.TotalRAMBytes = mem.ullTotalPhys
		info.FreeRAMBytes = mem.ullAvailPhys
	}

	// 2. Physical cores and cache via GetLogicalProcessorInformation
	var returnLength uint32
	// First call to determine required buffer size
	procGetLogicalProcessorInfo.Call(0, uintptr(unsafe.Pointer(&returnLength)))
	if returnLength > 0 {
		buf := make([]byte, returnLength)
		r2, _, _ := procGetLogicalProcessorInfo.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&returnLength)))
		if r2 != 0 {
			recordSize := unsafe.Sizeof(systemLogicalProcessorInformation{})
			count := int(returnLength / uint32(recordSize))
			physicalCores := 0

			for i := 0; i < count; i++ {
				rec := (*systemLogicalProcessorInformation)(unsafe.Pointer(&buf[uintptr(i)*recordSize]))
				if rec.relationship == relationProcessorCore {
					physicalCores++
				} else if rec.relationship == relationCache {
					cache := (*cacheDescriptor)(unsafe.Pointer(&rec.data[0]))
					if cache.level == 1 && cache.size > 0 {
						info.L1DataCacheKB = int(cache.size / 1024)
					} else if cache.level == 2 && cache.size > 0 {
						info.L2CacheKB = int(cache.size / 1024)
					} else if cache.level == 3 && cache.size > 0 {
						info.L3CacheKB = int(cache.size / 1024)
					}
				}
			}

			if physicalCores > 0 {
				info.PhysicalCores = physicalCores
			}
		}
	}
}
