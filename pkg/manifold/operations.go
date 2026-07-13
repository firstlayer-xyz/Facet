//go:build !js

package manifold

/*
#include "facet_cxx.h"
*/
import "C"
import (
	"fmt"
	"runtime"
)

// ---------------------------------------------------------------------------
// 3D Operations
// ---------------------------------------------------------------------------

// Hull computes the convex hull of a solid.
func (s *Solid) Hull() *Solid {
	requireSolids("Hull", s)
	var ret C.FacetSolidRet
	C.facet_hull(s.ptr, &ret)
	runtime.KeepAlive(s)
	r := newSolid(ret)
	// Hull creates new geometry; carry over any color from the input.
	seedHullFaceMap(r, int(ret.original_id), firstFaceInfo(s))
	return r
}

// Reidentify returns the solid marked as a fresh original, so its faces carry a
// new identity distinct from whatever it was derived from. Transforms (Move,
// Rotate, …) preserve a solid's original ID, so two solids built from one
// primitive — e.g. b := a.Move(...) — otherwise share an ID and cannot be
// selected or colored apart. The evaluator calls this when a single-part solid
// is bound to a variable, so distinct variables are distinct objects with no
// user annotation. Like Hull, it collapses to a single ID and carries over any
// input color, so it is only applied to single-part solids (a multi-part solid
// already has distinct per-part identities, and collapsing would flatten them).
func (s *Solid) Reidentify() *Solid {
	requireSolids("Reidentify", s)
	var ret C.FacetSolidRet
	C.facet_as_original(s.ptr, &ret)
	runtime.KeepAlive(s)
	r := newSolid(ret)
	seedHullFaceMap(r, int(ret.original_id), firstFaceInfo(s))
	return r
}

// BatchHull computes the convex hull of multiple solids together.
// Returns an error if solids is empty.
func BatchHull(solids []*Solid) (*Solid, error) {
	if len(solids) == 0 {
		return nil, fmt.Errorf("BatchHull: solids is empty")
	}
	requireSolids("BatchHull", solids...)
	ptrs := make([]*C.ManifoldPtr, len(solids))
	for i, s := range solids {
		ptrs[i] = s.ptr
	}
	var ret C.FacetSolidRet
	C.facet_batch_hull(&ptrs[0], C.size_t(len(solids)), &ret)
	runtime.KeepAlive(solids)
	r := newSolid(ret)
	// Hull creates new geometry; carry over any color from inputs.
	seedHullFaceMap(r, int(ret.original_id), firstFaceInfo(solids...))
	return r, nil
}

// HullPoints computes the convex hull of a set of 3D points. Requires at
// least 4 points — fewer than 4 cannot form a non-degenerate 3D hull.
func HullPoints(points []Point3D) (*Solid, error) {
	n := len(points)
	if n < 4 {
		return nil, fmt.Errorf("HullPoints: need at least 4 points for a 3D hull, got %d", n)
	}
	coords := make([]C.double, n*3)
	for i, p := range points {
		coords[i*3] = C.double(p.X)
		coords[i*3+1] = C.double(p.Y)
		coords[i*3+2] = C.double(p.Z)
	}
	var ret C.FacetSolidRet
	C.facet_hull_points(&coords[0], C.size_t(n), &ret)
	return newSolidWithOrigin(ret), nil
}

// TrimByPlane trims a solid by the plane defined by normal and offset.
func (s *Solid) TrimByPlane(nx, ny, nz, offset float64) *Solid {
	var ret C.FacetSolidRet
	C.facet_trim_by_plane(s.ptr, C.double(nx), C.double(ny), C.double(nz), C.double(offset), &ret)
	runtime.KeepAlive(s)
	return transformSolid(s, ret)
}

// SmoothOut smooths sharp edges of a solid.
func (s *Solid) SmoothOut(minSharpAngle, minSmoothness float64) *Solid {
	var ret C.FacetSolidRet
	C.facet_smooth_out(s.ptr, C.double(minSharpAngle), C.double(minSmoothness), &ret)
	runtime.KeepAlive(s)
	return transformSolid(s, ret)
}

// Refine subdivides the mesh of a solid n times.
func (s *Solid) Refine(n int) *Solid {
	var ret C.FacetSolidRet
	C.facet_refine(s.ptr, C.int(n), &ret)
	runtime.KeepAlive(s)
	return transformSolid(s, ret)
}

// Simplify reduces the triangle count of a solid by merging edges shorter than tolerance.
func (s *Solid) Simplify(tolerance float64) *Solid {
	var ret C.FacetSolidRet
	C.facet_simplify(s.ptr, C.double(tolerance), &ret)
	runtime.KeepAlive(s)
	return transformSolid(s, ret)
}

// RefineToLength subdivides edges longer than the given length.
func (s *Solid) RefineToLength(length float64) *Solid {
	var ret C.FacetSolidRet
	C.facet_refine_to_length(s.ptr, C.double(length), &ret)
	runtime.KeepAlive(s)
	return transformSolid(s, ret)
}

// Offset grows (delta>0) or shrinks (delta<0) a solid by delta via an SDF
// re-mesh at edgeLen resolution (marching cubes). Approximate: resamples the
// whole body, not just the edges. Returns a possibly-empty solid (the caller
// checks Volume() for annihilation).
func (s *Solid) Offset(delta, edgeLen float64) *Solid {
	requireSolids("Offset", s)
	var ret C.FacetSolidRet
	C.facet_offset(s.ptr, C.double(delta), C.double(edgeLen), &ret)
	runtime.KeepAlive(s)
	r := newSolid(ret)
	// Offset creates new geometry; carry over any color from the input.
	seedHullFaceMap(r, int(ret.original_id), firstFaceInfo(s))
	return r
}

// splitPair wraps a kernel split result: both halves originate from src's
// geometry, so each carries its own copy of src's FaceMap.
func splitPair(src *Solid, pair C.FacetSolidPair) [2]*Solid {
	out := [2]*Solid{newSolid(pair.first), newSolid(pair.second)}
	for _, r := range out {
		if r != nil {
			r.FaceMap = src.withFaceMap()
		}
	}
	return out
}

// SplitSolid splits m by cutter, returning [inside, outside]. Both halves
// originate from m's geometry, so both carry m's FaceMap; cutter's FaceMap is
// intentionally not propagated — its faces don't appear in either result.
func SplitSolid(m, cutter *Solid) [2]*Solid {
	requireSolids("SplitSolid", m, cutter)
	var pair C.FacetSolidPair
	C.facet_split(m.ptr, cutter.ptr, &pair)
	runtime.KeepAlive(m)
	runtime.KeepAlive(cutter)
	return splitPair(m, pair)
}

// SplitSolidByPlane splits a solid by an infinite plane, returning [above, below].
func SplitSolidByPlane(s *Solid, nx, ny, nz, offset float64) [2]*Solid {
	requireSolids("SplitSolidByPlane", s)
	var pair C.FacetSolidPair
	C.facet_split_by_plane(s.ptr, C.double(nx), C.double(ny), C.double(nz), C.double(offset), &pair)
	runtime.KeepAlive(s)
	return splitPair(s, pair)
}

// ComposeSolids assembles non-overlapping solids into one without boolean
// operations. Returns an error if solids is empty.
func ComposeSolids(solids []*Solid) (*Solid, error) {
	if len(solids) == 0 {
		return nil, fmt.Errorf("ComposeSolids: solids is empty")
	}
	requireSolids("ComposeSolids", solids...)
	ptrs := make([]*C.ManifoldPtr, len(solids))
	for i, s := range solids {
		ptrs[i] = s.ptr
	}
	var ret C.FacetSolidRet
	C.facet_compose(&ptrs[0], C.int(len(solids)), &ret)
	runtime.KeepAlive(solids)
	r := newSolid(ret)
	if r == nil {
		return nil, fmt.Errorf("ComposeSolids: compose failed")
	}
	r.FaceMap = mergedFaceMaps(solids)
	return r, nil
}

// ---------------------------------------------------------------------------
// 2D Operations
// ---------------------------------------------------------------------------

// Hull computes the convex hull of a sketch.
func (p *Sketch) Hull() *Sketch {
	var ret C.FacetSketchRet
	C.facet_cs_hull(p.ptr, &ret)
	runtime.KeepAlive(p)
	return newSketch(ret)
}

// SketchBatchHull computes the convex hull of multiple sketches together.
// Returns an error if sketches is empty.
func SketchBatchHull(sketches []*Sketch) (*Sketch, error) {
	if len(sketches) == 0 {
		return nil, fmt.Errorf("SketchBatchHull: sketches is empty")
	}
	ptrs := make([]*C.ManifoldCrossSection, len(sketches))
	for i, p := range sketches {
		ptrs[i] = p.ptr
	}
	var ret C.FacetSketchRet
	C.facet_cs_batch_hull(&ptrs[0], C.size_t(len(sketches)), &ret)
	runtime.KeepAlive(sketches)
	return newSketch(ret), nil
}

// Offset offsets a sketch's edges by delta with round join.
func (p *Sketch) Offset(delta float64, segments int) *Sketch {
	var ret C.FacetSketchRet
	C.facet_cs_offset(p.ptr, C.double(delta), C.int(segments), &ret)
	runtime.KeepAlive(p)
	return newSketch(ret)
}
