//go:build !windows && !linux && !darwin
// +build !windows,!linux,!darwin

package cpu

func detectOS(info *Info) {
	// Fallback when OS-specific hooks are unavailable
}
