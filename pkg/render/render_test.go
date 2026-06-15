package render

import (
	"image"
	"testing"
)

// countOpaque returns how many pixels have a non-zero alpha (i.e. were drawn).
func countOpaque(img *image.RGBA) int {
	n := 0
	for i := 3; i < len(img.Pix); i += 4 {
		if img.Pix[i] > 0 {
			n++
		}
	}
	return n
}

// unitCubeTris returns the 12 outward-wound (CCW-from-outside) triangles of a
// unit cube as expanded positions (9 floats per triangle).
func unitCubeTris() []float32 {
	v := [8][3]float32{
		{0, 0, 0}, {1, 0, 0}, {1, 1, 0}, {0, 1, 0},
		{0, 0, 1}, {1, 0, 1}, {1, 1, 1}, {0, 1, 1},
	}
	faces := [12][3]int{
		{0, 2, 1}, {0, 3, 2}, // bottom (-z)
		{4, 5, 6}, {4, 6, 7}, // top (+z)
		{0, 1, 5}, {0, 5, 4}, // front (-y)
		{2, 3, 7}, {2, 7, 6}, // back (+y)
		{1, 2, 6}, {1, 6, 5}, // right (+x)
		{0, 4, 7}, {0, 7, 3}, // left (-x)
	}
	out := make([]float32, 0, 12*9)
	for _, f := range faces {
		for _, idx := range f {
			out = append(out, v[idx][0], v[idx][1], v[idx][2])
		}
	}
	return out
}

// A cube renders at the requested size with some pixels actually drawn.
func TestMeshRendersTriangles(t *testing.T) {
	img := Mesh(unitCubeTris(), 64, 48)
	if img.Bounds().Dx() != 64 || img.Bounds().Dy() != 48 {
		t.Fatalf("size = %v, want 64x48", img.Bounds().Size())
	}
	if n := countOpaque(img); n == 0 {
		t.Fatal("expected some rendered (opaque) pixels, got none")
	}
}

// No geometry → a fully transparent image, not a panic.
func TestMeshEmpty(t *testing.T) {
	if n := countOpaque(Mesh(nil, 32, 32)); n != 0 {
		t.Fatalf("empty mesh should be fully transparent, got %d opaque pixels", n)
	}
}

// Degenerate dimensions don't panic.
func TestMeshZeroSize(t *testing.T) {
	_ = Mesh(unitCubeTris(), 0, 0)
}
