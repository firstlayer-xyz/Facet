package main

import (
	"fmt"
	"os/exec"
	"runtime"
)

// revealInFileManager opens path in the OS file manager and activates the window.
func revealInFileManager(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		// -R reveals the item in Finder (selects it); Finder comes to front automatically.
		cmd = exec.Command("open", "-R", path)
	case "linux":
		cmd = exec.Command("xdg-open", path)
	case "windows":
		// /select highlights the item in Explorer.
		cmd = exec.Command("explorer", "/select,", path)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait() // reap child process to avoid zombie
	return nil
}
