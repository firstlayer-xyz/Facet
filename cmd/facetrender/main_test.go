package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/firstlayer-xyz/meshio"
)

func TestPreviewBuffers_MeshPath(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "tri.3mf")
	m := &meshio.Mesh{
		Vertices:   []float32{0, 0, 0, 10, 0, 0, 0, 10, 0},
		Indices:    []uint32{0, 1, 2},
		FaceColors: []meshio.FaceColor{{Hex: "#00FF00"}},
	}
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Encode3MF(f); err != nil {
		t.Fatal(err)
	}
	f.Close()

	positions, colors := previewBuffers(p)
	if len(positions) != 9 {
		t.Fatalf("positions len = %d, want 9", len(positions))
	}
	if len(colors) != 9 {
		t.Fatalf("colors len = %d, want 9", len(colors))
	}
	if colors[1] != 0xFF { // green channel of first vertex
		t.Errorf("expected green, got %v", colors[0:3])
	}
}

func TestPreviewBuffers_MeshPathNoColor(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "tri.stl")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	m := &meshio.Mesh{
		Vertices: []float32{0, 0, 0, 10, 0, 0, 0, 10, 0},
		Indices:  []uint32{0, 1, 2},
	}
	if err := m.EncodeSTL(f); err != nil {
		t.Fatal(err)
	}
	f.Close()

	positions, colors := previewBuffers(p)
	if len(positions) != 9 {
		t.Fatalf("positions len = %d, want 9", len(positions))
	}
	if colors != nil {
		t.Errorf("STL colors = %v, want nil", colors)
	}
}
