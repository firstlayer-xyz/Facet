package evaluator

import (
	"fmt"

	"facet/pkg/manifold"
)

func init() {
	builtinRegistry["_rotate_origin"] = sketchMethod("_rotate_origin", func(pf *manifold.Sketch, args []value) (value, error) {
		const name = "_rotate_origin"
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args))
		}
		deg, err := requireAngle(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		return pf.RotateOrigin(deg), nil
	})

	builtinRegistry["_offset"] = sketchMethod("_offset", func(pf *manifold.Sketch, args []value) (value, error) {
		const name = "_offset"
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args))
		}
		delta, err := requireLength(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		return pf.Offset(delta, 0), nil
	})

	builtinRegistry["_area"] = sketchMethod("_area", func(pf *manifold.Sketch, args []value) (value, error) {
		const name = "_area"
		if len(args) != 0 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args))
		}
		return float64(pf.Area()), nil
	})

	builtinRegistry["_extrude"] = sketchMethod("_extrude", func(pf *manifold.Sketch, args []value) (value, error) {
		const name = "_extrude"
		if len(args) != 1 && len(args) != 5 {
			return nil, fmt.Errorf("%s() expects 1 or 5 arguments, got %d", name, len(args))
		}
		height, err := requireLength(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		if len(args) == 1 {
			return pf.Extrude(height, 0, 0, 1, 1)
		}
		slices, err := requireNumber(name, 2, args[1])
		if err != nil {
			return nil, err
		}
		twist, err := requireAngle(name, 3, args[2])
		if err != nil {
			return nil, err
		}
		scaleX, err := requireNumber(name, 4, args[3])
		if err != nil {
			return nil, err
		}
		scaleY, err := requireNumber(name, 5, args[4])
		if err != nil {
			return nil, err
		}
		sl, err := requireCount(name, 2, slices, maxSegments)
		if err != nil {
			return nil, err
		}
		return pf.Extrude(height, sl, twist, scaleX, scaleY)
	})

	builtinRegistry["_revolve"] = sketchMethod("_revolve", func(pf *manifold.Sketch, args []value) (value, error) {
		const name = "_revolve"
		if len(args) > 2 {
			return nil, fmt.Errorf("%s() expects 0 to 2 arguments, got %d", name, len(args))
		}
		degrees := 360.0
		segments := 0
		if len(args) >= 1 {
			var err error
			degrees, err = requireAngle(name, 1, args[0])
			if err != nil {
				return nil, err
			}
		}
		if len(args) == 2 {
			n, err := requireNumber(name, 2, args[1])
			if err != nil {
				return nil, err
			}
			segments, err = requireCount(name, 2, n, maxSegments)
			if err != nil {
				return nil, err
			}
		}
		// The kernel clamps only angles > 360. A NEGATIVE angle drives the
		// slice count negative while cap triangles are still emitted —
		// indexing with -1 into the vertex array (memory corruption); zero
		// yields NaN vertex math and a silent empty solid. Both are domain
		// errors here.
		if degrees <= 0 {
			return nil, fmt.Errorf("%s() angle must be positive (up to 360 deg), got %v deg", name, degrees)
		}
		return pf.Revolve(segments, degrees)
	})

	builtinRegistry["_sweep"] = sketchMethod("_sweep", func(pf *manifold.Sketch, args []value) (value, error) {
		const name = "_sweep"
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args))
		}
		arr, ok := args[0].(array)
		if !ok {
			return nil, fmt.Errorf("%s() argument 1 must be Array, got %s", name, typeName(args[0]))
		}
		if len(arr.elems) < 2 {
			return nil, fmt.Errorf("%s() path must have at least 2 points, got %d", name, len(arr.elems))
		}
		path := make([]manifold.Point3D, len(arr.elems))
		for i, elem := range arr.elems {
			px, py, pz, ok := extractVec3(elem)
			if !ok {
				return nil, fmt.Errorf("%s() path[%d] must be Vec3, got %s", name, i, typeName(elem))
			}
			path[i] = manifold.Point3D{X: px, Y: py, Z: pz}
		}
		return pf.Sweep(path)
	})
}
