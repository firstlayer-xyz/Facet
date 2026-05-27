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
// 3D Transforms
// ---------------------------------------------------------------------------

// Translate moves a solid by (x, y, z).
func (s *Solid) Translate(x, y, z float64) *Solid {
	var ret C.FacetSolidRet
	C.facet_translate(s.ptr, C.double(x), C.double(y), C.double(z), &ret)
	return transformSolid(s, ret)
}

// Rotate rotates a solid by (x, y, z) degrees around each axis, pivoting on
// the bounding box center so the solid spins in place.
func (s *Solid) Rotate(x, y, z float64) *Solid {
	var ret C.FacetSolidRet
	C.facet_rotate_local(s.ptr, C.double(x), C.double(y), C.double(z), &ret)
	return transformSolid(s, ret)
}

// Scale scales a solid by (x, y, z) around pivot point (ox, oy, oz).
// Negative factors are permitted (they mirror along that axis). A zero
// factor collapses a spatial dimension and produces a degenerate manifold,
// so it is rejected up front rather than letting the kernel produce an
// invalid mesh that fails much later with a confusing error.
func (s *Solid) Scale(x, y, z, ox, oy, oz float64) (*Solid, error) {
	if x == 0 || y == 0 || z == 0 {
		return nil, fmt.Errorf("Scale: factors must be non-zero (got x=%g, y=%g, z=%g)", x, y, z)
	}
	var ret C.FacetSolidRet
	C.facet_scale_at(s.ptr,
		C.double(x), C.double(y), C.double(z),
		C.double(ox), C.double(oy), C.double(oz), &ret)
	return transformSolid(s, ret), nil
}

// Mirror mirrors a solid across the plane with normal (nx, ny, nz) at signed
// distance offset from the world origin. The normal is normalized in C.
// Returns an error if the normal has zero length (degenerate plane).
func (s *Solid) Mirror(nx, ny, nz, offset float64) (*Solid, error) {
	if nx == 0 && ny == 0 && nz == 0 {
		return nil, fmt.Errorf("Mirror: normal vector has zero length")
	}
	var ret C.FacetSolidRet
	C.facet_mirror_at(s.ptr,
		C.double(nx), C.double(ny), C.double(nz), C.double(offset), &ret)
	return transformSolid(s, ret), nil
}

// RotateAt rotates a solid by (rx, ry, rz) degrees around pivot point (ox, oy, oz).
func (s *Solid) RotateAt(rx, ry, rz, ox, oy, oz float64) *Solid {
	var ret C.FacetSolidRet
	C.facet_rotate_at(s.ptr,
		C.double(rx), C.double(ry), C.double(rz),
		C.double(ox), C.double(oy), C.double(oz), &ret)
	return transformSolid(s, ret)
}

// ---------------------------------------------------------------------------
// 2D Transforms
// ---------------------------------------------------------------------------

// Translate moves a sketch by (x, y).
func (p *Sketch) Translate(x, y float64) *Sketch {
	var ret C.FacetSketchRet
	C.facet_cs_translate(p.ptr, C.double(x), C.double(y), &ret)
	runtime.KeepAlive(p)
	return newSketch(ret)
}

// RotateOrigin rotates a sketch by degrees around the world origin (0, 0).
func (p *Sketch) RotateOrigin(degrees float64) *Sketch {
	var ret C.FacetSketchRet
	C.facet_cs_rotate(p.ptr, C.double(degrees), &ret)
	runtime.KeepAlive(p)
	return newSketch(ret)
}

// Rotate rotates a sketch by degrees, pivoting on the bounding box center.
func (p *Sketch) Rotate(degrees float64) *Sketch {
	var ret C.FacetSketchRet
	C.facet_cs_rotate_local(p.ptr, C.double(degrees), &ret)
	runtime.KeepAlive(p)
	return newSketch(ret)
}

// Scale scales a sketch by (x, y) around pivot point (px, py).
// Zero factors are rejected for the same reason as Solid.Scale — they
// collapse the sketch to a line or point.
func (p *Sketch) Scale(x, y, px, py float64) (*Sketch, error) {
	if x == 0 || y == 0 {
		return nil, fmt.Errorf("Scale: factors must be non-zero (got x=%g, y=%g)", x, y)
	}
	var ret C.FacetSketchRet
	C.facet_cs_scale_at(p.ptr, C.double(x), C.double(y), C.double(px), C.double(py), &ret)
	runtime.KeepAlive(p)
	return newSketch(ret), nil
}

// Mirror mirrors a sketch across the axis (ax, ay) at signed distance offset
// from the world origin. The axis is normalized in C.
func (p *Sketch) Mirror(ax, ay, offset float64) (*Sketch, error) {
	if ax == 0 && ay == 0 {
		return nil, fmt.Errorf("Mirror: axis vector has zero length")
	}
	var ret C.FacetSketchRet
	C.facet_cs_mirror_at(p.ptr, C.double(ax), C.double(ay), C.double(offset), &ret)
	runtime.KeepAlive(p)
	return newSketch(ret), nil
}
