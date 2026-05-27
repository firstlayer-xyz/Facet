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

func CreatePolygon(points []Point2D) (*Sketch, error) {
	n := len(points)
	if n < 3 {
		return nil, fmt.Errorf("Polygon requires at least 3 points, got %d", n)
	}
	var area2 float64
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		area2 += points[i].X*points[j].Y - points[j].X*points[i].Y
	}
	flat := make([]interface{}, n*2)
	if area2 < 0 {
		for i, p := range points {
			dst := n - 1 - i
			flat[dst*2] = p.X
			flat[dst*2+1] = p.Y
		}
	} else {
		for i, p := range points {
			flat[i*2] = p.X
			flat[i*2+1] = p.Y
		}
	}
	arr := js.Global().Get("Float64Array").New(n * 2)
	for i, v := range flat {
		arr.SetIndex(i, v)
	}
	id := js.Global().Call("_mf_polygon", arr, n).Int()
	return newSketch(id), nil
}
