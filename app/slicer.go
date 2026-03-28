package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// SlicerInfo describes a detected slicer application.
type SlicerInfo struct {
	Name string `json:"name"` // "BambuStudio", "OrcaSlicer", "PrusaSlicer"
	ID   string `json:"id"`   // "bambu", "orca", "prusa"
}

type slicerDef struct {
	Name string
	ID   string
	// macOS
	MacApp string
	// Linux
	LinuxBin string
	// Windows
	WinExe string
}

var slicerDefs = []slicerDef{
	{
		Name:     "BambuStudio",
		ID:       "bambu",
		MacApp:   "BambuStudio",
		LinuxBin: "bambu-studio",
		WinExe:   `BambuStudio\BambuStudio.exe`,
	},
	{
		Name:     "OrcaSlicer",
		ID:       "orca",
		MacApp:   "OrcaSlicer",
		LinuxBin: "orca-slicer",
		WinExe:   `OrcaSlicer\orca-slicer.exe`,
	},
	{
		Name:     "PrusaSlicer",
		ID:       "prusa",
		MacApp:   "PrusaSlicer",
		LinuxBin: "prusa-slicer",
		WinExe:   `Prusa3D\PrusaSlicer\prusa-slicer.exe`,
	},
	{
		Name:     "UltiMaker Cura",
		ID:       "cura",
		MacApp:   "UltiMaker Cura",
		LinuxBin: "cura",
		WinExe:   `UltiMaker Cura\UltiMaker-Cura.exe`,
	},
	{
		Name:     "AnycubicSlicer",
		ID:       "anycubic",
		MacApp:   "AnycubicSlicer",
		LinuxBin: "anycubic-slicer",
		WinExe:   `AnycubicSlicer\AnycubicSlicer.exe`,
	},
	{
		Name:     "Anycubic Photon Workshop",
		ID:       "photon-workshop",
		MacApp:   "Anycubic Photon Workshop",
		LinuxBin: "photon-workshop",
		WinExe:   `Anycubic\Anycubic Photon Workshop\Anycubic Photon Workshop.exe`,
	},
}

// findMacApp searches /Applications and ~/Applications recursively for name.app.
// Returns the full path or "" if not found.
func findMacApp(name string) string {
	target := name + ".app"
	home, _ := os.UserHomeDir()
	dirs := []string{"/Applications"}
	if home != "" {
		dirs = append(dirs, filepath.Join(home, "Applications"))
	}
	for _, root := range dirs {
		var found string
		filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return filepath.SkipDir
			}
			if d.IsDir() && d.Name() == target {
				found = path
				return filepath.SkipAll
			}
			// Don't descend into .app bundles
			if d.IsDir() && filepath.Ext(d.Name()) == ".app" {
				return filepath.SkipDir
			}
			return nil
		})
		if found != "" {
			return found
		}
	}
	return ""
}

func detectSlicers() []SlicerInfo {
	var found []SlicerInfo
	for _, d := range slicerDefs {
		if slicerExists(d) {
			found = append(found, SlicerInfo{Name: d.Name, ID: d.ID})
		}
	}
	return found
}

func slicerExists(d slicerDef) bool {
	switch runtime.GOOS {
	case "darwin":
		return findMacApp(d.MacApp) != ""
	case "linux":
		_, err := exec.LookPath(d.LinuxBin)
		return err == nil
	case "windows":
		for _, base := range []string{
			os.Getenv("ProgramFiles"),
			os.Getenv("ProgramFiles(x86)"),
		} {
			if base == "" {
				continue
			}
			if _, err := os.Stat(filepath.Join(base, d.WinExe)); err == nil {
				return true
			}
		}
		return false
	}
	return false
}

func slicerDefByID(id string) (slicerDef, bool) {
	for _, d := range slicerDefs {
		if d.ID == id {
			return d, true
		}
	}
	return slicerDef{}, false
}

func launchSlicer(id string, filePath string) error {
	d, ok := slicerDefByID(id)
	if !ok {
		return fmt.Errorf("unknown slicer: %s", id)
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		appPath := findMacApp(d.MacApp)
		if appPath == "" {
			return fmt.Errorf("%s not found", d.Name)
		}
		cmd = exec.Command("open", "-a", appPath, filePath)
	case "linux":
		cmd = exec.Command(d.LinuxBin, filePath)
	case "windows":
		var bin string
		for _, base := range []string{
			os.Getenv("ProgramFiles"),
			os.Getenv("ProgramFiles(x86)"),
		} {
			if base == "" {
				continue
			}
			p := filepath.Join(base, d.WinExe)
			if _, err := os.Stat(p); err == nil {
				bin = p
				break
			}
		}
		if bin == "" {
			return fmt.Errorf("%s not found", d.Name)
		}
		cmd = exec.Command(bin, filePath)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait() // reap child process to avoid zombie
	return nil
}
