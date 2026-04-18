package manifold

/*
#include "facet_cxx.h"
*/
import "C"
import (
	"fmt"
	"runtime"
	"unsafe"
)

// ---------------------------------------------------------------------------
// 3D Operations
// ---------------------------------------------------------------------------

// Hull computes the convex hull of a solid.
func (s *Solid) Hull() *Solid {
	ptr := C.facet_hull(s.ptr)
	runtime.KeepAlive(s)
	r := newSolid(ptr)
	origID := uint32(C.facet_original_id(r.ptr))
	runtime.KeepAlive(r)
	// Hull creates new geometry; carry over any color from the input.
	fi := FaceInfo{Color: NoColor}
	for _, v := range s.FaceMap {
		if v.Color != NoColor {
			fi.Color = v.Color
			break
		}
	}
	r.FaceMap = map[uint32]FaceInfo{origID: fi}
	return r
}

// BatchHull computes the convex hull of multiple solids together.
// Returns an error if solids is empty.
func BatchHull(solids []*Solid) (*Solid, error) {
	if len(solids) == 0 {
		return nil, fmt.Errorf("BatchHull: solids is empty")
	}
	ptrs := make([]*C.ManifoldPtr, len(solids))
	for i, s := range solids {
		ptrs[i] = s.ptr
	}
	ptr := C.facet_batch_hull(&ptrs[0], C.size_t(len(solids)))
	runtime.KeepAlive(solids)
	r := newSolid(ptr)
	origID := uint32(C.facet_original_id(r.ptr))
	runtime.KeepAlive(r)
	// Hull creates new geometry; carry over any color from inputs.
	fi := FaceInfo{Color: NoColor}
	for _, s := range solids {
		for _, v := range s.FaceMap {
			if v.Color != NoColor {
				fi.Color = v.Color
				break
			}
		}
		if fi.Color != NoColor {
			break
		}
	}
	r.FaceMap = map[uint32]FaceInfo{origID: fi}
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
	return newSolidWithOrigin(C.facet_hull_points(&coords[0], C.size_t(n))), nil
}

// TrimByPlane trims a solid by the plane defined by normal and offset.
func (s *Solid) TrimByPlane(nx, ny, nz, offset float64) *Solid {
	return transformSolid(s, C.facet_trim_by_plane(s.ptr, C.double(nx), C.double(ny), C.double(nz), C.double(offset)))
}

// SmoothOut smooths sharp edges of a solid.
func (s *Solid) SmoothOut(minSharpAngle, minSmoothness float64) *Solid {
	return transformSolid(s, C.facet_smooth_out(s.ptr, C.double(minSharpAngle), C.double(minSmoothness)))
}

// Refine subdivides the mesh of a solid n times.
func (s *Solid) Refine(n int) *Solid {
	return transformSolid(s, C.facet_refine(s.ptr, C.int(n)))
}

// Simplify reduces the triangle count of a solid by merging edges shorter than tolerance.
func (s *Solid) Simplify(tolerance float64) *Solid {
	return transformSolid(s, C.facet_simplify(s.ptr, C.double(tolerance)))
}

// RefineToLength subdivides edges longer than the given length.
func (s *Solid) RefineToLength(length float64) *Solid {
	return transformSolid(s, C.facet_refine_to_length(s.ptr, C.double(length)))
}

// SplitSolid splits m by cutter, returning [inside, outside].
func SplitSolid(m, cutter *Solid) [2]*Solid {
	pair := C.facet_split(m.ptr, cutter.ptr)
	runtime.KeepAlive(m)
	runtime.KeepAlive(cutter)
	fm := mergeFaceMaps(m.FaceMap, cutter.FaceMap)
	first := newSolid(pair.first)
	first.FaceMap = fm
	second := newSolid(pair.second)
	second.FaceMap = fm
	return [2]*Solid{first, second}
}

// SplitSolidByPlane splits a solid by an infinite plane, returning [above, below].
func SplitSolidByPlane(s *Solid, nx, ny, nz, offset float64) [2]*Solid {
	pair := C.facet_split_by_plane(s.ptr, C.double(nx), C.double(ny), C.double(nz), C.double(offset))
	runtime.KeepAlive(s)
	fm := s.withFaceMap()
	first := newSolid(pair.first)
	first.FaceMap = fm
	second := newSolid(pair.second)
	second.FaceMap = fm
	return [2]*Solid{first, second}
}

// ComposeSolids assembles non-overlapping solids into one without boolean
// operations. Returns an error if solids is empty.
func ComposeSolids(solids []*Solid) (*Solid, error) {
	if len(solids) == 0 {
		return nil, fmt.Errorf("ComposeSolids: solids is empty")
	}
	ptrs := make([]*C.ManifoldPtr, len(solids))
	for i, s := range solids {
		ptrs[i] = s.ptr
	}
	ptr := C.facet_compose((**C.ManifoldPtr)(unsafe.Pointer(&ptrs[0])), C.int(len(solids)))
	for _, s := range solids {
		runtime.KeepAlive(s)
	}
	r := newSolid(ptr)
	for _, s := range solids {
		r.FaceMap = mergeFaceMaps(r.FaceMap, s.FaceMap)
	}
	return r, nil
}

// ---------------------------------------------------------------------------
// 2D Operations
// ---------------------------------------------------------------------------

// Hull computes the convex hull of a sketch.
func (p *Sketch) Hull() *Sketch {
	ptr := C.facet_cs_hull(p.ptr)
	runtime.KeepAlive(p)
	return newSketch(ptr)
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
	ptr := C.facet_cs_batch_hull(&ptrs[0], C.size_t(len(sketches)))
	runtime.KeepAlive(sketches)
	return newSketch(ptr), nil
}

// Offset offsets a sketch's edges by delta with round join.
func (p *Sketch) Offset(delta float64, segments int) *Sketch {
	ptr := C.facet_cs_offset(p.ptr, C.double(delta), C.int(segments))
	runtime.KeepAlive(p)
	return newSketch(ptr)
}
