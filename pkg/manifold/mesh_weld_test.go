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
