package manifold

import (
	"encoding/base64"
	"encoding/json"
	"math"
	"testing"
	"unsafe"
)

func TestDisplayMeshCube(t *testing.T) {
	cube := CreateCube(10, 10, 10)
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
	sphereFuture := CreateSphere(5, 16)
	sphere, err := sphereFuture.Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	mesh := sphere.ToMesh()
	dm := sphereFuture.ToDisplayMesh()

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

func TestDisplayMeshMarshalJSON(t *testing.T) {
	cube := CreateCube(5, 5, 5)
	dm := cube.ToDisplayMesh()

	data, err := json.Marshal(dm)
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}

	var parsed struct {
		Vertices    string `json:"vertices"`
		Indices     string `json:"indices"`
		VertexCount int    `json:"vertexCount"`
		IndexCount  int    `json:"indexCount"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if parsed.VertexCount != dm.VertexCount {
		t.Errorf("vertexCount: got %d, want %d", parsed.VertexCount, dm.VertexCount)
	}
	if parsed.IndexCount != dm.IndexCount {
		t.Errorf("indexCount: got %d, want %d", parsed.IndexCount, dm.IndexCount)
	}
	if parsed.Vertices != base64.StdEncoding.EncodeToString(dm.VertRaw) {
		t.Error("vertices base64 mismatch")
	}
	if parsed.Indices != base64.StdEncoding.EncodeToString(dm.IdxRaw) {
		t.Error("indices base64 mismatch")
	}

	// Verify no normals field in output
	var raw map[string]json.RawMessage
	json.Unmarshal(data, &raw)
	if _, ok := raw["normals"]; ok {
		t.Error("JSON should not contain normals field")
	}
}

func TestDisplayMeshEmpty(t *testing.T) {
	// Empty solid should produce empty DisplayMesh
	cube := CreateCube(0, 0, 0)
	dm := cube.ToDisplayMesh()

	if dm.VertexCount != 0 {
		t.Errorf("expected 0 vertices, got %d", dm.VertexCount)
	}
	if dm.IndexCount != 0 {
		t.Errorf("expected 0 indices, got %d", dm.IndexCount)
	}
	if len(dm.VertRaw) != 0 {
		t.Error("expected empty vertex bytes")
	}
	if len(dm.IdxRaw) != 0 {
		t.Error("expected empty index bytes")
	}
}

func Test_mergeDisplayMeshes(t *testing.T) {
	a := CreateCube(5, 5, 5)
	b := CreateCube(3, 3, 3).Translate(10, 0, 0)
	dmA := a.ToDisplayMesh()
	dmB := b.ToDisplayMesh()

	merged := mergeDisplayMeshes([]*DisplayMesh{dmA, dmB})

	if merged.VertexCount != dmA.VertexCount+dmB.VertexCount {
		t.Errorf("merged vertex count: got %d, want %d", merged.VertexCount, dmA.VertexCount+dmB.VertexCount)
	}
	if merged.IndexCount != dmA.IndexCount+dmB.IndexCount {
		t.Errorf("merged index count: got %d, want %d", merged.IndexCount, dmA.IndexCount+dmB.IndexCount)
	}

	// Verify merged indices are properly offset
	indices := unsafe.Slice((*uint32)(unsafe.Pointer(&merged.IdxRaw[0])), merged.IndexCount)

	// All indices should be in range [0, merged.VertexCount)
	for i, idx := range indices {
		if idx >= uint32(merged.VertexCount) {
			t.Errorf("index[%d]=%d out of range (vertexCount=%d)", i, idx, merged.VertexCount)
			break
		}
	}

	// Second mesh's indices should be offset by first mesh's vertex count
	aIndices := unsafe.Slice((*uint32)(unsafe.Pointer(&dmA.IdxRaw[0])), dmA.IndexCount)
	bIndices := unsafe.Slice((*uint32)(unsafe.Pointer(&dmB.IdxRaw[0])), dmB.IndexCount)

	// First mesh indices should be unchanged
	for i := 0; i < dmA.IndexCount; i++ {
		if indices[i] != aIndices[i] {
			t.Errorf("first mesh index[%d]: got %d, want %d", i, indices[i], aIndices[i])
			break
		}
	}

	// Second mesh indices should be offset
	offset := uint32(dmA.VertexCount)
	for i := 0; i < dmB.IndexCount; i++ {
		want := bIndices[i] + offset
		got := indices[dmA.IndexCount+i]
		if got != want {
			t.Errorf("second mesh index[%d]: got %d, want %d (offset=%d)", i, got, want, offset)
			break
		}
	}
}

func Test_mergeDisplayMeshesSingle(t *testing.T) {
	cube := CreateCube(5, 5, 5)
	dm := cube.ToDisplayMesh()

	merged := mergeDisplayMeshes([]*DisplayMesh{dm})
	if merged != dm {
		t.Error("single mesh merge should return the same pointer")
	}
}

func TestDisplayMeshSketch(t *testing.T) {
	// Sketches should also produce valid DisplayMeshes
	sq := CreateSquare(10, 10)
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
	cube := CreateCube(10, 10, 10)
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
	cube := CreateCube(10, 10, 10)
	s, err := cube.Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(s.FaceMap) == 0 {
		t.Fatal("expected FaceMap to be populated after creation")
	}

	dm := s.ToDisplayMesh()
	t.Logf("FaceGroupRaw len: %d", len(dm.FaceGroupRaw))
	t.Logf("FaceGroupCount: %d", dm.FaceGroupCount)
}
