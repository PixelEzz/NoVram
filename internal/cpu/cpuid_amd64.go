//go:build amd64
// +build amd64

package cpu

// cpuid is implemented in cpuid_amd64.s
func cpuid(eaxArg, ecxArg uint32) (eax, ebx, ecx, edx uint32)
