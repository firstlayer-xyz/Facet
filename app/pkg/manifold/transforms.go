package manifold

/*
#include "facet_cxx.h"
*/
import "C"
import (
	"fmt"
	"math"
	"runtime"
)

// ---------------------------------------------------------------------------
// 3D Transforms
// ---------------------------------------------------------------------------

// Translate moves a solid by (x, y, z).
func (s *Solid) Translate(x, y, z float64) *Solid {
	return transformSolid(s, C.facet_translate(s.ptr, C.double(x), C.double(y), C.double(z)))
}

func scale(s *Solid, x, y, z float64) *Solid {
	return transformSolid(s, C.facet_scale(s.ptr, C.double(x), C.double(y), C.double(z)))
}

func mirror(s *Solid, nx, ny, nz float64) *Solid {
	return transformSolid(s, C.facet_mirror(s.ptr, C.double(nx), C.double(ny), C.double(nz)))
}

// Rotate rotates a solid by (x, y, z) degrees around each axis, pivoting on
// the bounding box center so the solid spins in place.
func (s *Solid) Rotate(x, y, z float64) *Solid {
	return transformSolid(s, C.facet_rotate_local(s.ptr, C.double(x), C.double(y), C.double(z)))
}

// Scale scales a solid by (x, y, z) around pivot point (ox, oy, oz).
// Negative factors are permitted (they mirror along that axis).  A zero
// factor collapses a spatial dimension and produces a degenerate manifold,
// so it is rejected up front rather than letting the kernel produce an
// invalid mesh that fails much later with a confusing error.
func (s *Solid) Scale(x, y, z, ox, oy, oz float64) (*Solid, error) {
	if x == 0 || y == 0 || z == 0 {
		return nil, fmt.Errorf("Scale: factors must be non-zero (got x=%g, y=%g, z=%g)", x, y, z)
	}
	return scale(s.Translate(-ox, -oy, -oz), x, y, z).Translate(ox, oy, oz), nil
}

// Mirror mirrors a solid across the plane with normal (nx, ny, nz) at signed
// distance offset from the world origin. The normal is normalized internally.
// Returns an error if the normal has zero length (degenerate plane).
func (s *Solid) Mirror(nx, ny, nz, offset float64) (*Solid, error) {
	ln := math.Sqrt(nx*nx + ny*ny + nz*nz)
	if ln == 0 {
		return nil, fmt.Errorf("Mirror: normal vector has zero length")
	}
	dx, dy, dz := nx/ln*offset, ny/ln*offset, nz/ln*offset
	return mirror(s.Translate(-dx, -dy, -dz), nx, ny, nz).Translate(dx, dy, dz), nil
}

// RotateAt rotates a solid by (rx, ry, rz) degrees around pivot point (ox, oy, oz).
func (s *Solid) RotateAt(rx, ry, rz, ox, oy, oz float64) *Solid {
	return transformSolid(s, C.facet_rotate_at(s.ptr, C.double(rx), C.double(ry), C.double(rz), C.double(ox), C.double(oy), C.double(oz)))
}

// ---------------------------------------------------------------------------
// 2D Transforms
// ---------------------------------------------------------------------------

// Translate moves a sketch by (x, y).
func (p *Sketch) Translate(x, y float64) *Sketch {
	ptr := C.facet_cs_translate(p.ptr, C.double(x), C.double(y))
	runtime.KeepAlive(p)
	return newSketch(ptr)
}

// RotateOrigin rotates a sketch by degrees around the world origin (0, 0).
func (p *Sketch) RotateOrigin(degrees float64) *Sketch {
	ptr := C.facet_cs_rotate(p.ptr, C.double(degrees))
	runtime.KeepAlive(p)
	return newSketch(ptr)
}

func sketchScale(p *Sketch, x, y float64) *Sketch {
	ptr := C.facet_cs_scale(p.ptr, C.double(x), C.double(y))
	runtime.KeepAlive(p)
	return newSketch(ptr)
}

func sketchMirror(p *Sketch, ax, ay float64) *Sketch {
	ptr := C.facet_cs_mirror(p.ptr, C.double(ax), C.double(ay))
	runtime.KeepAlive(p)
	return newSketch(ptr)
}

// Rotate rotates a sketch by degrees, pivoting on the bounding box center.
func (p *Sketch) Rotate(degrees float64) *Sketch {
	ptr := C.facet_cs_rotate_local(p.ptr, C.double(degrees))
	runtime.KeepAlive(p)
	return newSketch(ptr)
}

// Scale scales a sketch by (x, y) around pivot point (px, py).
// Zero factors are rejected for the same reason as Solid.Scale — they
// collapse the sketch to a line or point.
func (p *Sketch) Scale(x, y, px, py float64) (*Sketch, error) {
	if x == 0 || y == 0 {
		return nil, fmt.Errorf("Scale: factors must be non-zero (got x=%g, y=%g)", x, y)
	}
	return sketchScale(p.Translate(-px, -py), x, y).Translate(px, py), nil
}

// Mirror mirrors a sketch across the axis (ax, ay) at signed distance offset
// from the world origin. The axis is normalized internally.
func (p *Sketch) Mirror(ax, ay, offset float64) (*Sketch, error) {
	ln := math.Sqrt(ax*ax + ay*ay)
	if ln == 0 {
		return nil, fmt.Errorf("Mirror: axis vector has zero length")
	}
	dx, dy := ax/ln*offset, ay/ln*offset
	return sketchMirror(p.Translate(-dx, -dy), ax, ay).Translate(dx, dy), nil
}
