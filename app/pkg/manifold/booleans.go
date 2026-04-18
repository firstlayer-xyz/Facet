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

// Union computes the boolean union of two solids.
func (a *Solid) Union(b *Solid) *Solid {
	ptr := C.facet_union(a.ptr, b.ptr)
	runtime.KeepAlive(a)
	runtime.KeepAlive(b)
	s := newSolid(ptr)
	s.FaceMap = mergeFaceMaps(a.FaceMap, b.FaceMap)
	return s
}

// Difference computes the boolean difference of two solids.
func (a *Solid) Difference(b *Solid) *Solid {
	ptr := C.facet_difference(a.ptr, b.ptr)
	runtime.KeepAlive(a)
	runtime.KeepAlive(b)
	s := newSolid(ptr)
	s.FaceMap = mergeFaceMaps(a.FaceMap, b.FaceMap)
	return s
}

// Intersection computes the boolean intersection of two solids.
func (a *Solid) Intersection(b *Solid) *Solid {
	ptr := C.facet_intersection(a.ptr, b.ptr)
	runtime.KeepAlive(a)
	runtime.KeepAlive(b)
	s := newSolid(ptr)
	s.FaceMap = mergeFaceMaps(a.FaceMap, b.FaceMap)
	return s
}

// Insert cuts a hole in a for b, removes floating inner plugs, and seats b.
func (a *Solid) Insert(b *Solid) *Solid {
	ptr := C.facet_insert(a.ptr, b.ptr)
	runtime.KeepAlive(a)
	runtime.KeepAlive(b)
	s := newSolid(ptr)
	s.FaceMap = mergeFaceMaps(a.FaceMap, b.FaceMap)
	return s
}

// DecomposeSolid splits a solid into its disconnected connected components.
func DecomposeSolid(s *Solid) []*Solid {
	var outArr **C.ManifoldPtr
	n := int(C.facet_decompose(s.ptr, &outArr))
	runtime.KeepAlive(s)
	if n == 0 {
		return nil
	}
	cSlice := unsafe.Slice(outArr, n)
	result := make([]*Solid, n)
	for i, ptr := range cSlice {
		result[i] = newSolid(ptr)
		result[i].FaceMap = s.withFaceMap()
	}
	C.free(unsafe.Pointer(outArr))
	return result
}

// ---------------------------------------------------------------------------
// 2D Boolean Operations
// ---------------------------------------------------------------------------

// Union computes the boolean union of two sketches.
func (a *Sketch) Union(b *Sketch) *Sketch {
	ptr := C.facet_cs_union(a.ptr, b.ptr)
	runtime.KeepAlive(a)
	runtime.KeepAlive(b)
	return newSketch(ptr)
}

// Difference computes the boolean difference of two sketches.
func (a *Sketch) Difference(b *Sketch) *Sketch {
	ptr := C.facet_cs_difference(a.ptr, b.ptr)
	runtime.KeepAlive(a)
	runtime.KeepAlive(b)
	return newSketch(ptr)
}

// Intersection computes the boolean intersection of two sketches.
func (a *Sketch) Intersection(b *Sketch) *Sketch {
	ptr := C.facet_cs_intersection(a.ptr, b.ptr)
	runtime.KeepAlive(a)
	runtime.KeepAlive(b)
	return newSketch(ptr)
}
