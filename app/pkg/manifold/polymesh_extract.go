package manifold

/*
#include "facet_cxx.h"
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"runtime"
	"unsafe"
)

// ExtractPolyMesh extracts a PolyMesh from a Solid using faceID grouping.
// Calls the C++ facet_extract_polymesh which handles merge-pair dedup and
// boundary-loop chaining.
func ExtractPolyMesh(s *Solid) *PolyMesh {
	var outVerts *C.double
	var nVerts C.int
	var outFaceIdx *C.int
	var faceIdxLen C.int
	var outFaceSizes *C.int
	var nFaces C.int

	C.facet_extract_polymesh(s.ptr,
		&outVerts, &nVerts,
		&outFaceIdx, &faceIdxLen,
		&outFaceSizes, &nFaces)
	runtime.KeepAlive(s)

	if nVerts == 0 || nFaces == 0 {
		return &PolyMesh{}
	}

	defer C.free(unsafe.Pointer(outVerts))
	defer C.free(unsafe.Pointer(outFaceIdx))
	defer C.free(unsafe.Pointer(outFaceSizes))

	// Copy C arrays → Go slices
	numV := int(nVerts)
	vertices := make([]float64, numV*3)
	cVerts := unsafe.Slice((*float64)(unsafe.Pointer(outVerts)), numV*3)
	copy(vertices, cVerts)

	numF := int(nFaces)
	cSizes := unsafe.Slice((*int32)(unsafe.Pointer(outFaceSizes)), numF)
	totalIdx := int(faceIdxLen)
	cIdx := unsafe.Slice((*int32)(unsafe.Pointer(outFaceIdx)), totalIdx)

	faces := make([][]int, numF)
	offset := 0
	for fi := 0; fi < numF; fi++ {
		sz := int(cSizes[fi])
		if offset+sz > totalIdx {
			break
		}
		face := make([]int, sz)
		for j := 0; j < sz; j++ {
			face[j] = int(cIdx[offset+j])
		}
		faces[fi] = face
		offset += sz
	}

	return &PolyMesh{Vertices: vertices, Faces: faces}
}

// CreateSolidFromMeshWithFaceIDs creates a Manifold from mesh data with per-triangle faceIDs.
// The faceIDs survive through boolean operations, enabling polygon reconstruction.
func CreateSolidFromMeshWithFaceIDs(verts []float32, indices, faceIDs []uint32) (*SolidFuture, error) {
	if len(verts) == 0 || len(indices) == 0 || len(faceIDs) == 0 {
		return nil, fmt.Errorf("CreateSolidFromMeshWithFaceIDs: empty vertex, index, or faceID data")
	}
	return startSolidFuture(func() (*Solid, error) {
		ptr := C.facet_solid_from_mesh_with_face_ids(
			(*C.float)(unsafe.Pointer(&verts[0])), C.size_t(len(verts)/3),
			(*C.uint32_t)(unsafe.Pointer(&indices[0])), C.size_t(len(indices)/3),
			(*C.uint32_t)(unsafe.Pointer(&faceIDs[0])), C.size_t(len(faceIDs)))
		if ptr == nil {
			return nil, fmt.Errorf("CreateSolidFromMeshWithFaceIDs: manifold creation failed")
		}
		s := newSolid(ptr)
		origID := uint32(C.facet_original_id(s.ptr))
		runtime.KeepAlive(s)
		s.FaceMap = map[uint32]FaceInfo{origID: {Color: NoColor}}
		return s, nil
	}), nil
}
