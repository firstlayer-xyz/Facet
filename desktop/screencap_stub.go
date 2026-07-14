//go:build !darwin || crossbuild

package main

import "fmt"

// startWindowCapture is unavailable off macOS. A loud error, not a silent
// fallback: page-mode recording is a native-macOS capability.
func startWindowCapture(outPath string, pid, width, height int) error {
	return fmt.Errorf("window recording is only supported on macOS")
}

func startCompositeCapture(outPath string, pid, width, height int) error {
	return fmt.Errorf("screen recording is only supported on macOS")
}

func captureAddApp(appName string) error {
	return fmt.Errorf("screen recording is only supported on macOS")
}

func stopWindowCapture() error {
	return fmt.Errorf("window recording is only supported on macOS")
}

func captureWindowImage(outPath string, pid int) error {
	return fmt.Errorf("window screenshot is only supported on macOS")
}
