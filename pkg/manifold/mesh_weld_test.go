//go:build !js

package manifold

import (
	"math"
	"testing"
)

// A triangle mesh assembled from independent per-face vertices — every shared
// edge's endpoints duplicated rather than indexed — must still close into a
// solid: the kernel welds coincident vertices before constructing the Manifold.
// This is the shape produced by procedurally-built meshes, e.g. an icosphere
// subdivision that computes each edge midpoint once per adjacent triangle.
func TestCreateSolidFromMeshWeldsCoincidentVertices(t *testing.T) {
	// A tetrahedron (verts + outward-CCW winding from import_test's OBJ fixture),
	// then expanded so each of the 4 triangles owns 3 private, unshared vertices.
	tetra := [4][3]float32{
		{0, 0, 0},  // v0
		{10, 0, 0}, // v1
		{5, 10, 0}, // v2
		{5, 5, 10}, // v3
	}
	tris := [4][3]int{
		{0, 2, 1},
		{0, 1, 3},
		{1, 2, 3},
		{0, 3, 2},
	}
	var verts []float32
	var indices []uint32
	for _, tri := range tris {
		for _, vi := range tri {
			verts = append(verts, tetra[vi][0], tetra[vi][1], tetra[vi][2])
			indices = append(indices, uint32(len(indices)))
		}
	}

	solid, err := CreateSolidFromMesh(verts, indices)
	if err != nil {
		t.Fatalf("CreateSolidFromMesh: %v", err)
	}
	const want = 500.0 / 3.0 // (1/3)·base-area(50)·height(10)
	if got := solid.Volume(); math.Abs(got-want) > 1e-3 {
		t.Fatalf("welded tetrahedron volume = %v, want %v (a volume of 0 means the coincident vertices were not welded)", got, want)
	}
}

// The same fix must apply on the faceID path used by PolyMesh.ToSolid —
// procedural polyhedra (icosphere subdivision, etc.) reach the kernel
// through facet_solid_from_mesh_with_face_ids, not the no-IDs entry point.
// Both routes share wrap_solid_from_mesh, so welding must work here too.
func TestPolyMeshToSolidWeldsCoincidentVertices(t *testing.T) {
	// Same tetrahedron, but expressed as a PolyMesh where each of the 4 faces
	// owns 3 private vertices. ToSolid fan-triangulates and emits a faceID
	// per output triangle, routing through facet_solid_from_mesh_with_face_ids.
	tetra := [4][3]float64{
		{0, 0, 0},
		{10, 0, 0},
		{5, 10, 0},
		{5, 5, 10},
	}
	tris := [4][3]int{
		{0, 2, 1},
		{0, 1, 3},
		{1, 2, 3},
		{0, 3, 2},
	}
	var verts []float64
	var faces [][]int
	for _, tri := range tris {
		base := len(verts) / 3
		for _, vi := range tri {
			verts = append(verts, tetra[vi][0], tetra[vi][1], tetra[vi][2])
		}
		faces = append(faces, []int{base, base + 1, base + 2})
	}
	pm := &PolyMesh{Vertices: verts, Faces: faces}

	solid, err := pm.ToSolid()
	if err != nil {
		t.Fatalf("PolyMesh.ToSolid: %v", err)
	}
	const want = 500.0 / 3.0
	if got := solid.Volume(); math.Abs(got-want) > 1e-3 {
		t.Fatalf("welded PolyMesh tetrahedron volume = %v, want %v (faceID path failed to weld coincident vertices)", got, want)
	}
}
