package evaluator

import (
	"fmt"
	"math"
	"strings"

	"facet/app/pkg/manifold"
)

func init() {
	// ---------------------------------------------------------------------------
	// Solid + Sketch shared methods
	// ---------------------------------------------------------------------------

	builtinRegistry["_translate"] = func(e *evaluator, args []value) (value, error) {
		const name = "_translate"
		switch r := args[0].(type) {
		case *manifold.Solid:
			if len(args) != 4 {
				return nil, fmt.Errorf("%s() expects 3 arguments, got %d", name, len(args)-1)
			}
			x, err := requireLength(name, 1, args[1])
			if err != nil {
				return nil, err
			}
			y, err := requireLength(name, 2, args[2])
			if err != nil {
				return nil, err
			}
			z, err := requireLength(name, 3, args[3])
			if err != nil {
				return nil, err
			}
			return r.Translate(x, y, z), nil
		case *manifold.Sketch:
			if len(args) != 3 {
				return nil, fmt.Errorf("%s() expects 2 arguments, got %d", name, len(args)-1)
			}
			x, err := requireLength(name, 1, args[1])
			if err != nil {
				return nil, err
			}
			y, err := requireLength(name, 2, args[2])
			if err != nil {
				return nil, err
			}
			return r.Translate(x, y), nil
		default:
			return nil, fmt.Errorf("%s: expected Solid or Sketch, got %s", name, typeName(args[0]))
		}
	}

	builtinRegistry["_rotate"] = func(e *evaluator, args []value) (value, error) {
		const name = "_rotate"
		switch r := args[0].(type) {
		case *manifold.Solid:
			if len(args) != 7 {
				return nil, fmt.Errorf("%s() expects 6 arguments (rx, ry, rz, ox, oy, oz), got %d", name, len(args)-1)
			}
			rx, err := requireAngle(name, 1, args[1])
			if err != nil {
				return nil, err
			}
			ry, err := requireAngle(name, 2, args[2])
			if err != nil {
				return nil, err
			}
			rz, err := requireAngle(name, 3, args[3])
			if err != nil {
				return nil, err
			}
			ox, err := requireLength(name, 4, args[4])
			if err != nil {
				return nil, err
			}
			oy, err := requireLength(name, 5, args[5])
			if err != nil {
				return nil, err
			}
			oz, err := requireLength(name, 6, args[6])
			if err != nil {
				return nil, err
			}
			return r.RotateAt(rx, ry, rz, ox, oy, oz), nil
		case *manifold.Sketch:
			if len(args) != 2 {
				return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args)-1)
			}
			deg, err := requireAngle(name, 1, args[1])
			if err != nil {
				return nil, err
			}
			return r.Rotate(deg), nil
		default:
			return nil, fmt.Errorf("%s: expected Solid or Sketch, got %s", name, typeName(args[0]))
		}
	}

	builtinRegistry["_scale"] = func(e *evaluator, args []value) (value, error) {
		const name = "_scale"
		switch r := args[0].(type) {
		case *manifold.Solid:
			if len(args) != 7 {
				return nil, fmt.Errorf("%s() expects 6 arguments (x, y, z, ox, oy, oz), got %d", name, len(args)-1)
			}
			x, err := requireNumber(name, 1, args[1])
			if err != nil {
				return nil, err
			}
			y, err := requireNumber(name, 2, args[2])
			if err != nil {
				return nil, err
			}
			z, err := requireNumber(name, 3, args[3])
			if err != nil {
				return nil, err
			}
			ox, err := requireLength(name, 4, args[4])
			if err != nil {
				return nil, err
			}
			oy, err := requireLength(name, 5, args[5])
			if err != nil {
				return nil, err
			}
			oz, err := requireLength(name, 6, args[6])
			if err != nil {
				return nil, err
			}
			return r.Scale(x, y, z, ox, oy, oz)
		case *manifold.Sketch:
			if len(args) != 5 {
				return nil, fmt.Errorf("%s() expects 4 arguments (x, y, px, py), got %d", name, len(args)-1)
			}
			x, err := requireNumber(name, 1, args[1])
			if err != nil {
				return nil, err
			}
			y, err := requireNumber(name, 2, args[2])
			if err != nil {
				return nil, err
			}
			px, err := requireLength(name, 3, args[3])
			if err != nil {
				return nil, err
			}
			py, err := requireLength(name, 4, args[4])
			if err != nil {
				return nil, err
			}
			return r.Scale(x, y, px, py)
		default:
			return nil, fmt.Errorf("%s: expected Solid or Sketch, got %s", name, typeName(args[0]))
		}
	}

	builtinRegistry["_mirror"] = func(e *evaluator, args []value) (value, error) {
		const name = "_mirror"
		switch r := args[0].(type) {
		case *manifold.Solid:
			if len(args) != 5 {
				return nil, fmt.Errorf("%s() expects 4 arguments (nx, ny, nz, offset), got %d", name, len(args)-1)
			}
			nx, err := requireNumber(name, 1, args[1])
			if err != nil {
				return nil, err
			}
			ny, err := requireNumber(name, 2, args[2])
			if err != nil {
				return nil, err
			}
			nz, err := requireNumber(name, 3, args[3])
			if err != nil {
				return nil, err
			}
			offset, err := requireLength(name, 4, args[4])
			if err != nil {
				return nil, err
			}
			return r.Mirror(nx, ny, nz, offset)
		case *manifold.Sketch:
			if len(args) != 4 {
				return nil, fmt.Errorf("%s() expects 3 arguments (ax, ay, offset), got %d", name, len(args)-1)
			}
			ax, err := requireNumber(name, 1, args[1])
			if err != nil {
				return nil, err
			}
			ay, err := requireNumber(name, 2, args[2])
			if err != nil {
				return nil, err
			}
			offset, err := requireLength(name, 3, args[3])
			if err != nil {
				return nil, err
			}
			return r.Mirror(ax, ay, offset)
		default:
			return nil, fmt.Errorf("%s: expected Solid or Sketch, got %s", name, typeName(args[0]))
		}
	}

	builtinRegistry["_bounding_box"] = func(e *evaluator, args []value) (value, error) {
		const name = "_bounding_box"
		sanitize := func(v float64) float64 {
			if math.IsInf(v, 0) || math.IsNaN(v) {
				return 0
			}
			return v
		}
		switch r := args[0].(type) {
		case *manifold.Solid:
			if len(args) != 1 {
				return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args)-1)
			}
			minX, minY, minZ, maxX, maxY, maxZ := r.BoundingBox()
			minX, minY, minZ = sanitize(minX), sanitize(minY), sanitize(minZ)
			maxX, maxY, maxZ = sanitize(maxX), sanitize(maxY), sanitize(maxZ)
			return &structVal{
				typeName: "Box",
				fields: map[string]value{
					"min": makePtVecStruct3("Vec3", minX, minY, minZ),
					"max": makePtVecStruct3("Vec3", maxX, maxY, maxZ),
				},
			}, nil
		case *manifold.Sketch:
			if len(args) != 1 {
				return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args)-1)
			}
			minX, minY, maxX, maxY := r.BoundingBox()
			minX, minY = sanitize(minX), sanitize(minY)
			maxX, maxY = sanitize(maxX), sanitize(maxY)
			return &structVal{
				typeName: "Box",
				fields: map[string]value{
					"min": makePtVecStruct3("Vec3", minX, minY, 0),
					"max": makePtVecStruct3("Vec3", maxX, maxY, 0),
				},
			}, nil
		default:
			return nil, fmt.Errorf("%s: expected Solid or Sketch, got %s", name, typeName(args[0]))
		}
	}

	builtinRegistry["_fillet"] = func(e *evaluator, args []value) (value, error) {
		const name = "_fillet"
		switch r := args[0].(type) {
		case *manifold.Sketch:
			if len(args) != 2 {
				return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args)-1)
			}
			radius, err := requireLength(name, 1, args[1])
			if err != nil {
				return nil, err
			}
			shrunk := r.Offset(-radius, 0)
			result := shrunk.Offset(radius, 0)
			return result, nil
		default:
			return nil, fmt.Errorf("%s: expected Sketch, got %s", name, typeName(args[0]))
		}
	}

	builtinRegistry["_chamfer"] = func(e *evaluator, args []value) (value, error) {
		const name = "_chamfer"
		switch r := args[0].(type) {
		case *manifold.Sketch:
			if len(args) != 2 {
				return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args)-1)
			}
			dist, err := requireLength(name, 1, args[1])
			if err != nil {
				return nil, err
			}
			shrunk := r.Offset(-dist, 1)
			result := shrunk.Offset(dist, 1)
			return result, nil
		default:
			return nil, fmt.Errorf("%s: expected Sketch, got %s", name, typeName(args[0]))
		}
	}

	// ---------------------------------------------------------------------------
	// Solid-only methods
	// ---------------------------------------------------------------------------

	builtinRegistry["_trim"] = func(e *evaluator, args []value) (value, error) {
		const name = "_trim"
		r, ok := args[0].(*manifold.Solid)
		if !ok {
			return nil, fmt.Errorf("%s: expected Solid, got %s", name, typeName(args[0]))
		}
		if len(args) != 5 {
			return nil, fmt.Errorf("%s() expects 4 arguments, got %d", name, len(args)-1)
		}
		nx, err := requireNumber(name, 1, args[1])
		if err != nil {
			return nil, err
		}
		ny, err := requireNumber(name, 2, args[2])
		if err != nil {
			return nil, err
		}
		nz, err := requireNumber(name, 3, args[3])
		if err != nil {
			return nil, err
		}
		offset, err := requireLength(name, 4, args[4])
		if err != nil {
			return nil, err
		}
		return r.TrimByPlane(nx, ny, nz, offset), nil
	}

	builtinRegistry["_smooth"] = func(e *evaluator, args []value) (value, error) {
		const name = "_smooth"
		r, ok := args[0].(*manifold.Solid)
		if !ok {
			return nil, fmt.Errorf("%s: expected Solid, got %s", name, typeName(args[0]))
		}
		if len(args) != 3 {
			return nil, fmt.Errorf("%s() expects 2 arguments, got %d", name, len(args)-1)
		}
		minSharpAngle, err := requireAngle(name, 1, args[1])
		if err != nil {
			return nil, err
		}
		minSmoothness, err := requireNumber(name, 2, args[2])
		if err != nil {
			return nil, err
		}
		return r.SmoothOut(minSharpAngle, minSmoothness), nil
	}

	builtinRegistry["_refine"] = func(e *evaluator, args []value) (value, error) {
		const name = "_refine"
		r, ok := args[0].(*manifold.Solid)
		if !ok {
			return nil, fmt.Errorf("%s: expected Solid, got %s", name, typeName(args[0]))
		}
		if len(args) != 2 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args)-1)
		}
		n, err := requireNumber(name, 1, args[1])
		if err != nil {
			return nil, err
		}
		return r.Refine(int(n)), nil
	}

	builtinRegistry["_simplify"] = func(e *evaluator, args []value) (value, error) {
		const name = "_simplify"
		r, ok := args[0].(*manifold.Solid)
		if !ok {
			return nil, fmt.Errorf("%s: expected Solid, got %s", name, typeName(args[0]))
		}
		if len(args) != 2 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args)-1)
		}
		tol, err := requireLength(name, 1, args[1])
		if err != nil {
			return nil, err
		}
		return r.Simplify(tol), nil
	}

	builtinRegistry["_refine_to_length"] = func(e *evaluator, args []value) (value, error) {
		const name = "_refine_to_length"
		r, ok := args[0].(*manifold.Solid)
		if !ok {
			return nil, fmt.Errorf("%s: expected Solid, got %s", name, typeName(args[0]))
		}
		if len(args) != 2 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args)-1)
		}
		l, err := requireLength(name, 1, args[1])
		if err != nil {
			return nil, err
		}
		return r.RefineToLength(l), nil
	}

	builtinRegistry["_genus"] = func(e *evaluator, args []value) (value, error) {
		const name = "_genus"
		r, ok := args[0].(*manifold.Solid)
		if !ok {
			return nil, fmt.Errorf("%s: expected Solid, got %s", name, typeName(args[0]))
		}
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args)-1)
		}
		return float64(r.Genus()), nil
	}

	builtinRegistry["_min_gap"] = func(e *evaluator, args []value) (value, error) {
		const name = "_min_gap"
		r, ok := args[0].(*manifold.Solid)
		if !ok {
			return nil, fmt.Errorf("%s: expected Solid, got %s", name, typeName(args[0]))
		}
		if len(args) != 3 {
			return nil, fmt.Errorf("%s() expects 2 arguments, got %d", name, len(args)-1)
		}
		other, ok := args[1].(*manifold.Solid)
		if !ok {
			return nil, fmt.Errorf("%s() expects Solid as first argument, got %s", name, typeName(args[1]))
		}
		searchLen, err := requireLength(name, 2, args[2])
		if err != nil {
			return nil, err
		}
		return length{mm: r.MinGap(other, searchLen)}, nil
	}

	builtinRegistry["_split"] = func(e *evaluator, args []value) (value, error) {
		const name = "_split"
		switch r := args[0].(type) {
		case string:
			if len(args) != 2 {
				return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args)-1)
			}
			delim, err := requireString(name, 1, args[1])
			if err != nil {
				return nil, err
			}
			parts := strings.Split(r, delim)
			elems := make([]value, len(parts))
			for i, p := range parts {
				elems[i] = p
			}
			return array{elems: elems, elemType: "String"}, nil
		case *manifold.Solid:
			if len(args) != 2 {
				return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args)-1)
			}
			cutter, ok := args[1].(*manifold.Solid)
			if !ok {
				return nil, fmt.Errorf("%s() expects Solid, got %s", name, typeName(args[1]))
			}
			pair := manifold.SplitSolid(r, cutter)
			return array{elems: []value{pair[0], pair[1]}, elemType: "Solid"}, nil
		default:
			return nil, fmt.Errorf("%s: expected String or Solid, got %s", name, typeName(args[0]))
		}
	}

	builtinRegistry["_split_plane"] = func(e *evaluator, args []value) (value, error) {
		const name = "_split_plane"
		r, ok := args[0].(*manifold.Solid)
		if !ok {
			return nil, fmt.Errorf("%s: expected Solid, got %s", name, typeName(args[0]))
		}
		if len(args) != 5 {
			return nil, fmt.Errorf("%s() expects 4 arguments, got %d", name, len(args)-1)
		}
		nx, err := requireNumber(name, 1, args[1])
		if err != nil {
			return nil, err
		}
		ny, err := requireNumber(name, 2, args[2])
		if err != nil {
			return nil, err
		}
		nz, err := requireNumber(name, 3, args[3])
		if err != nil {
			return nil, err
		}
		offset, err := requireLength(name, 4, args[4])
		if err != nil {
			return nil, err
		}
		pair := manifold.SplitSolidByPlane(r, nx, ny, nz, offset)
		return array{elems: []value{pair[0], pair[1]}, elemType: "Solid"}, nil
	}

	builtinRegistry["_volume"] = func(e *evaluator, args []value) (value, error) {
		const name = "_volume"
		r, ok := args[0].(*manifold.Solid)
		if !ok {
			return nil, fmt.Errorf("%s: expected Solid, got %s", name, typeName(args[0]))
		}
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args)-1)
		}
		return float64(r.Volume()), nil
	}

	builtinRegistry["_surface_area"] = func(e *evaluator, args []value) (value, error) {
		const name = "_surface_area"
		r, ok := args[0].(*manifold.Solid)
		if !ok {
			return nil, fmt.Errorf("%s: expected Solid, got %s", name, typeName(args[0]))
		}
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args)-1)
		}
		return float64(r.SurfaceArea()), nil
	}

	builtinRegistry["_slice"] = func(e *evaluator, args []value) (value, error) {
		const name = "_slice"
		r, ok := args[0].(*manifold.Solid)
		if !ok {
			return nil, fmt.Errorf("%s: expected Solid, got %s", name, typeName(args[0]))
		}
		if len(args) != 2 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args)-1)
		}
		height, err := requireLength(name, 1, args[1])
		if err != nil {
			return nil, err
		}
		return r.Slice(height), nil
	}

	builtinRegistry["_project"] = func(e *evaluator, args []value) (value, error) {
		const name = "_project"
		r, ok := args[0].(*manifold.Solid)
		if !ok {
			return nil, fmt.Errorf("%s: expected Solid, got %s", name, typeName(args[0]))
		}
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args)-1)
		}
		return r.Project(), nil
	}

	builtinRegistry["_warp"] = func(e *evaluator, args []value) (value, error) {
		const name = "_warp"
		r, ok := args[0].(*manifold.Solid)
		if !ok {
			return nil, fmt.Errorf("%s: expected Solid, got %s", name, typeName(args[0]))
		}
		if len(args) != 2 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args)-1)
		}
		fv, ok := args[1].(*functionVal)
		if !ok {
			return nil, fmt.Errorf("%s() expects Function, got %s", name, typeName(args[1]))
		}
		var warpErr error
		result := r.Warp(func(x, y, z float64) (float64, float64, float64) {
			if warpErr != nil {
				return x, y, z
			}
			pt := makePtVecStruct3("Vec3", x, y, z)
			res, callErr := e.callFunctionVal(fv, map[string]value{fv.params[0].Name: pt})
			if callErr != nil {
				warpErr = callErr
				return x, y, z
			}
			rx, ry, rz, ok := extractVec3(res)
			if !ok {
				warpErr = fmt.Errorf("Warp callback must return Vec3, got %s", typeName(res))
				return x, y, z
			}
			return rx, ry, rz
		})
		if warpErr != nil {
			return nil, warpErr
		}
		return result, nil
	}

	builtinRegistry["_color"] = func(e *evaluator, args []value) (value, error) {
		const name = "_color"
		r, ok := args[0].(*manifold.Solid)
		if !ok {
			return nil, fmt.Errorf("%s: expected Solid, got %s", name, typeName(args[0]))
		}
		if len(args) != 4 {
			return nil, fmt.Errorf("%s() expects 3 arguments (r, g, b), got %d", name, len(args)-1)
		}
		rv, err := requireNumber(name, 1, args[1])
		if err != nil {
			return nil, err
		}
		g, err := requireNumber(name, 2, args[2])
		if err != nil {
			return nil, err
		}
		b, err := requireNumber(name, 3, args[3])
		if err != nil {
			return nil, err
		}
		return r.SetColor(rv, g, b), nil
	}

	builtinRegistry["_color_hex"] = func(e *evaluator, args []value) (value, error) {
		const name = "_color_hex"
		r, ok := args[0].(*manifold.Solid)
		if !ok {
			return nil, fmt.Errorf("%s: expected Solid, got %s", name, typeName(args[0]))
		}
		if len(args) != 2 {
			return nil, fmt.Errorf("%s() expects 1 argument (hex string), got %d", name, len(args)-1)
		}
		hex, err := requireString(name, 1, args[1])
		if err != nil {
			return nil, err
		}
		rv, g, b, err := parseHexColor(hex)
		if err != nil {
			return nil, fmt.Errorf("%s(): %w", name, err)
		}
		return r.SetColor(rv, g, b), nil
	}

	builtinRegistry["_polymesh"] = func(e *evaluator, args []value) (value, error) {
		const name = "_polymesh"
		r, ok := args[0].(*manifold.Solid)
		if !ok {
			return nil, fmt.Errorf("%s: expected Solid, got %s", name, typeName(args[0]))
		}
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args)-1)
		}
		return polyMeshToStructVal(manifold.ExtractPolyMesh(r)), nil
	}

	builtinRegistry["_mesh"] = func(e *evaluator, args []value) (value, error) {
		const name = "_mesh"
		r, ok := args[0].(*manifold.Solid)
		if !ok {
			return nil, fmt.Errorf("%s: expected Solid, got %s", name, typeName(args[0]))
		}
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args)-1)
		}
		m := manifold.ExtractMeshShared(r)
		// Build vertices array of Vec3
		numVerts := len(m.Vertices) / 3
		verts := make([]value, numVerts)
		for i := 0; i < numVerts; i++ {
			verts[i] = makePtVecStruct3("Vec3",
				float64(m.Vertices[i*3+0]),
				float64(m.Vertices[i*3+1]),
				float64(m.Vertices[i*3+2]))
		}
		// Build indices array of Face structs
		numTris := len(m.Indices) / 3
		tris := make([]value, numTris)
		for i := 0; i < numTris; i++ {
			tris[i] = &structVal{
				typeName: "Face",
				fields: map[string]value{
					"v0": float64(m.Indices[i*3+0]),
					"v1": float64(m.Indices[i*3+1]),
					"v2": float64(m.Indices[i*3+2]),
				},
			}
		}
		return &structVal{
			typeName: "Mesh",
			fields: map[string]value{
				"vertices": array{elems: verts, elemType: "Vec3"},
				"indices":  array{elems: tris, elemType: "Face"},
			},
		}, nil
	}
}
