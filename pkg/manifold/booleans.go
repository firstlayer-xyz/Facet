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

// binaryBool is the shared body of the pairwise Solid booleans: validate,
// run the kernel op, merge the face maps (a's entries win). Panics if either
// operand is nil — a nil Solid here is an internal bug, not a recoverable
// input. Returns nil when the kernel fails (the exception barrier nulled the
// result) — nil, not a panic.
func (a *Solid) binaryBool(name string, b *Solid, call func(x, y *C.ManifoldPtr, ret *C.FacetSolidRet)) *Solid {
	requireSolids(name, a, b)
	var ret C.FacetSolidRet
	call(a.ptr, b.ptr, &ret)
	runtime.KeepAlive(a)
	runtime.KeepAlive(b)
	s := newSolid(ret)
	if s == nil {
		return nil
	}
	s.FaceMap = mergeFaceMaps(a.FaceMap, b.FaceMap)
	return s
}

// Union computes the boolean union of two solids.
func (a *Solid) Union(b *Solid) *Solid {
	return a.binaryBool("Union", b, func(x, y *C.ManifoldPtr, ret *C.FacetSolidRet) { C.facet_union(x, y, ret) })
}

// Difference computes the boolean difference of two solids.
func (a *Solid) Difference(b *Solid) *Solid {
	return a.binaryBool("Difference", b, func(x, y *C.ManifoldPtr, ret *C.FacetSolidRet) { C.facet_difference(x, y, ret) })
}

// Intersection computes the boolean intersection of two solids.
func (a *Solid) Intersection(b *Solid) *Solid {
	return a.binaryBool("Intersection", b, func(x, y *C.ManifoldPtr, ret *C.FacetSolidRet) { C.facet_intersection(x, y, ret) })
}

// Insert cuts a hole in a for b, removes floating inner plugs, and seats b.
// Panics if either operand is nil. Returns an error if every piece of a is
// enclosed by b's convex hull, leaving no outer shell to seat b into — see
// errInsertNoShell.
func (a *Solid) Insert(b *Solid) (*Solid, error) {
	requireSolids("Insert", a, b)
	var ret C.FacetSolidRet
	C.facet_insert(a.ptr, b.ptr, &ret)
	runtime.KeepAlive(a)
	runtime.KeepAlive(b)
	s := newSolid(ret)
	if s == nil {
		return nil, errInsertNoShell
	}
	s.FaceMap = mergeFaceMaps(a.FaceMap, b.FaceMap)
	return s, nil
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

// binaryBool is the shared body of the pairwise Sketch booleans (sketches
// carry no face map). Panics if either operand is nil.
func (a *Sketch) binaryBool(name string, b *Sketch, call func(x, y *C.ManifoldCrossSection, ret *C.FacetSketchRet)) *Sketch {
	requireSketches(name, a, b)
	var ret C.FacetSketchRet
	call(a.ptr, b.ptr, &ret)
	runtime.KeepAlive(a)
	runtime.KeepAlive(b)
	return newSketch(ret)
}

// Union computes the boolean union of two sketches.
func (a *Sketch) Union(b *Sketch) *Sketch {
	return a.binaryBool("Sketch.Union", b, func(x, y *C.ManifoldCrossSection, ret *C.FacetSketchRet) { C.facet_cs_union(x, y, ret) })
}

// Difference computes the boolean difference of two sketches.
func (a *Sketch) Difference(b *Sketch) *Sketch {
	return a.binaryBool("Sketch.Difference", b, func(x, y *C.ManifoldCrossSection, ret *C.FacetSketchRet) { C.facet_cs_difference(x, y, ret) })
}

// Intersection computes the boolean intersection of two sketches.
func (a *Sketch) Intersection(b *Sketch) *Sketch {
	return a.binaryBool("Sketch.Intersection", b, func(x, y *C.ManifoldCrossSection, ret *C.FacetSketchRet) { C.facet_cs_intersection(x, y, ret) })
}

// BatchBoolean combines solids with op in a single kernel tree-reduction, which
// is far cheaper than folding pairwise booleans over a growing accumulator
// (O(N log N) vs O(N^2)). Panics on a nil operand; errors on an empty slice.
// Face maps are merged in input order (first-wins), matching the pairwise ops.
func BatchBoolean(solids []*Solid, op BoolOp) (*Solid, error) {
	if len(solids) == 0 {
		return nil, errBatchBooleanEmpty
	}
	requireSolids("BatchBoolean", solids...)
	ptrs := make([]*C.ManifoldPtr, len(solids))
	for i, s := range solids {
		ptrs[i] = s.ptr
	}
	var ret C.FacetSolidRet
	C.facet_batch_boolean(&ptrs[0], C.size_t(len(solids)), C.int(op), &ret)
	runtime.KeepAlive(solids)
	s := newSolid(ret)
	if s == nil {
		return nil, errBatchBooleanFailed
	}
	s.FaceMap = mergedFaceMaps(solids)
	return s, nil
}

// SketchBatchBoolean is the 2D counterpart of BatchBoolean (sketches carry no
// face map).
func SketchBatchBoolean(sketches []*Sketch, op BoolOp) (*Sketch, error) {
	if len(sketches) == 0 {
		return nil, errBatchBooleanEmpty
	}
	requireSketches("SketchBatchBoolean", sketches...)
	ptrs := make([]*C.ManifoldCrossSection, len(sketches))
	for i, p := range sketches {
		ptrs[i] = p.ptr
	}
	var ret C.FacetSketchRet
	C.facet_cs_batch_boolean(&ptrs[0], C.size_t(len(sketches)), C.int(op), &ret)
	runtime.KeepAlive(sketches)
	s := newSketch(ret)
	if s == nil {
		return nil, errBatchBooleanFailed
	}
	return s, nil
}
