package manifold

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestParseHexRGB(t *testing.T) {
	if c, ok := ParseHexRGB("#FF8000"); !ok || c != [3]byte{0xFF, 0x80, 0x00} {
		t.Errorf("ParseHexRGB(#FF8000) = %v,%v", c, ok)
	}
	if c, ok := ParseHexRGB("#11223344"); !ok || c != [3]byte{0x11, 0x22, 0x33} {
		t.Errorf("ParseHexRGB(#RRGGBBAA) should drop alpha, got %v,%v", c, ok)
	}
	if _, ok := ParseHexRGB("nope"); ok {
		t.Errorf("ParseHexRGB(nope) should fail")
	}
}

// le encodes float32s little-endian (matches DisplayMesh raw buffers).
func le(vals ...float32) []byte {
	b := make([]byte, len(vals)*4)
	for i, v := range vals {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(v))
	}
	return b
}

func leU32(vals ...uint32) []byte {
	b := make([]byte, len(vals)*4)
	for i, v := range vals {
		binary.LittleEndian.PutUint32(b[i*4:], v)
	}
	return b
}

func TestExpandedColors_NilWhenNoFaceColors(t *testing.T) {
	dm := &DisplayMesh{ExpandedCount: 3, ExpandedRaw: le(0, 0, 0, 1, 0, 0, 0, 1, 0)}
	if got := dm.ExpandedColors(); got != nil {
		t.Errorf("ExpandedColors with no FaceColorMap = %v, want nil", got)
	}
}

func TestExpandedColors_PerTriangle(t *testing.T) {
	// One triangle (3 expanded verts), face id 7 → red.
	dm := &DisplayMesh{
		ExpandedCount: 3,
		FaceGroupRaw:  leU32(7),
		FaceColorMap:  map[uint32]string{7: "#FF0000"},
	}
	got := dm.ExpandedColors()
	if len(got) != 9 {
		t.Fatalf("len = %d, want 9", len(got))
	}
	for v := 0; v < 3; v++ {
		if got[v*3] != 0xFF || got[v*3+1] != 0 || got[v*3+2] != 0 {
			t.Errorf("vertex %d = %v, want red", v, got[v*3:v*3+3])
		}
	}
}

func TestExpandedColors_FallbackForUnassignedFace(t *testing.T) {
	// Two triangles (6 expanded verts). Tri 0's face id 7 is red; tri 1's
	// face id 9 has no entry in FaceColorMap → DefaultFaceColor.
	dm := &DisplayMesh{
		ExpandedCount: 6,
		FaceGroupRaw:  leU32(7, 9), // one id per triangle
		FaceColorMap:  map[uint32]string{7: "#FF0000"},
	}
	got := dm.ExpandedColors()
	if len(got) != 18 {
		t.Fatalf("len = %d, want 18", len(got))
	}
	// tri 0 verts (0..2) red
	if got[0] != 0xFF || got[1] != 0 || got[2] != 0 {
		t.Errorf("tri0 vert0 = %v, want red", got[0:3])
	}
	// tri 1 verts (3..5) default
	d := DefaultFaceColor
	if got[9] != d[0] || got[10] != d[1] || got[11] != d[2] {
		t.Errorf("tri1 vert0 = %v, want DefaultFaceColor %v", got[9:12], d)
	}
}

func TestExpandedColors_PerVertexFaceGroups(t *testing.T) {
	// FaceGroupRaw has one id per expanded vertex (len/4 == ExpandedCount).
	dm := &DisplayMesh{
		ExpandedCount: 3,
		FaceGroupRaw:  leU32(5, 5, 5),
		FaceColorMap:  map[uint32]string{5: "#00FF00"},
	}
	got := dm.ExpandedColors()
	if len(got) != 9 {
		t.Fatalf("len = %d, want 9", len(got))
	}
	for v := 0; v < 3; v++ {
		if got[v*3] != 0 || got[v*3+1] != 0xFF || got[v*3+2] != 0 {
			t.Errorf("vert %d = %v, want green", v, got[v*3:v*3+3])
		}
	}
}

func TestExpandedPositions(t *testing.T) {
	dm := &DisplayMesh{ExpandedRaw: le(1, 2, 3, 4, 5, 6)}
	got := dm.ExpandedPositions()
	want := []float32{1, 2, 3, 4, 5, 6}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}
