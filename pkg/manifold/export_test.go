//go:build !js

package manifold

import (
	"path/filepath"
	"testing"
)

// TestExportSTLRoundTrip verifies that a successful STL export returns nil.
// This exercises the manifold-level export path: mesh extraction → meshio's
// pure-Go STL writer.
func TestExportSTLRoundTrip(t *testing.T) {
	cube, err := CreateCube(1, 1, 1)
	if err != nil {
		t.Fatalf("CreateCube: %v", err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "cube.stl")
	if err := ExportSTL(cube, path); err != nil {
		t.Fatalf("ExportSTL: %v", err)
	}
}

// TestExportSTLUnwritablePath verifies that the writer reports failures writing
// to an unreachable directory rather than silently succeeding.
func TestExportSTLUnwritablePath(t *testing.T) {
	cube, err := CreateCube(1, 1, 1)
	if err != nil {
		t.Fatalf("CreateCube: %v", err)
	}
	// A path under a non-existent directory — the filesystem should refuse
	// to create the file.
	path := filepath.Join(t.TempDir(), "no-such-subdir", "cube.stl")
	if err := ExportSTL(cube, path); err == nil {
		t.Fatal("expected error for unwritable path")
	}
}
