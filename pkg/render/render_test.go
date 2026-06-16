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
	img := Mesh(unitCubeTris(), nil, 64, 48)
	if img.Bounds().Dx() != 64 || img.Bounds().Dy() != 48 {
		t.Fatalf("size = %v, want 64x48", img.Bounds().Size())
	}
	if n := countOpaque(img); n == 0 {
		t.Fatal("expected some rendered (opaque) pixels, got none")
	}
}

// No geometry → a fully transparent image, not a panic.
func TestMeshEmpty(t *testing.T) {
	if n := countOpaque(Mesh(nil, nil, 32, 32)); n != 0 {
		t.Fatalf("empty mesh should be fully transparent, got %d opaque pixels", n)
	}
}

// Degenerate dimensions don't panic.
func TestMeshZeroSize(t *testing.T) {
	_ = Mesh(unitCubeTris(), nil, 0, 0)
}

// A single large triangle facing the camera; sample the image center.
func centerColorOf(t *testing.T, colors []byte) (r, g, b uint8) {
	t.Helper()
	positions := []float32{-10, -10, 0, 10, -10, 0, 0, 10, 0}
	img := Mesh(positions, colors, 64, 64)
	c := img.RGBAAt(32, 32)
	return c.R, c.G, c.B
}

func TestMesh_NilColorsIsGray(t *testing.T) {
	r, g, b := centerColorOf(t, nil)
	if r == 0 && g == 0 && b == 0 {
		t.Fatal("center is transparent/black; expected a shaded gray triangle")
	}
	// Default base is near-neutral: channels close to each other.
	if int(b)-int(r) > 40 {
		t.Errorf("nil colors should be near-neutral gray, got R=%d G=%d B=%d", r, g, b)
	}
	if int(g)-int(r) > 40 {
		t.Errorf("nil colors should be near-neutral gray, got R=%d G=%d B=%d", r, g, b)
	}
}

func TestMesh_PerTriangleColorsDiffer(t *testing.T) {
	// Two separated triangles: left one red, right one blue.
	positions := []float32{
		-30, -10, 0, -10, -10, 0, -20, 10, 0, // left triangle
		10, -10, 0, 30, -10, 0, 20, 10, 0, // right triangle
	}
	colors := []byte{
		255, 0, 0, 255, 0, 0, 255, 0, 0, // left → red
		0, 0, 255, 0, 0, 255, 0, 0, 255, // right → blue
	}
	img := Mesh(positions, colors, 128, 128)
	b := img.Bounds()
	redLeft, blueRight := false, false
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := img.RGBAAt(x, y)
			if c.A == 0 {
				continue
			}
			if x < b.Dx()/2 && c.R > c.G+40 && c.R > c.B+40 {
				redLeft = true
			}
			if x >= b.Dx()/2 && c.B > c.R+40 && c.B > c.G+40 {
				blueRight = true
			}
		}
	}
	if !redLeft {
		t.Error("expected a red-dominant pixel in the left half (left triangle)")
	}
	if !blueRight {
		t.Error("expected a blue-dominant pixel in the right half (right triangle)")
	}
}

func TestMesh_RedColorsRender(t *testing.T) {
	// Per-expanded-vertex RGB, red for all three verts of the one triangle.
	colors := []byte{255, 0, 0, 255, 0, 0, 255, 0, 0}
	r, g, b := centerColorOf(t, colors)
	if !(r > g+40 && r > b+40) {
		t.Errorf("expected red-dominant pixel, got R=%d G=%d B=%d", r, g, b)
	}
}
