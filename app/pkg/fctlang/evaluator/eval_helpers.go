package evaluator

import (
	"fmt"

	"facet/app/pkg/manifold"
)

// requireLength extracts the mm value from a length argument.
// Also accepts bare numbers (float64), treating them as mm.
func requireLength(funcName string, argNum int, v value) (float64, error) {
	v = unwrap(v)
	switch n := v.(type) {
	case length:
		return n.mm, nil
	case float64:
		return n, nil
	default:
		return 0, fmt.Errorf("%s() argument %d must be a Length (e.g. 10 mm), got %s", funcName, argNum, typeName(v))
	}
}

// requireNumber extracts a plain numeric value from an argument.
// Accepts both float64 and length (uses the mm value).
func requireNumber(funcName string, argNum int, v value) (float64, error) {
	v = unwrap(v)
	switch n := v.(type) {
	case float64:
		return n, nil
	case length:
		return n.mm, nil
	default:
		return 0, fmt.Errorf("%s() argument %d must be a Number, got %s", funcName, argNum, typeName(v))
	}
}

// requireAngle extracts the degree value from an angle argument.
func requireAngle(funcName string, argNum int, v value) (float64, error) {
	v = unwrap(v)
	a, ok := v.(angle)
	if !ok {
		return 0, fmt.Errorf("%s() argument %d must be an Angle (e.g. 45 deg), got %s", funcName, argNum, typeName(v))
	}
	return a.deg, nil
}

// requireSolid resolves a *manifold.SolidFuture and returns the *manifold.Solid.
func requireSolid(funcName string, argNum int, v value) (*manifold.Solid, error) {
	v = unwrap(v)
	sf, ok := v.(*manifold.SolidFuture)
	if !ok {
		return nil, fmt.Errorf("%s() argument %d must be a Solid, got %s", funcName, argNum, typeName(v))
	}
	return sf.Resolve()
}

// requireSketch resolves a *manifold.SketchFuture and returns the *manifold.Sketch.
func requireSketch(funcName string, argNum int, v value) (*manifold.Sketch, error) {
	v = unwrap(v)
	pf, ok := v.(*manifold.SketchFuture)
	if !ok {
		return nil, fmt.Errorf("%s() argument %d must be a Sketch, got %s", funcName, argNum, typeName(v))
	}
	return pf.Resolve()
}

// requireString extracts a string from a value.
func requireString(funcName string, argNum int, v value) (string, error) {
	v = unwrap(v)
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%s() argument %d must be a String, got %s", funcName, argNum, typeName(v))
	}
	return s, nil
}
