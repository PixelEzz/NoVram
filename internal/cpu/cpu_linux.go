//go:build linux
// +build linux

package cpu

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func detectOS(info *Info) {
	// 1. Memory stats from /proc/meminfo
	if f, err := os.Open("/proc/meminfo"); err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				key := strings.TrimSuffix(fields[0], ":")
				valKB, _ := strconv.ParseUint(fields[1], 10, 64)
				switch key {
				case "MemTotal":
					info.TotalRAMBytes = valKB * 1024
				case "MemAvailable":
					info.FreeRAMBytes = valKB * 1024
				}
			}
		}
	}

	// 2. CPU Model Name from /proc/cpuinfo (especially useful for ARM64 and virtualized Linux)
	if f, err := os.Open("/proc/cpuinfo"); err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				if (key == "model name" || key == "Model" || key == "Hardware") && val != "" {
					if info.ModelName == "" || strings.HasPrefix(info.ModelName, "Generic") {
						info.ModelName = val
					}
					break
				}
			}
		}
	}

	// 3. Physical cores from /sys/devices/system/cpu/cpu*/topology
	coreMap := make(map[string]bool)
	matches, _ := filepath.Glob("/sys/devices/system/cpu/cpu[0-9]*/topology/core_id")
	for _, coreFile := range matches {
		cpuDir := filepath.Dir(coreFile)
		pkgFile := filepath.Join(cpuDir, "physical_package_id")

		coreIDBytes, err1 := os.ReadFile(coreFile)
		pkgIDBytes, err2 := os.ReadFile(pkgFile)

		coreID := strings.TrimSpace(string(coreIDBytes))
		pkgID := "0"
		if err2 == nil {
			pkgID = strings.TrimSpace(string(pkgIDBytes))
		}

		if err1 == nil && coreID != "" {
			key := fmt.Sprintf("%s:%s", pkgID, coreID)
			coreMap[key] = true
		}
	}
	if len(coreMap) > 0 {
		info.PhysicalCores = len(coreMap)
	}

	// 4. Cache sizes from /sys/devices/system/cpu/cpu0/cache/index*/size
	parseCache := func(index int) int {
		path := fmt.Sprintf("/sys/devices/system/cpu/cpu0/cache/index%d/size", index)
		data, err := os.ReadFile(path)
		if err != nil {
			return 0
		}

		s := strings.ToUpper(strings.TrimSpace(string(data)))
		s = strings.TrimSuffix(s, "B")
		s = strings.TrimSuffix(s, "I")

		if strings.HasSuffix(s, "K") {
			val, _ := strconv.Atoi(strings.TrimSuffix(s, "K"))
			return val
		} else if strings.HasSuffix(s, "M") {
			val, _ := strconv.Atoi(strings.TrimSuffix(s, "M"))
			return val * 1024
		} else if val, err := strconv.Atoi(s); err == nil {
			// Raw bytes
			return val / 1024
		}
		return 0
	}

	info.L1DataCacheKB = parseCache(0)
	info.L2CacheKB = parseCache(2)
	info.L3CacheKB = parseCache(3)
}
