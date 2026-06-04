//go:build !js

package manifold

/*
#include "facet_cxx.h"
#include <stdlib.h>
*/
import "C"
import (
	"runtime"
	"unsafe"
)

// ---------------------------------------------------------------------------
// 3D Boolean Operations
// ---------------------------------------------------------------------------

// Union computes the boolean union of two solids. Panics if either operand
// is nil — a nil Solid here is an internal bug, not a recoverable input.
func (a *Solid) Union(b *Solid) *Solid {
	requireSolids("Union", a, b)
	var ret C.FacetSolidRet
	C.facet_union(a.ptr, b.ptr, &ret)
	runtime.KeepAlive(a)
	runtime.KeepAlive(b)
	s := newSolid(ret)
	s.FaceMap = mergeFaceMaps(a.FaceMap, b.FaceMap)
	return s
}

// Difference computes the boolean difference of two solids. Panics if either
// operand is nil.
func (a *Solid) Difference(b *Solid) *Solid {
	requireSolids("Difference", a, b)
	var ret C.FacetSolidRet
	C.facet_difference(a.ptr, b.ptr, &ret)
	runtime.KeepAlive(a)
	runtime.KeepAlive(b)
	s := newSolid(ret)
	s.FaceMap = mergeFaceMaps(a.FaceMap, b.FaceMap)
	return s
}

// Intersection computes the boolean intersection of two solids. Panics if
// either operand is nil.
func (a *Solid) Intersection(b *Solid) *Solid {
	requireSolids("Intersection", a, b)
	var ret C.FacetSolidRet
	C.facet_intersection(a.ptr, b.ptr, &ret)
	runtime.KeepAlive(a)
	runtime.KeepAlive(b)
	s := newSolid(ret)
	s.FaceMap = mergeFaceMaps(a.FaceMap, b.FaceMap)
	return s
}

// Insert cuts a hole in a for b, removes floating inner plugs, and seats b.
// Panics if either operand is nil.
func (a *Solid) Insert(b *Solid) *Solid {
	requireSolids("Insert", a, b)
	var ret C.FacetSolidRet
	C.facet_insert(a.ptr, b.ptr, &ret)
	runtime.KeepAlive(a)
	runtime.KeepAlive(b)
	s := newSolid(ret)
	s.FaceMap = mergeFaceMaps(a.FaceMap, b.FaceMap)
	return s
}

// DecomposeSolid splits a solid into its disconnected connected components.
// Entries that the kernel returns with a nil pointer are skipped — the caller
// receives only well-formed components.
func DecomposeSolid(s *Solid) []*Solid {
	var outArr *C.FacetSolidRet
	n := int(C.facet_decompose(s.ptr, &outArr))
	runtime.KeepAlive(s)
	if n == 0 {
		return nil
	}
	cSlice := unsafe.Slice(outArr, n)
	result := make([]*Solid, 0, n)
	for i := range cSlice {
		part := newSolid(cSlice[i])
		if part == nil {
			continue
		}
		part.FaceMap = s.withFaceMap()
		result = append(result, part)
	}
	C.free(unsafe.Pointer(outArr))
	return result
}

// ---------------------------------------------------------------------------
// 2D Boolean Operations
// ---------------------------------------------------------------------------

// Union computes the boolean union of two sketches.
func (a *Sketch) Union(b *Sketch) *Sketch {
	var ret C.FacetSketchRet
	C.facet_cs_union(a.ptr, b.ptr, &ret)
	runtime.KeepAlive(a)
	runtime.KeepAlive(b)
	return newSketch(ret)
}

// Difference computes the boolean difference of two sketches.
func (a *Sketch) Difference(b *Sketch) *Sketch {
	var ret C.FacetSketchRet
	C.facet_cs_difference(a.ptr, b.ptr, &ret)
	runtime.KeepAlive(a)
	runtime.KeepAlive(b)
	return newSketch(ret)
}

// Intersection computes the boolean intersection of two sketches.
func (a *Sketch) Intersection(b *Sketch) *Sketch {
	var ret C.FacetSketchRet
	C.facet_cs_intersection(a.ptr, b.ptr, &ret)
	runtime.KeepAlive(a)
	runtime.KeepAlive(b)
	return newSketch(ret)
}
