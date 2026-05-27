//go:build !js

package manifold

/*
#cgo CFLAGS: -I${SRCDIR}/cxx/include
#include "facet_cxx.h"
*/
import "C"

import "runtime"

// Warp deforms each vertex of a solid using the given function.
// The function receives the current position and returns the new position.
func (s *Solid) Warp(fn func(x, y, z float64) (float64, float64, float64)) *Solid {
	id := registerWarp(fn)
	defer unregisterWarp(id)
	var ret C.FacetSolidRet
	C.facet_warp(s.ptr, C.int(id), &ret)
	runtime.KeepAlive(s)
	r := newSolid(ret)
	r.FaceMap = s.withFaceMap()
	return r
}

// LevelSet creates a solid from a signed-distance-field (SDF) function.
// The SDF returns negative values inside the solid and positive outside.
// bounds defines the region to sample; edgeLen controls mesh resolution.
func LevelSet(fn func(x, y, z float64) float64, minX, minY, minZ, maxX, maxY, maxZ, edgeLen float64) *Solid {
	id := registerLevelSet(fn)
	defer unregisterLevelSet(id)
	var ret C.FacetSolidRet
	C.facet_level_set(C.int(id),
		C.double(minX), C.double(minY), C.double(minZ),
		C.double(maxX), C.double(maxY), C.double(maxZ),
		C.double(edgeLen), &ret)
	return newSolidWithOrigin(ret)
}
