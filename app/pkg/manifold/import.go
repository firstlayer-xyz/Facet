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

// ImportMesh reads a mesh file and returns a Solid. The format is auto-detected
// from the file extension by Assimp (STL, OBJ, and 100+ other formats).
func ImportMesh(path string) (*Solid, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("ImportMesh: %w", err)
	}

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
}

// CreateSolidFromMesh creates a Manifold Solid from raw vertex and index data.
// vertices is a flat array of xyz floats, indices is a flat array of triangle vertex indices.
func CreateSolidFromMesh(vertices []float32, indices []uint32) (*Solid, error) {
	if len(vertices) == 0 || len(indices) == 0 {
		return nil, fmt.Errorf("CreateSolidFromMesh: empty vertex or index data")
	}

	nVerts := len(vertices) / 3
	nTris := len(indices) / 3

	ptr := C.facet_solid_from_mesh(
		(*C.float)(unsafe.Pointer(&vertices[0])), C.size_t(nVerts),
		(*C.uint32_t)(unsafe.Pointer(&indices[0])), C.size_t(nTris))
	if ptr == nil {
		return nil, fmt.Errorf("CreateSolidFromMesh: manifold creation failed")
	}
	s := newSolid(ptr)
	origID := uint32(C.facet_original_id(s.ptr))
	runtime.KeepAlive(s)
	s.FaceMap = map[uint32]FaceInfo{origID: {Color: NoColor}}
	return s, nil
}
