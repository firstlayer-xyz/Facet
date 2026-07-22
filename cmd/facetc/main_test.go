package main

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/firstlayer-xyz/meshio"
)

func TestRenderMeshFile_PNG(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "tri.3mf")
	out := filepath.Join(dir, "tri.png")

	m := &meshio.Mesh{
		Geometry:   meshio.Geometry{Vertices: []float32{0, 0, 0, 10, 0, 0, 0, 10, 0}, Indices: []uint32{0, 1, 2}},
		FaceColors: []meshio.FaceColor{{Hex: "#FF0000"}},
	}
	f, err := os.Create(in)
	if err != nil {
		t.Fatal(err)
	}
	if err := meshio.Encode(f, m, "3mf"); err != nil {
		t.Fatal(err)
	}
	f.Close()

	if err := renderMeshFile(in, out, ".png", 128); err != nil {
		t.Fatalf("renderMeshFile: %v", err)
	}
	rf, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer rf.Close()
	img, err := png.Decode(rf)
	if err != nil {
		t.Fatalf("output is not a valid PNG: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 128 || b.Dy() != 128 {
		t.Errorf("size = %v, want 128x128", b)
	}

	// The triangle is #FF0000 — confirm color actually rendered (not gray).
	redFound := false
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y && !redFound; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA()
			if a > 0 && r > g+0x2000 && r > bl+0x2000 { // 16-bit channels
				redFound = true
				break
			}
		}
	}
	if !redFound {
		t.Error("expected a red-dominant pixel from the #FF0000 triangle")
	}
}
