package manifold

/*
#include "facet_cxx.h"
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"os"
	"runtime"
	"unsafe"
)

// ImportMesh reads a mesh file and returns a SolidFuture. The format is auto-detected
// from the file extension by Assimp (STL, OBJ, and 100+ other formats).
// File existence is checked synchronously; the CGo import runs asynchronously.
func ImportMesh(path string) (*SolidFuture, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("ImportMesh: %w", err)
	}

	return startSolidFuture(func() (*Solid, error) {
		cPath := C.CString(path)
		defer C.free(unsafe.Pointer(cPath))

		ptr := C.facet_import_mesh(cPath)
		if ptr == nil {
			return nil, fmt.Errorf("ImportMesh: no vertices found in %s", path)
		}
		s := newSolid(ptr)
		origID := uint32(C.facet_original_id(s.ptr))
		runtime.KeepAlive(s)
		s.FaceMap = map[uint32]FaceInfo{origID: {Color: NoColor}}
		return s, nil
	}), nil
}

// CreateSolidFromMesh creates a Manifold Solid from raw vertex and index data.
// vertices is a flat array of xyz floats, indices is a flat array of triangle vertex indices.
func CreateSolidFromMesh(vertices []float32, indices []uint32) (*SolidFuture, error) {
	// Copy slices so the caller can't mutate them after returning
	verts := make([]float32, len(vertices))
	copy(verts, vertices)
	idxs := make([]uint32, len(indices))
	copy(idxs, indices)

	if len(verts) == 0 || len(idxs) == 0 {
		return nil, fmt.Errorf("CreateSolidFromMesh: empty vertex or index data")
	}

	return startSolidFuture(func() (*Solid, error) {
		nVerts := len(verts) / 3
		nTris := len(idxs) / 3

		ptr := C.facet_solid_from_mesh(
			(*C.float)(unsafe.Pointer(&verts[0])), C.size_t(nVerts),
			(*C.uint32_t)(unsafe.Pointer(&idxs[0])), C.size_t(nTris))
		if ptr == nil {
			return nil, fmt.Errorf("CreateSolidFromMesh: manifold creation failed")
		}
		s := newSolid(ptr)
		origID := uint32(C.facet_original_id(s.ptr))
		runtime.KeepAlive(s)
		s.FaceMap = map[uint32]FaceInfo{origID: {Color: NoColor}}
		return s, nil
	}), nil
}
