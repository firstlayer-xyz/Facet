package evaluator

import (
	"facet/app/pkg/fctlang/parser"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"facet/app/pkg/manifold"
)

// builtinFn is the signature for all internal builtin functions.
// Each validates its own argument count and types, returning plain errors
// (position wrapping is handled by the caller in evalCall).
type builtinFn func(e *evaluator, args []value) (value, error)

// builtinRegistry maps _snake_case builtin names to their implementations.
// Populated by init() to avoid package-level initialization cycles.
var builtinRegistry map[string]builtinFn

// ---------------------------------------------------------------------------
// 3D Primitive builtins
// ---------------------------------------------------------------------------

func (e *evaluator) builtinCube(args []value) (value, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf("_cube() expects 3 arguments, got %d", len(args))
	}
	x, err := requireLength("_cube", 1, args[0])
	if err != nil {
		return nil, err
	}
	y, err := requireLength("_cube", 2, args[1])
	if err != nil {
		return nil, err
	}
	z, err := requireLength("_cube", 3, args[2])
	if err != nil {
		return nil, err
	}
	result := manifold.CreateCube(x, y, z)
	return result, nil
}

func (e *evaluator) builtinSphere(args []value) (value, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, fmt.Errorf("_sphere() expects 1 or 2 arguments, got %d", len(args))
	}
	radius, err := requireLength("_sphere", 1, args[0])
	if err != nil {
		return nil, err
	}
	segments := 0
	if len(args) == 2 {
		n, err := requireNumber("_sphere", 2, args[1])
		if err != nil {
			return nil, err
		}
		segments = int(n)
	}
	result := manifold.CreateSphere(radius, segments)
	return result, nil
}

func (e *evaluator) builtinCylinder(args []value) (value, error) {
	if len(args) < 3 || len(args) > 4 {
		return nil, fmt.Errorf("_cylinder() expects 3 or 4 arguments, got %d", len(args))
	}
	height, err := requireLength("_cylinder", 1, args[0])
	if err != nil {
		return nil, err
	}
	radiusLow, err := requireLength("_cylinder", 2, args[1])
	if err != nil {
		return nil, err
	}
	radiusHigh, err := requireLength("_cylinder", 3, args[2])
	if err != nil {
		return nil, err
	}
	segments := 0
	if len(args) == 4 {
		n, err := requireNumber("_cylinder", 4, args[3])
		if err != nil {
			return nil, err
		}
		segments = int(n)
	}
	result := manifold.CreateCylinder(height, radiusLow, radiusHigh, segments)
	return result, nil
}

// ---------------------------------------------------------------------------
// 2D Primitive builtins
// ---------------------------------------------------------------------------

func (e *evaluator) builtinSquare(args []value) (value, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("_square() expects 2 arguments, got %d", len(args))
	}
	x, err := requireLength("_square", 1, args[0])
	if err != nil {
		return nil, err
	}
	y, err := requireLength("_square", 2, args[1])
	if err != nil {
		return nil, err
	}
	result := manifold.CreateSquare(x, y)
	return result, nil
}

func (e *evaluator) builtinCircle(args []value) (value, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, fmt.Errorf("_circle() expects 1 or 2 arguments, got %d", len(args))
	}
	radius, err := requireLength("_circle", 1, args[0])
	if err != nil {
		return nil, err
	}
	segments := 0
	if len(args) == 2 {
		n, err := requireNumber("_circle", 2, args[1])
		if err != nil {
			return nil, err
		}
		segments = int(n)
	}
	result := manifold.CreateCircle(radius, segments)
	return result, nil
}

// makePtVecStruct creates a 2-component Vec structVal with Length fields.
func makePtVecStruct(typeName string, x, y float64) *structVal {
	return &structVal{
		typeName: typeName,
		fields: map[string]value{
			"x": length{mm: x},
			"y": length{mm: y},
		},
	}
}

// makePtVecStruct3 creates a 3-component Vec structVal with Length fields.
func makePtVecStruct3(typeName string, x, y, z float64) *structVal {
	return &structVal{
		typeName: typeName,
		fields: map[string]value{
			"x": length{mm: x},
			"y": length{mm: y},
			"z": length{mm: z},
		},
	}
}

func (e *evaluator) builtinNewPolygon(args []value) (value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("_polygon() expects 1 argument, got %d", len(args))
	}
	arr, ok := args[0].(array)
	if !ok {
		return nil, fmt.Errorf("_polygon() argument 1 must be an Array of Vec2, got %s", typeName(args[0]))
	}
	if len(arr.elems) < 3 {
		return nil, fmt.Errorf("_polygon() requires at least 3 points, got %d", len(arr.elems))
	}
	points := make([]manifold.Point2D, len(arr.elems))
	for i, elem := range arr.elems {
		x, y, ok := extractVec2(elem)
		if !ok {
			return nil, fmt.Errorf("_polygon() element %d must be a Vec2, got %s", i+1, typeName(elem))
		}
		points[i] = manifold.Point2D{X: x, Y: y}
	}
	result := manifold.CreatePolygon(points)
	return result, nil
}

func (e *evaluator) builtinHull(_ *parser.CallExpr, args []value) (value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("_hull() expects 1 argument (an Array), got %d", len(args))
	}
	arr, ok := args[0].(array)
	if !ok {
		return nil, fmt.Errorf("_hull() argument must be an Array, got %s", typeName(args[0]))
	}
	if len(arr.elems) == 0 {
		return nil, fmt.Errorf("_hull() requires at least 1 element")
	}

	// Determine type from first element
	switch arr.elems[0].(type) {
	case *manifold.SolidFuture:
		futures := make([]*manifold.SolidFuture, len(arr.elems))
		for i, elem := range arr.elems {
			s, ok := elem.(*manifold.SolidFuture)
			if !ok {
				return nil, fmt.Errorf("_hull() element %d must be a Solid, got %s", i+1, typeName(elem))
			}
			futures[i] = s
		}
		result := manifold.BatchHull(futures)
		return result, nil
	case *manifold.SketchFuture:
		futures := make([]*manifold.SketchFuture, len(arr.elems))
		for i, elem := range arr.elems {
			p, ok := elem.(*manifold.SketchFuture)
			if !ok {
				return nil, fmt.Errorf("_hull() element %d must be a Sketch, got %s", i+1, typeName(elem))
			}
			futures[i] = p
		}
		result := manifold.SketchBatchHull(futures)
		return result, nil
	case *structVal:
		if sv := arr.elems[0].(*structVal); sv == nil || sv.typeName != "Vec3" {
			return nil, fmt.Errorf("_hull() elements must be Solids, Sketches, or Vec3, got %s", typeName(arr.elems[0]))
		}
		pts := make([]manifold.Point3D, len(arr.elems))
		for i, elem := range arr.elems {
			x, y, z, ok := extractVec3(elem)
			if !ok {
				return nil, fmt.Errorf("_hull() element %d must be a Vec3, got %s", i+1, typeName(elem))
			}
			pts[i] = manifold.Point3D{X: x, Y: y, Z: z}
		}
		result := manifold.HullPoints(pts)
		return result, nil
	default:
		return nil, fmt.Errorf("_hull() elements must be Solids, Sketches, or Vec3, got %s", typeName(arr.elems[0]))
	}
}

// ---------------------------------------------------------------------------
// Batch boolean builtins (Union, Difference, Intersection)
// ---------------------------------------------------------------------------

func (e *evaluator) builtinBatchBool(name string, args []value) (value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("%s() expects 1 argument (an Array), got %d", name, len(args))
	}
	arr, ok := args[0].(array)
	if !ok {
		return nil, fmt.Errorf("%s() argument must be an Array, got %s", name, typeName(args[0]))
	}
	if len(arr.elems) < 2 {
		return nil, fmt.Errorf("%s() requires at least 2 elements, got %d", name, len(arr.elems))
	}

	switch arr.elems[0].(type) {
	case *manifold.SolidFuture:
		futures := make([]*manifold.SolidFuture, len(arr.elems))
		for i, elem := range arr.elems {
			s, sOk := elem.(*manifold.SolidFuture)
			if !sOk {
				return nil, fmt.Errorf("%s() element %d must be a Solid, got %s", name, i+1, typeName(elem))
			}
			futures[i] = s
		}
		result := futures[0]
		for _, f := range futures[1:] {
			switch name {
			case "_union":
				result = result.Union(f)
			case "_difference":
				result = result.Difference(f)
			case "_intersection":
				result = result.Intersection(f)
			}
		}
		return result, nil
	case *manifold.SketchFuture:
		futures := make([]*manifold.SketchFuture, len(arr.elems))
		for i, elem := range arr.elems {
			p, pOk := elem.(*manifold.SketchFuture)
			if !pOk {
				return nil, fmt.Errorf("%s() element %d must be a Sketch, got %s", name, i+1, typeName(elem))
			}
			futures[i] = p
		}
		result := futures[0]
		for _, f := range futures[1:] {
			switch name {
			case "_union":
				result = result.Union(f)
			case "_difference":
				result = result.Difference(f)
			case "_intersection":
				result = result.Intersection(f)
			}
		}
		return result, nil
	default:
		return nil, fmt.Errorf("%s() elements must be Solids or Sketches, got %s", name, typeName(arr.elems[0]))
	}
}

// ---------------------------------------------------------------------------
// Loft builtin
// ---------------------------------------------------------------------------

func (e *evaluator) builtinLoft(args []value) (value, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("_loft() expects 2 arguments, got %d", len(args))
	}
	profilesArr, ok := args[0].(array)
	if !ok {
		return nil, fmt.Errorf("_loft() argument 1 must be an Array of Sketch, got %s", typeName(args[0]))
	}
	heightsArr, ok := args[1].(array)
	if !ok {
		return nil, fmt.Errorf("_loft() argument 2 must be an Array of Length, got %s", typeName(args[1]))
	}
	if len(profilesArr.elems) != len(heightsArr.elems) {
		return nil, fmt.Errorf("_loft() profiles and heights must have the same length, got %d and %d", len(profilesArr.elems), len(heightsArr.elems))
	}
	if len(profilesArr.elems) < 2 {
		return nil, fmt.Errorf("_loft() requires at least 2 profiles, got %d", len(profilesArr.elems))
	}
	futures := make([]*manifold.SketchFuture, len(profilesArr.elems))
	for i, elem := range profilesArr.elems {
		sf, ok := elem.(*manifold.SketchFuture)
		if !ok {
			return nil, fmt.Errorf("_loft() profiles[%d] must be a Sketch, got %s", i, typeName(elem))
		}
		futures[i] = sf
	}
	heights := make([]float64, len(heightsArr.elems))
	for i, elem := range heightsArr.elems {
		l, ok := elem.(length)
		if !ok {
			return nil, fmt.Errorf("_loft() heights[%d] must be a Length, got %s", i, typeName(elem))
		}
		heights[i] = l.mm
	}
	return manifold.Loft(futures, heights), nil
}

// ---------------------------------------------------------------------------
// Mesh import builtin
// ---------------------------------------------------------------------------

func (e *evaluator) builtinLoadMesh(args []value) (value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("_load_mesh() expects 1 argument, got %d", len(args))
	}
	path, err := requireString("_load_mesh", 1, args[0])
	if err != nil {
		return nil, err
	}

	// Resolve relative paths against CWD
	if !filepath.IsAbs(path) {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("_load_mesh(): failed to get working directory: %w", err)
		}
		path = filepath.Join(cwd, path)
	}

	result, err := manifold.ImportMesh(path)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// Text builtin
// ---------------------------------------------------------------------------

func (e *evaluator) builtinNewText(args []value) (value, error) {
	if len(args) < 2 || len(args) > 3 {
		return nil, fmt.Errorf("_text() expects 2 or 3 arguments, got %d", len(args))
	}
	text, err := requireString("_text", 1, args[0])
	if err != nil {
		return nil, err
	}
	size, err := requireLength("_text", 2, args[1])
	if err != nil {
		return nil, err
	}
	fontPath := ""
	if len(args) == 3 {
		fontPath, err = requireString("_text", 3, args[2])
		if err != nil {
			return nil, err
		}
	}
	if fontPath == "" {
		fontPath = manifold.DefaultFontPath()
	} else if !filepath.IsAbs(fontPath) {
		cwd, cwdErr := os.Getwd()
		if cwdErr != nil {
			return nil, fmt.Errorf("_text(): failed to get working directory: %w", cwdErr)
		}
		fontPath = filepath.Join(cwd, fontPath)
	}
	return manifold.CreateText(fontPath, text, size), nil
}

// ---------------------------------------------------------------------------
// Math helpers
// ---------------------------------------------------------------------------

// coerceNumericArgs promotes bare Numbers to Length or Angle when any arg in
// the slice is Length or Angle, so that mixed calls like Min(5 mm, 0) work.
func coerceNumericArgs(args []value) {
	// Detect target type: first Length or Angle wins.
	var toLength, toAngle bool
	for _, a := range args {
		switch a.(type) {
		case length:
			toLength = true
		case angle:
			toAngle = true
		}
	}
	if !toLength && !toAngle {
		return
	}
	for i, a := range args {
		if n, ok := a.(float64); ok {
			if toLength {
				args[i] = length{mm: n}
			} else if toAngle {
				args[i] = angle{deg: n}
			}
		}
	}
}


// ---------------------------------------------------------------------------
// SolidFromMesh builtin (moved from eval_call.go)
// ---------------------------------------------------------------------------

func (e *evaluator) builtinSolidFromMesh(args []value) (value, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("_solid_from_mesh() expects 2 arguments, got %d", len(args))
	}
	vertsArr, ok := args[0].(array)
	if !ok {
		return nil, fmt.Errorf("_solid_from_mesh() argument 1 must be []Vec3, got %s", typeName(args[0]))
	}
	indicesArr, ok := args[1].(array)
	if !ok {
		return nil, fmt.Errorf("_solid_from_mesh() argument 2 must be []Face, got %s", typeName(args[1]))
	}
	vertices := make([]float32, len(vertsArr.elems)*3)
	for i, v := range vertsArr.elems {
		x, y, z, ok := extractVec3(v)
		if !ok {
			return nil, fmt.Errorf("_solid_from_mesh() vertices[%d] must be Vec3, got %s", i, typeName(v))
		}
		vertices[i*3+0] = float32(x)
		vertices[i*3+1] = float32(y)
		vertices[i*3+2] = float32(z)
	}
	indices := make([]uint32, len(indicesArr.elems)*3)
	for i, v := range indicesArr.elems {
		sv, svOk := v.(*structVal)
		if !svOk {
			return nil, fmt.Errorf("_solid_from_mesh() indices[%d] must be Face, got %s", i, typeName(v))
		}
		v0, err := requireNumber("_solid_from_mesh", 1, sv.fields["v0"])
		if err != nil {
			return nil, fmt.Errorf("_solid_from_mesh() indices[%d].v0: %w", i, err)
		}
		v1, err := requireNumber("_solid_from_mesh", 2, sv.fields["v1"])
		if err != nil {
			return nil, fmt.Errorf("_solid_from_mesh() indices[%d].v1: %w", i, err)
		}
		v2, err := requireNumber("_solid_from_mesh", 3, sv.fields["v2"])
		if err != nil {
			return nil, fmt.Errorf("_solid_from_mesh() indices[%d].v2: %w", i, err)
		}
		numVerts := float64(len(vertsArr.elems))
		for j, idx := range []float64{v0, v1, v2} {
			if idx < 0 || idx >= numVerts || idx != math.Floor(idx) {
				return nil, fmt.Errorf("_solid_from_mesh() indices[%d].v%d: index %v out of range [0, %d)", i, j, idx, len(vertsArr.elems))
			}
		}
		indices[i*3+0] = uint32(v0)
		indices[i*3+1] = uint32(v1)
		indices[i*3+2] = uint32(v2)
	}
	return manifold.CreateSolidFromMesh(vertices, indices)
}

func init() {
	builtinRegistry = map[string]builtinFn{
		// 3D primitives
		"_cube":     func(e *evaluator, args []value) (value, error) { return e.builtinCube(args) },
		"_sphere":   func(e *evaluator, args []value) (value, error) { return e.builtinSphere(args) },
		"_cylinder": func(e *evaluator, args []value) (value, error) { return e.builtinCylinder(args) },
		// 2D primitives
		"_square": func(e *evaluator, args []value) (value, error) { return e.builtinSquare(args) },
		"_circle": func(e *evaluator, args []value) (value, error) { return e.builtinCircle(args) },
		// Constructors
		"_polygon": func(e *evaluator, args []value) (value, error) { return e.builtinNewPolygon(args) },
		// Aggregate
		"_hull":         func(e *evaluator, args []value) (value, error) { return e.builtinHull(nil, args) },
		"_union":        func(e *evaluator, args []value) (value, error) { return e.builtinBatchBool("_union", args) },
		"_difference":   func(e *evaluator, args []value) (value, error) { return e.builtinBatchBool("_difference", args) },
		"_intersection": func(e *evaluator, args []value) (value, error) { return e.builtinBatchBool("_intersection", args) },
		"_insert": func(e *evaluator, args []value) (value, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("_insert() expects 2 arguments, got %d", len(args))
			}
			sa, oka := args[0].(*manifold.SolidFuture)
			sb, okb := args[1].(*manifold.SolidFuture)
			if !oka || !okb {
				return nil, fmt.Errorf("_insert() expects Solid, Solid")
			}
			return sa.Insert(sb), nil
		},
		"_decompose": func(e *evaluator, args []value) (value, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("_decompose() expects 1 argument, got %d", len(args))
			}
			sf, ok := args[0].(*manifold.SolidFuture)
			if !ok {
				return nil, fmt.Errorf("_decompose() expects Solid, got %s", typeName(args[0]))
			}
			futures, err := manifold.DecomposeSolid(sf)
			if err != nil {
				return nil, err
			}
			elems := make([]value, len(futures))
			for i, f := range futures {
				elems[i] = f
			}
			return array{elems: elems, elemType: "Solid"}, nil
		},
		"_compose": func(e *evaluator, args []value) (value, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("_compose() expects 1 argument, got %d", len(args))
			}
			arr, ok := args[0].(array)
			if !ok {
				return nil, fmt.Errorf("_compose() expects []Solid, got %s", typeName(args[0]))
			}
			if len(arr.elems) == 0 {
				return nil, fmt.Errorf("_compose() requires at least one Solid")
			}
			futures := make([]*manifold.SolidFuture, len(arr.elems))
			for i, elem := range arr.elems {
				sf, ok := elem.(*manifold.SolidFuture)
				if !ok {
					return nil, fmt.Errorf("_compose() element %d is %s, expected Solid", i, typeName(elem))
				}
				futures[i] = sf
			}
			return manifold.ComposeSolids(futures), nil
		},
		// Trig
		"_sin":   builtinSin,
		"_cos":   builtinCos,
		"_tan":   builtinTan,
		"_asin":  builtinAsin,
		"_acos":  builtinAcos,
		"_atan2": builtinAtan2,
		// Math
		"_min":   func(_ *evaluator, args []value) (value, error) { return builtinMin(args) },
		"_max":   func(_ *evaluator, args []value) (value, error) { return builtinMax(args) },
		"_abs":   func(_ *evaluator, args []value) (value, error) { return builtinAbs(args) },
		"_sqrt":  builtinSqrt,
		"_pow":   builtinPow,
		"_floor": builtinFloor,
		"_ceil":  builtinCeil,
		"_round": builtinRound,
		"_lerp":  func(_ *evaluator, args []value) (value, error) { return builtinLerp(args) },
		// Conversion
		"_string": builtinString,
		"_number": builtinNumber,
		"_size":   builtinSize,
		// Time
		"_utc_date": builtinUtcDate,
		"_utc_time": builtinUtcTime,
		// Aggregate (multi-sketch)
		"_loft": func(e *evaluator, args []value) (value, error) { return e.builtinLoft(args) },
		// Color
		"_color_from_hex": func(_ *evaluator, args []value) (value, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("_color_from_hex() expects 1 argument, got %d", len(args))
			}
			hex, ok := args[0].(string)
			if !ok {
				return nil, fmt.Errorf("_color_from_hex() argument must be String, got %s", typeName(args[0]))
			}
			r, g, b, a, err := parseHexColorRGBA(hex)
			if err != nil {
				return nil, fmt.Errorf("_color_from_hex(): %w", err)
			}
			return &structVal{
				typeName: "Color",
				fields: map[string]value{
					"r": r,
					"g": g,
					"b": b,
					"a": a,
				},
			}, nil
		},
		// IO / Mesh
		"_load_mesh":       func(e *evaluator, args []value) (value, error) { return e.builtinLoadMesh(args) },
		"_text":            func(e *evaluator, args []value) (value, error) { return e.builtinNewText(args) },
		"_solid_from_mesh": func(e *evaluator, args []value) (value, error) { return e.builtinSolidFromMesh(args) },
		// Callback-based operations (require init to avoid init cycle via callFunctionVal → evalCall)
		// ---------------------------------------------------------------------------
		// Method builtins (former self.method() calls, now free functions)
		// ---------------------------------------------------------------------------

		// Solid methods, Solid+Sketch shared methods, and Sketch-only _fillet/_chamfer
		// are registered in eval_builtins_solid.go init().

		// Sketch-only methods (_rotate_origin, _offset, _extrude, _area, _revolve, _sweep)
		// are registered in eval_builtins_sketch.go init().
		// String methods are registered in eval_builtins_string.go init().
		// _split is registered in eval_builtins_solid.go init() (handles both String and Solid).

		// Struct methods (Mesh, PolyMesh, Color) are registered in eval_builtins_struct.go init().

		"_level_set": func(e *evaluator, args []value) (value, error) {
			if len(args) != 3 {
				return nil, fmt.Errorf("_level_set() expects 3 arguments (fn, Box, Length), got %d", len(args))
			}
			fv, ok := args[0].(*functionVal)
			if !ok {
				return nil, fmt.Errorf("_level_set() argument 1 must be parser.Function, got %s", typeName(args[0]))
			}
			boxSV, ok := args[1].(*structVal)
			if !ok || boxSV.typeName != "Box" {
				return nil, fmt.Errorf("_level_set() argument 2 must be Box, got %s", typeName(args[1]))
			}
			edgeLen, err := requireLength("_level_set", 3, args[2])
			if err != nil {
				return nil, err
			}
			minVec, ok1 := boxSV.fields["min"].(*structVal)
			maxVec, ok2 := boxSV.fields["max"].(*structVal)
			if !ok1 || !ok2 {
				return nil, fmt.Errorf("_level_set() Box must have min and max Vec3 fields")
			}
			minX, minY, minZ, ok3 := extractVec3(minVec)
			maxX, maxY, maxZ, ok4 := extractVec3(maxVec)
			if !ok3 || !ok4 {
				return nil, fmt.Errorf("_level_set() Box min/max must be Vec3")
			}
			var lsErr error
			solid := manifold.LevelSet(func(x, y, z float64) float64 {
				if lsErr != nil {
					return 1
				}
				pt := makePtVecStruct3("Vec3", x, y, z)
				result, callErr := e.callFunctionVal(fv, map[string]value{fv.params[0].Name: pt})
				if callErr != nil {
					lsErr = callErr
					return 1 // positive = outside (safe default)
				}
				switch n := result.(type) {
				case float64:
					return n
				case length:
					return n.mm
				default:
					lsErr = fmt.Errorf("LevelSet callback must return Number or Length, got %s", typeName(result))
					return 1
				}
			}, minX, minY, minZ, maxX, maxY, maxZ, edgeLen)
			if lsErr != nil {
				return nil, lsErr
			}
			return solid, nil
		},
	}
}

func mathLerp(args []value) (value, error) {
	t, err := requireNumber("Lerp", 3, args[2])
	if err != nil {
		return nil, err
	}
	// Coerce Number → Length/Angle so mixed calls like Lerp(5 mm, 0, 0.5) work.
	coerceNumericArgs(args[:2])
	switch a := args[0].(type) {
	case length:
		b, ok := args[1].(length)
		if !ok {
			return nil, fmt.Errorf("Lerp() first two arguments must be the same type, got %s and %s", typeName(args[0]), typeName(args[1]))
		}
		return length{mm: a.mm + (b.mm-a.mm)*t}, nil
	case float64:
		bn, ok := args[1].(float64)
		if !ok {
			return nil, fmt.Errorf("Lerp() first two arguments must be the same type, got %s and %s", typeName(args[0]), typeName(args[1]))
		}
		return a + (bn-a)*t, nil
	case angle:
		b, ok := args[1].(angle)
		if !ok {
			return nil, fmt.Errorf("Lerp() first two arguments must be the same type, got %s and %s", typeName(args[0]), typeName(args[1]))
		}
		return angle{deg: a.deg + (b.deg-a.deg)*t}, nil
	default:
		return nil, fmt.Errorf("Lerp() arguments must be numeric, got %s", typeName(args[0]))
	}
}

// polyMeshToStructVal converts a Go *manifold.PolyMesh to a language-level structVal.
func polyMeshToStructVal(pm *manifold.PolyMesh) *structVal {
	nv := len(pm.Vertices) / 3
	vertElems := make([]value, nv)
	for i := 0; i < nv; i++ {
		vertElems[i] = makePtVecStruct3("Vec3", pm.Vertices[i*3], pm.Vertices[i*3+1], pm.Vertices[i*3+2])
	}
	faceElems := make([]value, len(pm.Faces))
	for i, face := range pm.Faces {
		idxElems := make([]value, len(face))
		for j, idx := range face {
			idxElems[j] = float64(idx)
		}
		faceElems[i] = array{elems: idxElems, elemType: "Number"}
	}
	return &structVal{
		typeName: "PolyMesh",
		fields: map[string]value{
			"vertices": array{elems: vertElems, elemType: "Vec3"},
			"faces":    array{elems: faceElems, elemType: "Array"},
		},
	}
}

// structValToPolyMesh converts a language-level PolyMesh structVal to a Go *manifold.PolyMesh.
func structValToPolyMesh(sv *structVal) (*manifold.PolyMesh, error) {
	vertsArr, ok := sv.fields["vertices"].(array)
	if !ok {
		return nil, fmt.Errorf("PolyMesh.vertices must be []Vec3")
	}
	facesArr, ok := sv.fields["faces"].(array)
	if !ok {
		return nil, fmt.Errorf("PolyMesh.faces must be Number[][]")
	}
	vertices := make([]float64, len(vertsArr.elems)*3)
	for i, v := range vertsArr.elems {
		px, py, pz, ok := extractVec3(v)
		if !ok {
			return nil, fmt.Errorf("PolyMesh.vertices[%d] must be Vec3, got %s", i, typeName(v))
		}
		vertices[i*3+0] = px
		vertices[i*3+1] = py
		vertices[i*3+2] = pz
	}
	faces := make([][]int, len(facesArr.elems))
	for i, v := range facesArr.elems {
		faceArr, ok := v.(array)
		if !ok {
			return nil, fmt.Errorf("PolyMesh.faces[%d] must be Number[], got %s", i, typeName(v))
		}
		face := make([]int, len(faceArr.elems))
		for j, idx := range faceArr.elems {
			n, err := requireNumber("PolyMesh.faces", j+1, idx)
			if err != nil {
				return nil, err
			}
			face[j] = int(n)
		}
		faces[i] = face
	}
	return &manifold.PolyMesh{Vertices: vertices, Faces: faces}, nil
}

func meshFaceNormals(sv *structVal, args []value) (value, error) {
	const name = "_face_normals"
	if len(args) != 0 {
		return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args))
	}
	vertsArr, ok := sv.fields["vertices"].(array)
	if !ok {
		return nil, fmt.Errorf("%s() requires vertices field to be an Array", name)
	}
	indicesArr, ok := sv.fields["indices"].(array)
	if !ok {
		return nil, fmt.Errorf("%s() requires indices field to be an Array", name)
	}

	// Extract vertices as float64 triples
	verts := make([][3]float64, len(vertsArr.elems))
	for i, v := range vertsArr.elems {
		px, py, pz, ok := extractVec3(v)
		if !ok {
			return nil, fmt.Errorf("%s() vertex %d is %s, expected Vec3", name, i, typeName(v))
		}
		verts[i] = [3]float64{px, py, pz}
	}

	// Compute face normals
	normals := make([]value, len(indicesArr.elems))
	for i, v := range indicesArr.elems {
		face, ok := v.(*structVal)
		if !ok {
			return nil, fmt.Errorf("%s() index %d is %s, expected Face struct", name, i, typeName(v))
		}
		v0, ok0 := face.fields["v0"].(float64)
		v1, ok1 := face.fields["v1"].(float64)
		v2, ok2 := face.fields["v2"].(float64)
		if !ok0 || !ok1 || !ok2 {
			return nil, fmt.Errorf("%s() face %d has non-numeric vertex indices", name, i)
		}
		i0 := int(v0)
		i1 := int(v1)
		i2 := int(v2)
		if i0 < 0 || i0 >= len(verts) || i1 < 0 || i1 >= len(verts) || i2 < 0 || i2 >= len(verts) {
			return nil, fmt.Errorf("%s() face %d has out-of-bounds vertex index", name, i)
		}
		p0, p1, p2 := verts[i0], verts[i1], verts[i2]
		// Edge vectors
		e1x, e1y, e1z := p1[0]-p0[0], p1[1]-p0[1], p1[2]-p0[2]
		e2x, e2y, e2z := p2[0]-p0[0], p2[1]-p0[1], p2[2]-p0[2]
		// Cross product
		nx := e1y*e2z - e1z*e2y
		ny := e1z*e2x - e1x*e2z
		nz := e1x*e2y - e1y*e2x
		// Normalize
		ln := math.Sqrt(nx*nx + ny*ny + nz*nz)
		if ln > 0 {
			nx, ny, nz = nx/ln, ny/ln, nz/ln
		}
		normals[i] = makePtVecStruct3("Vec3", nx, ny, nz)
	}
	return array{elems: normals, elemType: "Vec3"}, nil
}

func meshVertexNormals(sv *structVal, args []value) (value, error) {
	const name = "_vertex_normals"
	if len(args) != 0 {
		return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args))
	}
	vertsArr, ok := sv.fields["vertices"].(array)
	if !ok {
		return nil, fmt.Errorf("%s() requires vertices field to be an Array", name)
	}
	indicesArr, ok := sv.fields["indices"].(array)
	if !ok {
		return nil, fmt.Errorf("%s() requires indices field to be an Array", name)
	}
	numVerts := len(vertsArr.elems)

	// Extract vertices
	verts := make([][3]float64, numVerts)
	for i, v := range vertsArr.elems {
		px, py, pz, ok := extractVec3(v)
		if !ok {
			return nil, fmt.Errorf("%s() vertex %d is %s, expected Vec3", name, i, typeName(v))
		}
		verts[i] = [3]float64{px, py, pz}
	}

	// Accumulate face normals per vertex
	acc := make([][3]float64, numVerts)
	for fi, v := range indicesArr.elems {
		face, ok := v.(*structVal)
		if !ok {
			return nil, fmt.Errorf("%s() index %d is %s, expected Face struct", name, fi, typeName(v))
		}
		fv0, ok0 := face.fields["v0"].(float64)
		fv1, ok1 := face.fields["v1"].(float64)
		fv2, ok2 := face.fields["v2"].(float64)
		if !ok0 || !ok1 || !ok2 {
			return nil, fmt.Errorf("%s() face %d has non-numeric vertex indices", name, fi)
		}
		i0 := int(fv0)
		i1 := int(fv1)
		i2 := int(fv2)
		if i0 < 0 || i0 >= numVerts || i1 < 0 || i1 >= numVerts || i2 < 0 || i2 >= numVerts {
			return nil, fmt.Errorf("%s() face %d has out-of-bounds vertex index", name, fi)
		}
		p0, p1, p2 := verts[i0], verts[i1], verts[i2]
		e1x, e1y, e1z := p1[0]-p0[0], p1[1]-p0[1], p1[2]-p0[2]
		e2x, e2y, e2z := p2[0]-p0[0], p2[1]-p0[1], p2[2]-p0[2]
		nx := e1y*e2z - e1z*e2y
		ny := e1z*e2x - e1x*e2z
		nz := e1x*e2y - e1y*e2x
		ln := math.Sqrt(nx*nx + ny*ny + nz*nz)
		if ln > 0 {
			nx, ny, nz = nx/ln, ny/ln, nz/ln
		}
		acc[i0][0] += nx; acc[i0][1] += ny; acc[i0][2] += nz
		acc[i1][0] += nx; acc[i1][1] += ny; acc[i1][2] += nz
		acc[i2][0] += nx; acc[i2][1] += ny; acc[i2][2] += nz
	}

	// Normalize per-vertex normals
	normals := make([]value, numVerts)
	for i, a := range acc {
		ln := math.Sqrt(a[0]*a[0] + a[1]*a[1] + a[2]*a[2])
		if ln > 0 {
			normals[i] = makePtVecStruct3("Vec3", a[0]/ln, a[1]/ln, a[2]/ln)
		} else {
			normals[i] = makePtVecStruct3("Vec3", 0, 0, 0)
		}
	}
	return array{elems: normals, elemType: "Vec3"}, nil
}

// clampByte converts a 0-1 float to a 0-255 int, clamping to [0, 255].
func clampByte(f float64) int {
	v := int(f*255 + 0.5)
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

// parseHexColor parses "#RGB" or "#RRGGBB" to float64 r, g, b in 0-1.
func parseHexColor(s string) (float64, float64, float64, error) {
	r, g, b, _, err := parseHexColorRGBA(s)
	return r, g, b, err
}

// parseHexColorRGBA parses "#RGB", "#RRGGBB", or "#RRGGBBAA" to float64 r, g, b, a in 0-1.
func parseHexColorRGBA(s string) (float64, float64, float64, float64, error) {
	s = strings.TrimPrefix(s, "#")
	var r, g, b, a uint8
	a = 255
	switch len(s) {
	case 3:
		_, err := fmt.Sscanf(s, "%1x%1x%1x", &r, &g, &b)
		if err != nil {
			return 0, 0, 0, 0, fmt.Errorf("invalid hex color %q", s)
		}
		r, g, b = r*17, g*17, b*17
	case 6:
		_, err := fmt.Sscanf(s, "%02x%02x%02x", &r, &g, &b)
		if err != nil {
			return 0, 0, 0, 0, fmt.Errorf("invalid hex color %q", s)
		}
	case 8:
		_, err := fmt.Sscanf(s, "%02x%02x%02x%02x", &r, &g, &b, &a)
		if err != nil {
			return 0, 0, 0, 0, fmt.Errorf("invalid hex color %q", s)
		}
	default:
		return 0, 0, 0, 0, fmt.Errorf("hex color must be 3, 6, or 8 digits, got %q", s)
	}
	return float64(r) / 255, float64(g) / 255, float64(b) / 255, float64(a) / 255, nil
}
