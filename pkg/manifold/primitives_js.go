//go:build js

package manifold

import (
	"fmt"
	"syscall/js"
)

func CreateCube(x, y, z float64) (*Solid, error) {
	if err := validateCubeDims(x, y, z); err != nil {
		return nil, err
	}
	id := js.Global().Call("_mf_cube", x, y, z).Int()
	s := newSolidWithOrigin(id)
	if s == nil {
		return nil, fmt.Errorf("manifold: failed to create cube")
	}
	return s, nil
}

func CreateSphere(radius float64, segments int) (*Solid, error) {
	if err := validateSphereRadius(radius); err != nil {
		return nil, err
	}
	id := js.Global().Call("_mf_sphere", radius, segments).Int()
	s := newSolidWithOrigin(id)
	if s == nil {
		return nil, fmt.Errorf("manifold: failed to create sphere")
	}
	return s, nil
}

func CreateCylinder(height, radiusLow, radiusHigh float64, segments int) (*Solid, error) {
	if err := validateCylinder(height, radiusLow, radiusHigh); err != nil {
		return nil, err
	}
	id := js.Global().Call("_mf_cylinder", height, radiusLow, radiusHigh, segments).Int()
	s := newSolidWithOrigin(id)
	if s == nil {
		return nil, fmt.Errorf("manifold: failed to create cylinder")
	}
	return s, nil
}

func CreateSquare(x, y float64) (*Sketch, error) {
	if err := validateSquareDims(x, y); err != nil {
		return nil, err
	}
	id := js.Global().Call("_mf_square", x, y).Int()
	return newSketch(id), nil
}

func CreateCircle(radius float64, segments int) (*Sketch, error) {
	if err := validateCircleRadius(radius); err != nil {
		return nil, err
	}
	// _mf_circle translates the circle in C so its bbox starts at (0,0).
	id := js.Global().Call("_mf_circle", radius, segments).Int()
	return newSketch(id), nil
}

func CreatePolygon(outer []Point2D, holes [][]Point2D) (*Sketch, error) {
	if err := validatePolygonRings(outer, holes); err != nil {
		return nil, err
	}

	totalPoints := len(outer)
	for _, h := range holes {
		totalPoints += len(h)
	}
	coords := js.Global().Get("Float64Array").New(totalPoints * 2)
	idx := 0
	for _, p := range outer {
		coords.SetIndex(idx*2, p.X)
		coords.SetIndex(idx*2+1, p.Y)
		idx++
	}
	for _, h := range holes {
		for _, p := range h {
			coords.SetIndex(idx*2, p.X)
			coords.SetIndex(idx*2+1, p.Y)
			idx++
		}
	}

	holeSizes := js.Global().Get("Uint32Array").New(len(holes))
	for i, h := range holes {
		holeSizes.SetIndex(i, len(h))
	}

	id := js.Global().Call("_mf_polygon", coords, len(outer), holeSizes).Int()
	return newSketch(id), nil
}
