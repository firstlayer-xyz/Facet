package evaluator

import (
	"fmt"
	"math"
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
			sa, oka := args[0].(*manifold.Solid)
			sb, okb := args[1].(*manifold.Solid)
			if !oka || !okb {
				return nil, fmt.Errorf("_insert() expects Solid, Solid")
			}
			return sa.Insert(sb), nil
		},
		"_decompose": func(e *evaluator, args []value) (value, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("_decompose() expects 1 argument, got %d", len(args))
			}
			sf, ok := args[0].(*manifold.Solid)
			if !ok {
				return nil, fmt.Errorf("_decompose() expects Solid, got %s", typeName(args[0]))
			}
			parts := manifold.DecomposeSolid(sf)
			elems := make([]value, len(parts))
			for i, f := range parts {
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
			solids := make([]*manifold.Solid, len(arr.elems))
			for i, elem := range arr.elems {
				sf, ok := elem.(*manifold.Solid)
				if !ok {
					return nil, fmt.Errorf("_compose() element %d is %s, expected Solid", i, typeName(elem))
				}
				solids[i] = sf
			}
			return manifold.ComposeSolids(solids), nil
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
		// Layout
		"_grid": func(_ *evaluator, args []value) (value, error) {
			const name = "_grid"
			if len(args) != 3 {
				return nil, fmt.Errorf("%s() expects 3 arguments (solids, cols, gap), got %d", name, len(args))
			}
			arr, ok := args[0].(array)
			if !ok {
				return nil, fmt.Errorf("%s() argument 1 must be []Solid, got %s", name, typeName(args[0]))
			}
			solids := make([]*manifold.Solid, len(arr.elems))
			for i, elem := range arr.elems {
				s, ok := elem.(*manifold.Solid)
				if !ok {
					return nil, fmt.Errorf("%s() element %d must be Solid, got %s", name, i, typeName(elem))
				}
				solids[i] = s
			}
			cols, err := requireNumber(name, 2, args[1])
			if err != nil {
				return nil, err
			}
			gap, err := requireLength(name, 3, args[2])
			if err != nil {
				return nil, err
			}
			result := arrangeGrid(solids, int(cols), gap)
			elems := make([]value, len(result))
			for i, s := range result {
				elems[i] = s
			}
			return array{elems: elems, elemType: "Solid"}, nil
		},
		// IO / Mesh
		"_load_mesh":       func(e *evaluator, args []value) (value, error) { return e.builtinLoadMesh(args) },
		"_text":            func(e *evaluator, args []value) (value, error) { return e.builtinNewText(args) },
		"_solid_from_mesh": func(e *evaluator, args []value) (value, error) { return e.builtinSolidFromMesh(args) },
		// Callback-based operations (require init to avoid init cycle via callFunctionVal → evalCall)
		// ---------------------------------------------------------------------------
		// Method builtins (receiver is first arg)
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
				return nil, fmt.Errorf("_level_set() argument 1 must be Function, got %s", typeName(args[0]))
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
