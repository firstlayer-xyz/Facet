//go:build js

package manifold

import (
	"fmt"
	"syscall/js"
)

func CreateCube(x, y, z float64) (*Solid, error) {
	if x <= 0 || y <= 0 || z <= 0 {
		return nil, fmt.Errorf("Cube: all dimensions must be positive, got (%.4g, %.4g, %.4g)", x, y, z)
	}
	id := js.Global().Call("_mf_cube", x, y, z).Int()
	return newSolidWithOrigin(id), nil
}

func CreateSphere(radius float64, segments int) (*Solid, error) {
	if radius <= 0 {
		return nil, fmt.Errorf("Sphere: radius must be positive, got %.4g", radius)
	}
	id := js.Global().Call("_mf_sphere", radius, segments).Int()
	return newSolidWithOrigin(id), nil
}

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
	id := js.Global().Call("_mf_cylinder", height, radiusLow, radiusHigh, segments).Int()
	return newSolidWithOrigin(id), nil
}

func CreateSquare(x, y float64) *Sketch {
	id := js.Global().Call("_mf_square", x, y).Int()
	return newSketch(id)
}

func CreateCircle(radius float64, segments int) *Sketch {
	// _mf_circle translates the circle in C so its bbox starts at (0,0).
	id := js.Global().Call("_mf_circle", radius, segments).Int()
	return newSketch(id)
}

func CreatePolygon(outer []Point2D, holes [][]Point2D) (*Sketch, error) {
	if len(outer) < 3 {
		return nil, fmt.Errorf("Polygon outline requires at least 3 points, got %d", len(outer))
	}
	for i, h := range holes {
		if len(h) < 3 {
			return nil, fmt.Errorf("Polygon hole %d requires at least 3 points, got %d", i, len(h))
		}
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
