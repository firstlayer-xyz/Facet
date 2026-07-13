//go:build !darwin || crossbuild

package main

import "fmt"

// startWindowCapture is unavailable off macOS. A loud error, not a silent
// fallback: page-mode recording is a native-macOS capability.
func startWindowCapture(outPath string, pid int) error {
	return fmt.Errorf("window recording is only supported on macOS")
}

func stopWindowCapture() error {
	return fmt.Errorf("window recording is only supported on macOS")
}
