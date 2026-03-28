package main

import (
	"fmt"
	"os"
	"path/filepath"

	"facet/app/pkg/manifold"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// ExportMesh exports the last evaluated model in the given format.
// Uses Manifold's Assimp-backed I/O for export.
// Shows a native save dialog to choose the output path.
func (a *App) ExportMesh(format string) error {
	r := a.runner.LastResult()
	var solids []*manifold.Solid
	if r != nil {
		solids = r.Solids
	}
	if len(solids) == 0 {
		return fmt.Errorf("no mesh to export — run your code first")
	}

	var filter wailsRuntime.FileFilter
	var defaultName string
	switch format {
	case "3mf":
		filter = wailsRuntime.FileFilter{DisplayName: "3MF Files (*.3mf)", Pattern: "*.3mf"}
		defaultName = "export.3mf"
	case "stl":
		filter = wailsRuntime.FileFilter{DisplayName: "STL Files (*.stl)", Pattern: "*.stl"}
		defaultName = "export.stl"
	case "obj":
		filter = wailsRuntime.FileFilter{DisplayName: "OBJ Files (*.obj)", Pattern: "*.obj"}
		defaultName = "export.obj"
	case "glb":
		filter = wailsRuntime.FileFilter{DisplayName: "GLB Files (*.glb)", Pattern: "*.glb"}
		defaultName = "export.glb"
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}

	path, err := wailsRuntime.SaveFileDialog(a.ctx, wailsRuntime.SaveDialogOptions{
		DefaultFilename: defaultName,
		Filters:         []wailsRuntime.FileFilter{filter},
	})
	if err != nil {
		return err
	}
	if path == "" {
		return nil // user cancelled
	}

	// Use Go-native writers for 3MF/STL/OBJ; assimp for other formats.
	switch format {
	case "3mf":
		return manifold.Export3MFMulti(solids, path)
	case "stl":
		return manifold.ExportSTLMulti(solids, path)
	case "obj":
		return manifold.ExportOBJMulti(solids, path)
	default:
		return manifold.ExportMeshes(solids, path)
	}
}

// DetectSlicers returns the list of slicer applications found on the system.
func (a *App) DetectSlicers() []SlicerInfo {
	return detectSlicers()
}

// SendToSlicer exports the last evaluated model as .3mf to a stable temp file
// and opens it in the specified slicer application. Uses a fixed path per slicer
// so repeated sends reuse the already-open file in the slicer.
func (a *App) SendToSlicer(slicerID string) error {
	r := a.runner.LastResult()
	var solids []*manifold.Solid
	if r != nil {
		solids = r.Solids
	}
	if len(solids) == 0 {
		return fmt.Errorf("no mesh to export — run your code first")
	}

	// Stable path per instance: reuse the same file so the slicer can detect
	// updates, but include PID so multiple app instances don't collide.
	path := filepath.Join(os.TempDir(), fmt.Sprintf("facet-slicer-%s-%d.3mf", slicerID, os.Getpid()))
	if err := manifold.Export3MFMulti(solids, path); err != nil {
		return err
	}
	return launchSlicer(slicerID, path)
}
