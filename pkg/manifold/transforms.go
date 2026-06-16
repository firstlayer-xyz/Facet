//go:build !js

package manifold

/*
#include "facet_cxx.h"
*/
import "C"
import (
	"runtime"
)

// ---------------------------------------------------------------------------
// 3D Transforms
// ---------------------------------------------------------------------------

// Translate moves a solid by (x, y, z).
func (s *Solid) Translate(x, y, z float64) *Solid {
	var ret C.FacetSolidRet
	C.facet_translate(s.ptr, C.double(x), C.double(y), C.double(z), &ret)
	runtime.KeepAlive(s)
	return transformSolid(s, ret)
}

// Rotate rotates a solid by (x, y, z) degrees around each axis, pivoting on
// the bounding box center so the solid spins in place.
func (s *Solid) Rotate(x, y, z float64) *Solid {
	var ret C.FacetSolidRet
	C.facet_rotate_local(s.ptr, C.double(x), C.double(y), C.double(z), &ret)
	runtime.KeepAlive(s)
	return transformSolid(s, ret)
}

// Scale scales a solid by (x, y, z) around pivot point (ox, oy, oz).
// Negative factors are permitted (they mirror along that axis). A zero,
// NaN, or infinite factor collapses or invalidates a spatial dimension and
// produces a degenerate manifold, so all three are rejected up front rather
// than letting the kernel produce an invalid mesh that fails much later
// with a confusing error.
func (s *Solid) Scale(x, y, z, ox, oy, oz float64) (*Solid, error) {
	if err := validateScaleFactor("x", x); err != nil {
		return nil, err
	}
	if err := validateScaleFactor("y", y); err != nil {
		return nil, err
	}
	if err := validateScaleFactor("z", z); err != nil {
		return nil, err
	}
	var ret C.FacetSolidRet
	C.facet_scale_at(s.ptr,
		C.double(x), C.double(y), C.double(z),
		C.double(ox), C.double(oy), C.double(oz), &ret)
	runtime.KeepAlive(s)
	return transformSolid(s, ret), nil
}

// Mirror mirrors a solid across the plane with normal (nx, ny, nz) at signed
// distance offset from the world origin. The normal is normalized in C.
// Returns an error if the normal vector's length is too small to normalize
// stably (degenerate plane).
func (s *Solid) Mirror(nx, ny, nz, offset float64) (*Solid, error) {
	if err := validateMirrorNormal3(nx, ny, nz); err != nil {
		return nil, err
	}
	var ret C.FacetSolidRet
	C.facet_mirror_at(s.ptr,
		C.double(nx), C.double(ny), C.double(nz), C.double(offset), &ret)
	runtime.KeepAlive(s)
	return transformSolid(s, ret), nil
}

// RotateAt rotates a solid by (rx, ry, rz) degrees around pivot point (ox, oy, oz).
func (s *Solid) RotateAt(rx, ry, rz, ox, oy, oz float64) *Solid {
	var ret C.FacetSolidRet
	C.facet_rotate_at(s.ptr,
		C.double(rx), C.double(ry), C.double(rz),
		C.double(ox), C.double(oy), C.double(oz), &ret)
	runtime.KeepAlive(s)
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
// Zero, NaN, and infinite factors are rejected for the same reason as
// Solid.Scale — they collapse or invalidate the transform.
func (p *Sketch) Scale(x, y, px, py float64) (*Sketch, error) {
	if err := validateScaleFactor("x", x); err != nil {
		return nil, err
	}
	if err := validateScaleFactor("y", y); err != nil {
		return nil, err
	}
	var ret C.FacetSketchRet
	C.facet_cs_scale_at(p.ptr, C.double(x), C.double(y), C.double(px), C.double(py), &ret)
	runtime.KeepAlive(p)
	return newSketch(ret), nil
}

// Mirror reflects a sketch across a line whose normal is (ax, ay) at
// signed distance offset from the world origin. So Mirror(1, 0, 0)
// flips across the Y axis (X coords negate). The normal is normalized in C.
func (p *Sketch) Mirror(ax, ay, offset float64) (*Sketch, error) {
	if err := validateMirrorNormal2(ax, ay); err != nil {
		return nil, err
	}
	var ret C.FacetSketchRet
	C.facet_cs_mirror_at(p.ptr, C.double(ax), C.double(ay), C.double(offset), &ret)
	runtime.KeepAlive(p)
	return newSketch(ret), nil
}
