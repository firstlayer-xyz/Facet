package manifold

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestImportMeshSTLBinary(t *testing.T) {
	// Create a minimal binary STL with 1 triangle
	dir := t.TempDir()
	path := filepath.Join(dir, "test.stl")

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}

	// 80-byte header
	header := make([]byte, 80)
	f.Write(header)

	// Triangle count: 1
	binary.Write(f, binary.LittleEndian, uint32(1))

	// Normal (0,0,1)
	binary.Write(f, binary.LittleEndian, float32(0))
	binary.Write(f, binary.LittleEndian, float32(0))
	binary.Write(f, binary.LittleEndian, float32(1))

	// Vertex 1: (0, 0, 0)
	binary.Write(f, binary.LittleEndian, float32(0))
	binary.Write(f, binary.LittleEndian, float32(0))
	binary.Write(f, binary.LittleEndian, float32(0))

	// Vertex 2: (10, 0, 0)
	binary.Write(f, binary.LittleEndian, float32(10))
	binary.Write(f, binary.LittleEndian, float32(0))
	binary.Write(f, binary.LittleEndian, float32(0))

	// Vertex 3: (0, 10, 0)
	binary.Write(f, binary.LittleEndian, float32(0))
	binary.Write(f, binary.LittleEndian, float32(10))
	binary.Write(f, binary.LittleEndian, float32(0))

	// Attribute byte count
	binary.Write(f, binary.LittleEndian, uint16(0))

	f.Close()

	solid, err := ImportMesh(path)
	if err != nil {
		t.Fatalf("ImportMesh STL binary: %v", err)
	}
	if solid == nil {
		t.Fatal("expected non-nil Solid")
	}
}

func TestImportMeshSTLASCII(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.stl")

	content := `solid test
facet normal 0 0 1
  outer loop
    vertex 0 0 0
    vertex 10 0 0
    vertex 0 10 0
  endloop
endfacet
endsolid test
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	solid, err := ImportMesh(path)
	if err != nil {
		t.Fatalf("ImportMesh STL ASCII: %v", err)
	}
	if solid == nil {
		t.Fatal("expected non-nil Solid")
	}
}

func TestImportMeshOBJ(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.obj")

	// Tetrahedron with consistent outward-facing winding
	content := `# test tetrahedron
v 0 0 0
v 10 0 0
v 5 10 0
v 5 5 10
f 1 3 2
f 1 2 4
f 2 3 4
f 1 4 3
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	solid, err := ImportMesh(path)
	if err != nil {
		t.Fatalf("ImportMesh OBJ: %v", err)
	}
	if solid == nil {
		t.Fatal("expected non-nil Solid")
	}
	vol := solid.Volume()
	if vol < 1.0 {
		t.Errorf("expected positive volume, got %f", vol)
	}
}

// TestImportMeshSTLRoundTrip creates a cube STL and verifies the imported solid has volume.
func TestImportMeshSTLRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cube.stl")

	// A unit cube with consistently oriented triangles (CCW for outward normals)
	type vec3 struct{ x, y, z float32 }
	type tri struct {
		normal         vec3
		v1, v2, v3     vec3
	}

	triangles := []tri{
		// Front (z=1, normal +z)
		{vec3{0, 0, 1}, vec3{0, 0, 1}, vec3{1, 0, 1}, vec3{1, 1, 1}},
		{vec3{0, 0, 1}, vec3{0, 0, 1}, vec3{1, 1, 1}, vec3{0, 1, 1}},
		// Back (z=0, normal -z)
		{vec3{0, 0, -1}, vec3{1, 0, 0}, vec3{0, 0, 0}, vec3{0, 1, 0}},
		{vec3{0, 0, -1}, vec3{1, 0, 0}, vec3{0, 1, 0}, vec3{1, 1, 0}},
		// Top (y=1, normal +y)
		{vec3{0, 1, 0}, vec3{0, 1, 0}, vec3{0, 1, 1}, vec3{1, 1, 1}},
		{vec3{0, 1, 0}, vec3{0, 1, 0}, vec3{1, 1, 1}, vec3{1, 1, 0}},
		// Bottom (y=0, normal -y)
		{vec3{0, -1, 0}, vec3{0, 0, 0}, vec3{1, 0, 0}, vec3{1, 0, 1}},
		{vec3{0, -1, 0}, vec3{0, 0, 0}, vec3{1, 0, 1}, vec3{0, 0, 1}},
		// Right (x=1, normal +x)
		{vec3{1, 0, 0}, vec3{1, 0, 0}, vec3{1, 1, 0}, vec3{1, 1, 1}},
		{vec3{1, 0, 0}, vec3{1, 0, 0}, vec3{1, 1, 1}, vec3{1, 0, 1}},
		// Left (x=0, normal -x)
		{vec3{-1, 0, 0}, vec3{0, 0, 0}, vec3{0, 0, 1}, vec3{0, 1, 1}},
		{vec3{-1, 0, 0}, vec3{0, 0, 0}, vec3{0, 1, 1}, vec3{0, 1, 0}},
	}

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	header := make([]byte, 80)
	f.Write(header)
	binary.Write(f, binary.LittleEndian, uint32(len(triangles)))
	for _, tri := range triangles {
		for _, v := range []vec3{tri.normal, tri.v1, tri.v2, tri.v3} {
			binary.Write(f, binary.LittleEndian, v.x)
			binary.Write(f, binary.LittleEndian, v.y)
			binary.Write(f, binary.LittleEndian, v.z)
		}
		binary.Write(f, binary.LittleEndian, uint16(0))
	}
	f.Close()

	solid, err := ImportMesh(path)
	if err != nil {
		t.Fatalf("ImportMesh: %v", err)
	}
	vol := solid.Volume()
	if math.Abs(vol-1.0) > 0.01 {
		t.Errorf("expected volume ~1.0, got %f", vol)
	}
}
