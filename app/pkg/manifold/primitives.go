package manifold

/*
#include "facet_cxx.h"
*/
import "C"
import (
	"fmt"
)

// ---------------------------------------------------------------------------
// 3D Primitives
// ---------------------------------------------------------------------------

// CreateCube creates a box with the given dimensions.
func CreateCube(x, y, z float64) (*Solid, error) {
	if x <= 0 || y <= 0 || z <= 0 {
		return nil, fmt.Errorf("Cube: all dimensions must be positive, got (%.4g, %.4g, %.4g)", x, y, z)
	}
	s := newSolidWithOrigin(C.facet_cube(C.double(x), C.double(y), C.double(z)))
	if s == nil {
		return nil, fmt.Errorf("manifold: failed to create cube")
	}
	return s, nil
}

// CreateSphere creates a sphere with the given radius and segment count.
// The sphere's bounding box starts at (0, 0, 0) and ends at (2r, 2r, 2r).
func CreateSphere(radius float64, segments int) (*Solid, error) {
	if radius <= 0 {
		return nil, fmt.Errorf("Sphere: radius must be positive, got %.4g", radius)
	}
	s := newSolidWithOrigin(C.facet_sphere(C.double(radius), C.int(segments)))
	if s == nil {
		return nil, fmt.Errorf("manifold: failed to create sphere")
	}
	return s, nil
}

// CreateCylinder creates a cylinder (or cone if radii differ).
// The cylinder's bounding box starts at (0, 0, 0) and ends at (2R, 2R, H)
// where R = max(radiusLow, radiusHigh).
func CreateCylinder(height, radiusLow, radiusHigh float64, segments int) (*Solid, error) {
	if height <= 0 {
		return nil, fmt.Errorf("Cylinder: height must be positive, got %.4g", height)
	}
	if radiusLow < 0 || radiusHigh < 0 {
		return nil, fmt.Errorf("Cylinder: radii must be non-negative, got (%.4g, %.4g)", radiusLow, radiusHigh)
	}
	if radiusLow == 0 && radiusHigh == 0 {
		return nil, fmt.Errorf("Cylinder: at least one radius must be positive")
	}
	s := newSolidWithOrigin(C.facet_cylinder(C.double(height), C.double(radiusLow), C.double(radiusHigh), C.int(segments)))
	if s == nil {
		return nil, fmt.Errorf("manifold: failed to create cylinder")
	}
	return s, nil
}

// ---------------------------------------------------------------------------
// 2D Primitives
// ---------------------------------------------------------------------------

// CreateSquare creates a 2D rectangle.
func CreateSquare(x, y float64) *Sketch {
	ptr := C.facet_square(C.double(x), C.double(y))
	return newSketch(ptr)
}

// CreateCircle creates a 2D circle.
// The circle's bounding box starts at (0, 0) and ends at (2r, 2r).
func CreateCircle(radius float64, segments int) *Sketch {
	ptr := C.facet_circle(C.double(radius), C.int(segments))
	s := newSketch(ptr)
	// Circle is centered at origin; translate so bounding box starts at (0,0).
	return s.Translate(radius, radius)
}

// CreatePolygon creates a 2D sketch from a slice of points.
func CreatePolygon(points []Point2D) (*Sketch, error) {
	n := len(points)
	if n < 3 {
		return nil, fmt.Errorf("Polygon requires at least 3 points, got %d", n)
	}
	// Ensure CCW winding (positive signed area) so extrusion normals face +Z.
	// Shoelace formula: positive = CCW, negative = CW.
	var area2 float64
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		area2 += points[i].X*points[j].Y - points[j].X*points[i].Y
	}
	// Copy into the C-bound coords buffer, reversing order if the caller's
	// polygon is CW.  We do NOT mutate the caller's slice — CreatePolygon is
	// an input and must not have surprising side effects on reused buffers.
	coords := make([]C.double, n*2)
	if area2 < 0 {
		for i, p := range points {
			dst := n - 1 - i
			coords[dst*2] = C.double(p.X)
			coords[dst*2+1] = C.double(p.Y)
		}
	} else {
		for i, p := range points {
			coords[i*2] = C.double(p.X)
			coords[i*2+1] = C.double(p.Y)
		}
	}
	ptr := C.facet_polygon(&coords[0], C.size_t(n))
	return newSketch(ptr), nil
}
