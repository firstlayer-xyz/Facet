package manifold

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestExportMeshRoundTrip verifies that a successful export returns nil.
// This also indirectly exercises the Assimp export path through facet_cxx.
func TestExportMeshRoundTrip(t *testing.T) {
	cube, err := CreateCube(1, 1, 1)
	if err != nil {
		t.Fatalf("CreateCube: %v", err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "cube.stl")
	if err := ExportMesh(cube, path); err != nil {
		t.Fatalf("ExportMesh: %v", err)
	}
}

// TestExportMeshUnsupportedExtension verifies that Assimp's exporter surface
// rejects an unknown extension with an error. The error must come from the
// exporter itself — not a post-hoc Go-side check on the output file — so the
// user sees why the export failed instead of "file didn't appear."
func TestExportMeshUnsupportedExtension(t *testing.T) {
	cube, err := CreateCube(1, 1, 1)
	if err != nil {
		t.Fatalf("CreateCube: %v", err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "cube.this-is-not-a-real-format")
	err = ExportMesh(cube, path)
	if err == nil {
		t.Fatal("expected error for unsupported export format")
	}
	// Sanity: the error message should reference the target path so the user
	// can identify which export failed.
	if !strings.Contains(err.Error(), "cube.this-is-not-a-real-format") {
		t.Errorf("error should name the output path: %v", err)
	}
}

// TestExportMeshUnwritablePath verifies that Assimp's exporter reports
// failures writing to an unreachable directory.
func TestExportMeshUnwritablePath(t *testing.T) {
	cube, err := CreateCube(1, 1, 1)
	if err != nil {
		t.Fatalf("CreateCube: %v", err)
	}
	// A path under a non-existent directory — the filesystem should refuse
	// to create the file.
	path := filepath.Join(t.TempDir(), "no-such-subdir", "cube.stl")
	err = ExportMesh(cube, path)
	if err == nil {
		t.Fatal("expected error for unwritable path")
	}
}
