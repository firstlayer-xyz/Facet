//go:build !darwin || crossbuild || !automation

package main

import "fmt"

// Native screen capture (ScreenCaptureKit) is a macOS-only, demo-automation-only
// capability: it needs the macOS 15 SDK and is compiled in only under the
// `automation` build tag. Off macOS, or in a normal (shipped) build, these are
// loud errors rather than silent fallbacks. In practice they are unreachable —
// the only callers are the automation-driven App recording methods, which the
// shipped app never invokes.
var errCaptureUnavailable = fmt.Errorf("native screen capture is unavailable in this build (macOS automation builds only)")

func startWindowCapture(outPath string, pid, width, height int) error { return errCaptureUnavailable }

func startCompositeCapture(outPath string, pid, width, height int) error {
	return errCaptureUnavailable
}

func captureAddApp(appName string) error { return errCaptureUnavailable }

func stopWindowCapture() error { return errCaptureUnavailable }

func captureWindowImage(outPath string, pid int) error { return errCaptureUnavailable }
