//go:build !js

package manifold

import (
	"math"
	"testing"
	"unsafe"
)

func TestDisplayMeshCube(t *testing.T) {
	cube, err := CreateCube(10, 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	dm := cube.ToDisplayMesh()

	if dm.VertexCount == 0 {
		t.Fatal("expected non-zero vertex count")
	}
	if dm.IndexCount == 0 {
		t.Fatal("expected non-zero index count")
	}
	if dm.IndexCount%3 != 0 {
		t.Errorf("index count %d is not a multiple of 3", dm.IndexCount)
	}

	// Verify raw byte lengths
	if len(dm.VertRaw) != dm.VertexCount*12 {
		t.Errorf("vertex bytes: got %d, want %d", len(dm.VertRaw), dm.VertexCount*12)
	}
	if len(dm.IdxRaw) != dm.IndexCount*4 {
		t.Errorf("index bytes: got %d, want %d", len(dm.IdxRaw), dm.IndexCount*4)
	}
}

func TestDisplayMeshMatchesMesh(t *testing.T) {
	// DisplayMesh and Mesh should produce equivalent vertex/index data
	sphere, err := CreateSphere(5, 16)
	if err != nil {
		t.Fatal(err)
	}
	mesh := sphere.ToMesh()
	dm := sphere.ToDisplayMesh()

	if dm.VertexCount != len(mesh.Vertices)/3 {
		t.Errorf("vertex count mismatch: DisplayMesh=%d, Mesh=%d", dm.VertexCount, len(mesh.Vertices)/3)
	}
	if dm.IndexCount != len(mesh.Indices) {
		t.Errorf("index count mismatch: DisplayMesh=%d, Mesh=%d", dm.IndexCount, len(mesh.Indices))
	}

	// Compare raw vertex bytes to Mesh vertices
	dmVerts := unsafe.Slice((*float32)(unsafe.Pointer(&dm.VertRaw[0])), dm.VertexCount*3)
	for i, v := range mesh.Vertices {
		if dmVerts[i] != v {
			t.Errorf("vertex[%d] mismatch: DisplayMesh=%f, Mesh=%f", i, dmVerts[i], v)
			break
		}
	}

	// Compare raw index bytes to Mesh indices
	dmIdx := unsafe.Slice((*uint32)(unsafe.Pointer(&dm.IdxRaw[0])), dm.IndexCount)
	for i, idx := range mesh.Indices {
		if dmIdx[i] != idx {
			t.Errorf("index[%d] mismatch: DisplayMesh=%d, Mesh=%d", i, dmIdx[i], idx)
			break
		}
	}
}

func TestDisplayMeshEmpty(t *testing.T) {
	// Zero-dimension cube should return an error, not degenerate geometry
	_, err := CreateCube(0, 0, 0)
	if err == nil {
		t.Fatal("expected error for zero-dimension cube")
	}
}

func TestDisplayMeshSketch(t *testing.T) {
	// Sketches should also produce valid DisplayMeshes
	sq, err := CreateSquare(10, 10)
	if err != nil {
		t.Fatal(err)
	}
	dm := sq.ToDisplayMesh()

	if dm.VertexCount == 0 {
		t.Fatal("expected non-zero vertex count for sketch")
	}
	if dm.IndexCount == 0 {
		t.Fatal("expected non-zero index count for sketch")
	}
}

func TestDisplayMeshVertexValues(t *testing.T) {
	// Create a known cube and verify vertex positions are reasonable
	cube, err := CreateCube(10, 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	dm := cube.ToDisplayMesh()

	verts := unsafe.Slice((*float32)(unsafe.Pointer(&dm.VertRaw[0])), dm.VertexCount*3)

	// Cube is not centered (center=false): vertices should be in [0, 10]
	for i := 0; i < len(verts); i++ {
		if verts[i] < -0.01 || verts[i] > 10.01 {
			t.Errorf("vertex component [%d]=%f out of expected range [0, 10]", i, verts[i])
			break
		}
	}

	// Bounding box check via decoded vertices
	minX, minY, minZ := float32(math.MaxFloat32), float32(math.MaxFloat32), float32(math.MaxFloat32)
	maxX, maxY, maxZ := float32(-math.MaxFloat32), float32(-math.MaxFloat32), float32(-math.MaxFloat32)
	for i := 0; i < dm.VertexCount; i++ {
		x, y, z := verts[i*3], verts[i*3+1], verts[i*3+2]
		if x < minX { minX = x }
		if y < minY { minY = y }
		if z < minZ { minZ = z }
		if x > maxX { maxX = x }
		if y > maxY { maxY = y }
		if z > maxZ { maxZ = z }
	}
	if math.Abs(float64(maxX-minX)-10) > 0.01 {
		t.Errorf("x extent: got %f, want ~10", maxX-minX)
	}
	if math.Abs(float64(maxY-minY)-10) > 0.01 {
		t.Errorf("y extent: got %f, want ~10", maxY-minY)
	}
	if math.Abs(float64(maxZ-minZ)-10) > 0.01 {
		t.Errorf("z extent: got %f, want ~10", maxZ-minZ)
	}
}

func TestDisplayMeshFaceMap(t *testing.T) {
	cube, err := CreateCube(10, 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(cube.FaceMap) == 0 {
		t.Fatal("expected FaceMap to be populated after creation")
	}

	dm := cube.ToDisplayMesh()
	t.Logf("FaceGroupRaw len: %d", len(dm.FaceGroupRaw))
	t.Logf("FaceGroupCount: %d", dm.FaceGroupCount)
}
