package manifold

/*
#include "facet_cxx.h"
*/
import "C"
import (
	"runtime"
)

// ---------------------------------------------------------------------------
// Info / Measurement (methods on resolved types)
// ---------------------------------------------------------------------------

// BoundingBox returns the axis-aligned bounding box of a solid as
// (minX, minY, minZ, maxX, maxY, maxZ) in mm.
func (s *Solid) BoundingBox() (minX, minY, minZ, maxX, maxY, maxZ float64) {
	var cMinX, cMinY, cMinZ, cMaxX, cMaxY, cMaxZ C.double
	C.facet_bounding_box(s.ptr, &cMinX, &cMinY, &cMinZ, &cMaxX, &cMaxY, &cMaxZ)
	runtime.KeepAlive(s)
	return float64(cMinX), float64(cMinY), float64(cMinZ),
		float64(cMaxX), float64(cMaxY), float64(cMaxZ)
}

// Volume returns the volume of a solid in mm³.
func (s *Solid) Volume() float64 {
	v := float64(C.facet_volume(s.ptr))
	runtime.KeepAlive(s)
	return v
}

// SurfaceArea returns the surface area of a solid in mm².
func (s *Solid) SurfaceArea() float64 {
	v := float64(C.facet_surface_area(s.ptr))
	runtime.KeepAlive(s)
	return v
}

// NumComponents returns the number of disconnected pieces in a solid.
func (s *Solid) NumComponents() int {
	n := int(C.facet_num_components(s.ptr))
	runtime.KeepAlive(s)
	return n
}

// Genus returns the topological genus of the solid (0 = sphere-like, 1 = torus-like, etc.).
func (s *Solid) Genus() int {
	result := C.facet_genus(s.ptr)
	runtime.KeepAlive(s)
	return int(result)
}

// MinGap returns the minimum distance between two solids, searching up to searchLength.
func (s *Solid) MinGap(other *Solid, searchLength float64) float64 {
	result := C.facet_min_gap(s.ptr, other.ptr, C.double(searchLength))
	runtime.KeepAlive(s)
	runtime.KeepAlive(other)
	return float64(result)
}

// BoundingBox returns the 2D axis-aligned bounding box of a sketch as
// (minX, minY, maxX, maxY) in mm.
func (p *Sketch) BoundingBox() (minX, minY, maxX, maxY float64) {
	var cMinX, cMinY, cMaxX, cMaxY C.double
	C.facet_cs_bounds(p.ptr, &cMinX, &cMinY, &cMaxX, &cMaxY)
	runtime.KeepAlive(p)
	return float64(cMinX), float64(cMinY), float64(cMaxX), float64(cMaxY)
}

// Area returns the area of a sketch in mm².
func (p *Sketch) Area() float64 {
	v := float64(C.facet_cs_area(p.ptr))
	runtime.KeepAlive(p)
	return v
}
