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
// 2D → 3D
// ---------------------------------------------------------------------------

// Extrude extrudes a sketch upward by height.
func (p *Sketch) Extrude(height float64, slices int, twist, scaleX, scaleY float64) (*Solid, error) {
	if height == 0 {
		return nil, fmt.Errorf("Extrude: height must be non-zero")
	}
	ptr := C.facet_extrude(p.ptr, C.double(height), C.int(slices), C.double(twist), C.double(scaleX), C.double(scaleY))
	runtime.KeepAlive(p)
	s := newSolidWithOrigin(ptr)
	if s == nil {
		return nil, fmt.Errorf("manifold: failed to extrude")
	}
	return s, nil
}

// Revolve revolves a sketch around the Y axis.
func (p *Sketch) Revolve(segments int, degrees float64) (*Solid, error) {
	ptr := C.facet_revolve(p.ptr, C.int(segments), C.double(degrees))
	runtime.KeepAlive(p)
	s := newSolidWithOrigin(ptr)
	if s == nil {
		return nil, fmt.Errorf("manifold: failed to revolve")
	}
	return s, nil
}

// Sweep extrudes a sketch along a 3D path.
func (p *Sketch) Sweep(path []Point3D) (*Solid, error) {
	if len(path) < 2 {
		return nil, fmt.Errorf("Sweep requires at least 2 path points, got %d", len(path))
	}
	flat := make([]C.double, len(path)*3)
	for i, pt := range path {
		flat[i*3], flat[i*3+1], flat[i*3+2] = C.double(pt.X), C.double(pt.Y), C.double(pt.Z)
	}
	ptr := C.facet_sweep(p.ptr, &flat[0], C.size_t(len(path)))
	runtime.KeepAlive(p)
	s := newSolidWithOrigin(ptr)
	if s == nil {
		return nil, fmt.Errorf("manifold: failed to sweep")
	}
	return s, nil
}

// Loft creates a solid by blending between cross-sections at different
// heights. Requires at least 2 profiles and len(sketches) == len(heights).
func Loft(sketches []*Sketch, heights []float64) (*Solid, error) {
	if len(sketches) < 2 {
		return nil, fmt.Errorf("Loft: need at least 2 profiles, got %d", len(sketches))
	}
	if len(sketches) != len(heights) {
		return nil, fmt.Errorf("Loft: profiles and heights must have the same length, got %d and %d", len(sketches), len(heights))
	}
	ptrs := make([]*C.ManifoldCrossSection, len(sketches))
	for i, s := range sketches {
		ptrs[i] = s.ptr
	}
	hs := make([]C.double, len(heights))
	for i, h := range heights {
		hs[i] = C.double(h)
	}
	ptr := C.facet_loft(&ptrs[0], C.size_t(len(sketches)), &hs[0], C.size_t(len(heights)))
	runtime.KeepAlive(sketches)
	return newSolidWithOrigin(ptr), nil
}

// ---------------------------------------------------------------------------
// 3D → 2D
// ---------------------------------------------------------------------------

// Slice takes a cross-section of a solid at the given Z height.
func (s *Solid) Slice(height float64) *Sketch {
	ptr := C.facet_slice(s.ptr, C.double(height))
	runtime.KeepAlive(s)
	return newSketch(ptr)
}

// Project projects a solid onto the XY plane.
func (s *Solid) Project() *Sketch {
	ptr := C.facet_project(s.ptr)
	runtime.KeepAlive(s)
	return newSketch(ptr)
}
