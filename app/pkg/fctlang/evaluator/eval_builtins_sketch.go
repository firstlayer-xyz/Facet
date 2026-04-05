package evaluator

import (
	"fmt"

	"facet/app/pkg/manifold"
)

func init() {
	builtinRegistry["_rotate_origin"] = func(e *evaluator, args []value) (value, error) {
		const name = "_rotate_origin"
		pf, ok := args[0].(*manifold.Sketch)
		if !ok {
			return nil, fmt.Errorf("%s: expected Sketch, got %s", name, typeName(args[0]))
		}
		if len(args) != 2 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args)-1)
		}
		deg, err := requireAngle(name, 1, args[1])
		if err != nil {
			return nil, err
		}
		return pf.RotateOrigin(deg), nil
	}

	builtinRegistry["_offset"] = func(e *evaluator, args []value) (value, error) {
		const name = "_offset"
		pf, ok := args[0].(*manifold.Sketch)
		if !ok {
			return nil, fmt.Errorf("%s: expected Sketch, got %s", name, typeName(args[0]))
		}
		if len(args) != 2 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args)-1)
		}
		delta, err := requireLength(name, 1, args[1])
		if err != nil {
			return nil, err
		}
		return pf.Offset(delta, 0), nil
	}

	builtinRegistry["_area"] = func(e *evaluator, args []value) (value, error) {
		const name = "_area"
		pf, ok := args[0].(*manifold.Sketch)
		if !ok {
			return nil, fmt.Errorf("%s: expected Sketch, got %s", name, typeName(args[0]))
		}
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args)-1)
		}
		return float64(pf.Area()), nil
	}

	builtinRegistry["_extrude"] = func(e *evaluator, args []value) (value, error) {
		const name = "_extrude"
		pf, ok := args[0].(*manifold.Sketch)
		if !ok {
			return nil, fmt.Errorf("%s: expected Sketch, got %s", name, typeName(args[0]))
		}
		if len(args) != 2 && len(args) != 6 {
			return nil, fmt.Errorf("%s() expects 1 or 5 arguments, got %d", name, len(args)-1)
		}
		height, err := requireLength(name, 1, args[1])
		if err != nil {
			return nil, err
		}
		if len(args) == 2 {
			return pf.Extrude(height, 0, 0, 1, 1)
		}
		slices, err := requireNumber(name, 2, args[2])
		if err != nil {
			return nil, err
		}
		twist, err := requireAngle(name, 3, args[3])
		if err != nil {
			return nil, err
		}
		scaleX, err := requireNumber(name, 4, args[4])
		if err != nil {
			return nil, err
		}
		scaleY, err := requireNumber(name, 5, args[5])
		if err != nil {
			return nil, err
		}
		return pf.Extrude(height, int(slices), twist, scaleX, scaleY)
	}

	builtinRegistry["_revolve"] = func(e *evaluator, args []value) (value, error) {
		const name = "_revolve"
		pf, ok := args[0].(*manifold.Sketch)
		if !ok {
			return nil, fmt.Errorf("%s: expected Sketch, got %s", name, typeName(args[0]))
		}
		if len(args) > 2 {
			return nil, fmt.Errorf("%s() expects 0 or 1 arguments, got %d", name, len(args)-1)
		}
		degrees := 360.0
		if len(args) == 2 {
			var err error
			degrees, err = requireAngle(name, 1, args[1])
			if err != nil {
				return nil, err
			}
		}
		return pf.Revolve(0, degrees)
	}

	builtinRegistry["_sweep"] = func(e *evaluator, args []value) (value, error) {
		const name = "_sweep"
		pf, ok := args[0].(*manifold.Sketch)
		if !ok {
			return nil, fmt.Errorf("%s: expected Sketch, got %s", name, typeName(args[0]))
		}
		if len(args) != 2 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args)-1)
		}
		arr, ok := args[1].(array)
		if !ok {
			return nil, fmt.Errorf("%s() argument 1 must be Array, got %s", name, typeName(args[1]))
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
	}
}
