package meshpreview

import (
	"os"
	"path/filepath"
	"testing"

	"facet/pkg/manifold"

	"github.com/firstlayer-xyz/meshio"
)

// two triangles
var triVerts = []float32{0, 0, 0, 1, 0, 0, 0, 1, 0, 0, 0, 0, 0, 1, 0, 0, 0, 1}
var triIdx = []uint32{0, 1, 2, 3, 4, 5}

func TestMeshToPreview_Colors(t *testing.T) {
	m := &meshio.Mesh{
		Vertices:   triVerts,
		Indices:    triIdx,
		FaceColors: []meshio.FaceColor{{Hex: "#FF0000"}, {Hex: "#00FF00"}},
	}
	pos, cols, err := meshToPreview(m)
	if err != nil {
		t.Fatal(err)
	}
	if len(pos) != 18 { // 2 tris * 9
		t.Fatalf("positions len = %d, want 18", len(pos))
	}
	if len(cols) != 18 { // 2 tris * 3 verts * 3 bytes
		t.Fatalf("colors len = %d, want 18", len(cols))
	}
	// triangle 0 → red on all three verts
	for v := 0; v < 3; v++ {
		if cols[v*3] != 0xFF || cols[v*3+1] != 0 || cols[v*3+2] != 0 {
			t.Errorf("tri0 vert %d = %v, want red", v, cols[v*3:v*3+3])
		}
	}
	// triangle 1 → green
	if cols[9] != 0 || cols[10] != 0xFF || cols[11] != 0 {
		t.Errorf("tri1 vert0 = %v, want green", cols[9:12])
	}
}

func TestMeshToPreview_NoColors(t *testing.T) {
	m := &meshio.Mesh{Vertices: triVerts, Indices: triIdx}
	pos, cols, err := meshToPreview(m)
	if err != nil {
		t.Fatal(err)
	}
	if len(pos) != 18 {
		t.Fatalf("positions len = %d, want 18", len(pos))
	}
	if cols != nil {
		t.Errorf("colors = %v, want nil", cols)
	}
}

// A triangle index past the vertex array must error, not panic — the OBJ/3MF
// decoders don't bounds-check indices and this runs over arbitrary files.
func TestMeshToPreview_IndexOutOfRange(t *testing.T) {
	m := &meshio.Mesh{
		Vertices: triVerts,           // 6 vertices
		Indices:  []uint32{0, 1, 99}, // 99 is out of range
	}
	if _, _, err := meshToPreview(m); err == nil {
		t.Fatal("expected an out-of-range error, got nil")
	}
}

// A face-color count that doesn't match the triangle count must error rather
// than silently dropping color.
func TestMeshToPreview_ColorCountMismatch(t *testing.T) {
	m := &meshio.Mesh{
		Vertices:   triVerts,
		Indices:    triIdx,                               // 2 triangles
		FaceColors: []meshio.FaceColor{{Hex: "#FF0000"}}, // only 1 color
	}
	if _, _, err := meshToPreview(m); err == nil {
		t.Fatal("expected a color-count mismatch error, got nil")
	}
}

func TestLoadColored_STLHasNoColor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tri.stl")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	m := &meshio.Mesh{Vertices: triVerts, Indices: triIdx}
	if err := m.EncodeSTL(f); err != nil {
		t.Fatal(err)
	}
	f.Close()

	pos, cols, err := LoadColored(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(pos) != 18 {
		t.Errorf("positions len = %d, want 18", len(pos))
	}
	if cols != nil {
		t.Errorf("STL colors = %v, want nil", cols)
	}
}

func TestMeshToPreview_DefaultFallback(t *testing.T) {
	m := &meshio.Mesh{
		Vertices:   triVerts,
		Indices:    triIdx,
		FaceColors: []meshio.FaceColor{{Hex: "#FF0000"}, {Hex: ""}},
	}
	_, cols, err := meshToPreview(m)
	if err != nil {
		t.Fatal(err)
	}
	if cols == nil || len(cols) != 18 {
		t.Fatalf("colors len = %d, want 18", len(cols))
	}
	d := manifold.DefaultFaceColor
	// tri1 (verts 3..5) has empty hex → DefaultFaceColor
	if cols[9] != d[0] || cols[10] != d[1] || cols[11] != d[2] {
		t.Errorf("empty-hex tri = %v, want DefaultFaceColor %v", cols[9:12], d)
	}
}

func TestLoadColored_3MFHasColor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "colored.3mf")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	m := &meshio.Mesh{
		Vertices:   triVerts,
		Indices:    triIdx,
		FaceColors: []meshio.FaceColor{{Hex: "#FF0000"}, {Hex: "#00FF00"}},
	}
	if err := m.Encode3MF(f); err != nil {
		t.Fatal(err)
	}
	f.Close()

	pos, cols, err := LoadColored(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(pos) != 18 {
		t.Errorf("positions len = %d, want 18", len(pos))
	}
	if len(cols) != 18 {
		t.Fatalf("3MF colors len = %d, want 18 (non-nil)", len(cols))
	}
	// triangle 0 → red
	if cols[0] != 0xFF || cols[1] != 0 || cols[2] != 0 {
		t.Errorf("3MF tri0 = %v, want red", cols[0:3])
	}
}
