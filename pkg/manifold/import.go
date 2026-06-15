//go:build !js

package manifold

/*
#include "facet_cxx.h"
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"unsafe"

	"github.com/firstlayer-xyz/meshio"
)

// ImportMesh reads a mesh file and returns a Solid. The format is auto-detected
// from the file extension (STL, OBJ, or 3MF). Decoding is pure Go (via meshio) —
// the resulting triangle mesh is handed to the kernel through CreateSolidFromMesh,
// which orients the winding and validates manifoldness.
func ImportMesh(path string) (*Solid, error) {
	mesh, err := meshio.Read(path)
	if err != nil {
		return nil, fmt.Errorf("ImportMesh %s: %w", path, err)
	}
	s, err := CreateSolidFromMesh(mesh.Vertices, mesh.Indices)
	if err != nil {
		return nil, fmt.Errorf("ImportMesh %s: %w", path, err)
	}
	return s, nil
}

// CreateSolidFromMesh creates a Manifold Solid from raw vertex and index data.
// vertices is a flat array of xyz floats, indices is a flat array of triangle vertex indices.
func CreateSolidFromMesh(vertices []float32, indices []uint32) (*Solid, error) {
	if len(vertices) == 0 || len(indices) == 0 {
		return nil, fmt.Errorf("CreateSolidFromMesh: empty vertex or index data")
	}

	indices = orientOutward(vertices, indices)
	nVerts := len(vertices) / 3
	nTris := len(indices) / 3

	var ret C.FacetSolidRet
	C.facet_solid_from_mesh(
		(*C.float)(unsafe.Pointer(&vertices[0])), C.size_t(nVerts),
		(*C.uint32_t)(unsafe.Pointer(&indices[0])), C.size_t(nTris),
		&ret)
	if ret.ptr == nil {
		return nil, fmt.Errorf("CreateSolidFromMesh: manifold creation failed")
	}
	s := newSolidWithOrigin(ret)
	if s.NumComponents() == 0 {
		// The kernel accepted the data but produced an empty manifold: the input
		// is not a valid closed 2-manifold (open, self-intersecting, or
		// non-orientable). Error rather than hand back vanished geometry.
		return nil, fmt.Errorf("CreateSolidFromMesh: mesh is not a valid closed manifold (open, self-intersecting, or non-orientable)")
	}
	return s, nil
}
