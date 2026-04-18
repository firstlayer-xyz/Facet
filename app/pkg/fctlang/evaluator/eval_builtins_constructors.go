package evaluator

import (
	"fmt"
	"math"
	"os"
	"path/filepath"

	"facet/app/pkg/fctlang/parser"
	"facet/app/pkg/manifold"
)

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
	result, err := manifold.CreateCube(x, y, z)
	if err != nil {
		return nil, err
	}
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
	result, err := manifold.CreateSphere(radius, segments)
	if err != nil {
		return nil, err
	}
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
	result, err := manifold.CreateCylinder(height, radiusLow, radiusHigh, segments)
	if err != nil {
		return nil, err
	}
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
	result, err := manifold.CreatePolygon(points)
	if err != nil {
		return nil, err
	}
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
	case *manifold.Solid:
		solids := make([]*manifold.Solid, len(arr.elems))
		for i, elem := range arr.elems {
			s, ok := elem.(*manifold.Solid)
			if !ok {
				return nil, fmt.Errorf("_hull() element %d must be a Solid, got %s", i+1, typeName(elem))
			}
			solids[i] = s
		}
		return manifold.BatchHull(solids)
	case *manifold.Sketch:
		sketches := make([]*manifold.Sketch, len(arr.elems))
		for i, elem := range arr.elems {
			p, ok := elem.(*manifold.Sketch)
			if !ok {
				return nil, fmt.Errorf("_hull() element %d must be a Sketch, got %s", i+1, typeName(elem))
			}
			sketches[i] = p
		}
		return manifold.SketchBatchHull(sketches)
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
		return manifold.HullPoints(pts)
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
	case *manifold.Solid:
		solids := make([]*manifold.Solid, len(arr.elems))
		for i, elem := range arr.elems {
			s, sOk := elem.(*manifold.Solid)
			if !sOk {
				return nil, fmt.Errorf("%s() element %d must be a Solid, got %s", name, i+1, typeName(elem))
			}
			solids[i] = s
		}
		result := solids[0]
		for _, f := range solids[1:] {
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
	case *manifold.Sketch:
		sketches := make([]*manifold.Sketch, len(arr.elems))
		for i, elem := range arr.elems {
			p, pOk := elem.(*manifold.Sketch)
			if !pOk {
				return nil, fmt.Errorf("%s() element %d must be a Sketch, got %s", name, i+1, typeName(elem))
			}
			sketches[i] = p
		}
		result := sketches[0]
		for _, f := range sketches[1:] {
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
	sketches := make([]*manifold.Sketch, len(profilesArr.elems))
	for i, elem := range profilesArr.elems {
		sf, ok := elem.(*manifold.Sketch)
		if !ok {
			return nil, fmt.Errorf("_loft() profiles[%d] must be a Sketch, got %s", i, typeName(elem))
		}
		sketches[i] = sf
	}
	heights := make([]float64, len(heightsArr.elems))
	for i, elem := range heightsArr.elems {
		h, err := requireLength("_loft", i+1, elem)
		if err != nil {
			return nil, fmt.Errorf("_loft() heights[%d]: %w", i, err)
		}
		heights[i] = h
	}
	return manifold.Loft(sketches, heights)
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
	return manifold.CreateText(fontPath, text, size)
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
// SolidFromMesh builtin
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
