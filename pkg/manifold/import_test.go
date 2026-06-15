//go:build !js

package manifold

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeBinarySTL writes a binary STL of the given triangles (each 3 vertices
// of 3 coords) to path.
func writeBinarySTL(t *testing.T, path string, tris [][9]float32) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	f.Write(make([]byte, 80)) // header
	binary.Write(f, binary.LittleEndian, uint32(len(tris)))
	for _, tri := range tris {
		// Normal (ignored by the importer's manifold build)
		for i := 0; i < 3; i++ {
			binary.Write(f, binary.LittleEndian, float32(0))
		}
		for _, c := range tri {
			binary.Write(f, binary.LittleEndian, c)
		}
		binary.Write(f, binary.LittleEndian, uint16(0))
	}
}

func TestImportMeshSTLBinary(t *testing.T) {
	// A closed tetrahedron — ImportMesh requires a closed 2-manifold.
	dir := t.TempDir()
	path := filepath.Join(dir, "test.stl")
	writeBinarySTL(t, path, [][9]float32{
		{0, 0, 0, 5, 10, 0, 10, 0, 0},
		{0, 0, 0, 10, 0, 0, 5, 5, 10},
		{10, 0, 0, 5, 10, 0, 5, 5, 10},
		{0, 0, 0, 5, 5, 10, 5, 10, 0},
	})

	solid, err := ImportMesh(path)
	if err != nil {
		t.Fatalf("ImportMesh STL binary: %v", err)
	}
	if solid == nil {
		t.Fatal("expected non-nil Solid")
	}
	if v := solid.Volume(); v <= 0 {
		t.Fatalf("expected positive volume, got %v", v)
	}
}

// An OPEN shell (a single triangle) is not a closed manifold: ImportMesh must
// error clearly instead of returning a silently-empty solid.
func TestImportMeshOpenShellErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "open.stl")
	writeBinarySTL(t, path, [][9]float32{
		{0, 0, 0, 10, 0, 0, 0, 10, 0},
	})
	_, err := ImportMesh(path)
	if err == nil || !strings.Contains(err.Error(), "closed manifold") {
		t.Fatalf("expected a closed-manifold error, got: %v", err)
	}
}

func TestImportMeshSTLASCII(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.stl")

	// A closed tetrahedron — ImportMesh requires a closed 2-manifold.
	content := `solid test
facet normal 0 0 0
  outer loop
    vertex 0 0 0
    vertex 5 10 0
    vertex 10 0 0
  endloop
endfacet
facet normal 0 0 0
  outer loop
    vertex 0 0 0
    vertex 10 0 0
    vertex 5 5 10
  endloop
endfacet
facet normal 0 0 0
  outer loop
    vertex 10 0 0
    vertex 5 10 0
    vertex 5 5 10
  endloop
endfacet
facet normal 0 0 0
  outer loop
    vertex 0 0 0
    vertex 5 5 10
    vertex 5 10 0
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

// OBJ is imported via meshio's pure-Go decoder (format auto-detected from the
// .obj extension); CreateSolidFromMesh orients the winding to a valid solid.
func TestImportMeshOBJ(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.obj")

	// Tetrahedron with consistent outward-facing winding.
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
	if v := solid.Volume(); v < 1.0 {
		t.Errorf("expected positive volume, got %f", v)
	}
}

// 3MF round-trips through the manifold-level export/import: a cube written by
// Export3MF reads back via ImportMesh with its volume intact.
func TestImportMesh3MF(t *testing.T) {
	cube, err := CreateCube(1, 1, 1)
	if err != nil {
		t.Fatalf("CreateCube: %v", err)
	}
	path := filepath.Join(t.TempDir(), "cube.3mf")
	if err := Export3MF(cube, path); err != nil {
		t.Fatalf("Export3MF: %v", err)
	}
	solid, err := ImportMesh(path)
	if err != nil {
		t.Fatalf("ImportMesh 3MF: %v", err)
	}
	if v := solid.Volume(); math.Abs(v-1.0) > 0.01 {
		t.Errorf("expected volume ~1.0, got %f", v)
	}
}

// A format meshio does not decode (e.g. .ply) is rejected with an error naming
// the unsupported extension, rather than silently producing nothing.
func TestImportMeshUnsupportedExtension(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model.ply")
	if err := os.WriteFile(path, []byte("ply\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := ImportMesh(path)
	if err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("expected an unsupported-extension error for .ply, got: %v", err)
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

// TestImportMeshMissingFile verifies that a missing file surfaces as an
// error that names the offending path. The error comes from the meshio reader's
// os.Open failure — the Go side does not pre-stat.
func TestImportMeshMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.stl")
	_, err := ImportMesh(path)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "does-not-exist.stl") {
		t.Errorf("error should name the missing file: %v", err)
	}
}

// TestImportMeshCorruptFile verifies that a file whose contents cannot be
// parsed yields a non-nil error rather than silently producing an empty
// Solid.
func TestImportMeshCorruptFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "garbage.stl")
	if err := os.WriteFile(path, []byte("this is not an STL file"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ImportMesh(path)
	if err == nil {
		t.Fatal("expected error for corrupt STL")
	}
}
