package main

import (
	"fmt"
	"os"
	"path/filepath"

	"facet/pkg/facet3mf"
	"facet/pkg/manifold"

	"github.com/firstlayer-xyz/meshio"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// ExportMesh exports the last evaluated model in the given format (3MF, STL, or
// OBJ) via Manifold's pure-Go writers. Shows a native save dialog to choose the
// output path.
func (a *App) ExportMesh(format string, sources map[string]string, key string, entry string, overrides map[string]interface{}, embedSource bool) error {
	solids, err := evalSolids(a.ctx, evalRequest{Sources: sources, Key: key, Entry: entry, Overrides: overrides})
	if err != nil {
		return fmt.Errorf("eval failed: %w", err)
	}
	if len(solids) == 0 {
		return fmt.Errorf("no mesh to export — model produced no solids")
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

	switch format {
	case "3mf":
		var atts []meshio.Attachment
		if embedSource {
			att, err := facetProjectAttachment(sources, key, entry, overrides)
			if err != nil {
				return err
			}
			atts = []meshio.Attachment{att}
		}
		return manifold.Export3MFMulti(solids, path, atts)
	case "stl":
		return manifold.ExportSTLMulti(solids, path)
	case "obj":
		return manifold.ExportOBJMulti(solids, path)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

// facetProjectAttachment builds the embedded Facet payload from the entry-point
// file. The entry-point file is sources[key]; its absence is a hard error.
func facetProjectAttachment(sources map[string]string, key, entry string, overrides map[string]interface{}) (meshio.Attachment, error) {
	src, ok := sources[key]
	if !ok {
		return meshio.Attachment{}, fmt.Errorf("export: entry-point source %q not present in sources", key)
	}
	return facet3mf.Marshal(facet3mf.Project{
		Version:   facet3mf.Version,
		Entry:     entry,
		Overrides: overrides,
		Source:    src,
	})
}

// DetectSlicers returns the list of slicer applications found on the system.
func (a *App) DetectSlicers() []SlicerInfo {
	return detectSlicers()
}

// SendToSlicer re-evaluates the current program and exports the result as .3mf
// to a stable temp file, then opens it in the specified slicer application.
func (a *App) SendToSlicer(slicerID string, sources map[string]string, key string, entry string, overrides map[string]interface{}) error {
	solids, err := evalSolids(a.ctx, evalRequest{Sources: sources, Key: key, Entry: entry, Overrides: overrides})
	if err != nil {
		return fmt.Errorf("eval failed: %w", err)
	}
	if len(solids) == 0 {
		return fmt.Errorf("no mesh to export — model produced no solids")
	}

	path := filepath.Join(os.TempDir(), fmt.Sprintf("facet-slicer-%s-%d.3mf", slicerID, os.Getpid()))
	if err := manifold.Export3MFMulti(solids, path, nil); err != nil {
		return err
	}
	return launchSlicer(slicerID, path)
}
