package manifold

/*
#cgo CFLAGS: -I${SRCDIR}/cxx/include
#include "facet_cxx.h"
*/
import "C"

import "runtime"

// warpSolid applies a per-vertex warp function to a solid.
func warpSolid(s *Solid, fn func(x, y, z float64) (float64, float64, float64)) *Solid {
	id := registerWarp(fn)
	defer unregisterWarp(id)
	ptr := C.facet_warp(s.ptr, C.int(id))
	runtime.KeepAlive(s)
	r := newSolid(ptr)
	r.FaceMap = s.withFaceMap()
	return r
}

// levelSetSolid creates a solid from an SDF callback over the given bounding box.
func levelSetSolid(fn func(x, y, z float64) float64, minX, minY, minZ, maxX, maxY, maxZ, edgeLen float64) *Solid {
	id := registerLevelSet(fn)
	defer unregisterLevelSet(id)
	ptr := C.facet_level_set(C.int(id),
		C.double(minX), C.double(minY), C.double(minZ),
		C.double(maxX), C.double(maxY), C.double(maxZ),
		C.double(edgeLen))
	s := newSolid(ptr)
	origID := uint32(C.facet_original_id(s.ptr))
	runtime.KeepAlive(s)
	s.FaceMap = map[uint32]FaceInfo{origID: {Color: NoColor}}
	return s
}
