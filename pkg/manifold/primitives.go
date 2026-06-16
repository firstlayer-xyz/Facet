//go:build !js

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
	if err := validateCubeDims(x, y, z); err != nil {
		return nil, err
	}
	var ret C.FacetSolidRet
	C.facet_cube(C.double(x), C.double(y), C.double(z), &ret)
	s := newSolidWithOrigin(ret)
	if s == nil {
		return nil, fmt.Errorf("manifold: failed to create cube")
	}
	return s, nil
}

// CreateSphere creates a sphere with the given radius and segment count.
// The sphere's bounding box starts at (0, 0, 0) and ends at (2r, 2r, 2r).
func CreateSphere(radius float64, segments int) (*Solid, error) {
	if err := validateSphereRadius(radius); err != nil {
		return nil, err
	}
	var ret C.FacetSolidRet
	C.facet_sphere(C.double(radius), C.int(segments), &ret)
	s := newSolidWithOrigin(ret)
	if s == nil {
		return nil, fmt.Errorf("manifold: failed to create sphere")
	}
	return s, nil
}

// CreateCylinder creates a cylinder (or cone if radii differ).
// The cylinder's bounding box starts at (0, 0, 0) and ends at (2R, 2R, H)
// where R = max(radiusLow, radiusHigh).
func CreateCylinder(height, radiusLow, radiusHigh float64, segments int) (*Solid, error) {
	if err := validateCylinder(height, radiusLow, radiusHigh); err != nil {
		return nil, err
	}
	var ret C.FacetSolidRet
	C.facet_cylinder(C.double(height), C.double(radiusLow), C.double(radiusHigh), C.int(segments), &ret)
	s := newSolidWithOrigin(ret)
	if s == nil {
		return nil, fmt.Errorf("manifold: failed to create cylinder")
	}
	return s, nil
}

// ---------------------------------------------------------------------------
// 2D Primitives
// ---------------------------------------------------------------------------

// CreateSquare creates a 2D rectangle.
func CreateSquare(x, y float64) (*Sketch, error) {
	if err := validateSquareDims(x, y); err != nil {
		return nil, err
	}
	var ret C.FacetSketchRet
	C.facet_square(C.double(x), C.double(y), &ret)
	return newSketch(ret), nil
}

// CreateCircle creates a 2D circle.
// The circle's bounding box starts at (0, 0) and ends at (2r, 2r).
// (The C side translates by (r, r) so a separate Go-side Translate cgo
// crossing isn't needed.)
func CreateCircle(radius float64, segments int) (*Sketch, error) {
	if err := validateCircleRadius(radius); err != nil {
		return nil, err
	}
	var ret C.FacetSketchRet
	C.facet_circle(C.double(radius), C.int(segments), &ret)
	return newSketch(ret), nil
}

// CreatePolygon creates a 2D sketch from an outer outline plus zero or
// more inner outlines (holes). The C++ side uses EvenOdd fill — every
// nested ring flips fill — so winding direction is irrelevant. Each
// ring must have at least 3 points. Pass `nil` holes for a plain polygon.
func CreatePolygon(outer []Point2D, holes [][]Point2D) (*Sketch, error) {
	if err := validatePolygonRings(outer, holes); err != nil {
		return nil, err
	}

	outerCoords := make([]C.double, len(outer)*2)
	for i, p := range outer {
		outerCoords[i*2] = C.double(p.X)
		outerCoords[i*2+1] = C.double(p.Y)
	}

	// Concatenate all hole coords into one flat buffer; remember each
	// hole's point count so the C side can slice the buffer back out.
	totalHolePoints := 0
	for _, h := range holes {
		totalHolePoints += len(h)
	}
	var (
		holesCoords []C.double
		holeSizes   []C.size_t
		holesPtr    *C.double
		sizesPtr    *C.size_t
	)
	if totalHolePoints > 0 {
		holesCoords = make([]C.double, totalHolePoints*2)
		holeSizes = make([]C.size_t, len(holes))
		idx := 0
		for hi, h := range holes {
			holeSizes[hi] = C.size_t(len(h))
			for _, p := range h {
				holesCoords[idx*2] = C.double(p.X)
				holesCoords[idx*2+1] = C.double(p.Y)
				idx++
			}
		}
		holesPtr = &holesCoords[0]
		sizesPtr = &holeSizes[0]
	}

	var ret C.FacetSketchRet
	C.facet_polygon(
		&outerCoords[0], C.size_t(len(outer)),
		holesPtr, sizesPtr, C.size_t(len(holes)),
		&ret,
	)
	return newSketch(ret), nil
}
