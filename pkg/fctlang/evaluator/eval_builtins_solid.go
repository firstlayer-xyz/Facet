package evaluator

import (
	"fmt"
	"math"
	"strings"

	"facet/pkg/manifold"
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
		// An EMPTY solid reports ±Inf bounds (Manifold's empty box) — map those
		// to 0 so an empty shape has zero extent. NaN is different: it means
		// corrupted geometry reached the kernel, and mapping it to 0 would mask
		// the corruption — error loudly instead.
		sanitize := func(v float64) (float64, error) {
			if math.IsNaN(v) {
				return 0, fmt.Errorf("%s: the shape's bounds are NaN (corrupted geometry)", name)
			}
			if math.IsInf(v, 0) {
				return 0, nil
			}
			return v, nil
		}
		sanitizeAll := func(vs ...float64) ([]float64, error) {
			out := make([]float64, len(vs))
			for i, v := range vs {
				s, err := sanitize(v)
				if err != nil {
					return nil, err
				}
				out[i] = s
			}
			return out, nil
		}
		switch r := args[0].(type) {
		case *manifold.Solid:
			if len(args) != 1 {
				return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args)-1)
			}
			minX, minY, minZ, maxX, maxY, maxZ := r.BoundingBox()
			b, err := sanitizeAll(minX, minY, minZ, maxX, maxY, maxZ)
			if err != nil {
				return nil, err
			}
			return &structVal{
				typeName: "Box",
				fields: map[string]value{
					"min": makePtVecStruct3("Vec3", b[0], b[1], b[2]),
					"max": makePtVecStruct3("Vec3", b[3], b[4], b[5]),
				},
			}, nil
		case *manifold.Sketch:
			if len(args) != 1 {
				return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args)-1)
			}
			minX, minY, maxX, maxY := r.BoundingBox()
			b, err := sanitizeAll(minX, minY, maxX, maxY)
			if err != nil {
				return nil, err
			}
			return &structVal{
				typeName: "Box",
				fields: map[string]value{
					"min": makePtVecStruct3("Vec3", b[0], b[1], 0),
					"max": makePtVecStruct3("Vec3", b[2], b[3], 0),
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

	builtinRegistry["_trim"] = solidMethod("_trim", func(r *manifold.Solid, args []value) (value, error) {
		const name = "_trim"
		if len(args) != 4 {
			return nil, fmt.Errorf("%s() expects 4 arguments, got %d", name, len(args))
		}
		nx, err := requireNumber(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		ny, err := requireNumber(name, 2, args[1])
		if err != nil {
			return nil, err
		}
		nz, err := requireNumber(name, 3, args[2])
		if err != nil {
			return nil, err
		}
		offset, err := requireLength(name, 4, args[3])
		if err != nil {
			return nil, err
		}
		return r.TrimByPlane(nx, ny, nz, offset), nil
	})

	builtinRegistry["_smooth"] = solidMethod("_smooth", func(r *manifold.Solid, args []value) (value, error) {
		const name = "_smooth"
		if len(args) != 2 {
			return nil, fmt.Errorf("%s() expects 2 arguments, got %d", name, len(args))
		}
		minSharpAngle, err := requireAngle(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		minSmoothness, err := requireNumber(name, 2, args[1])
		if err != nil {
			return nil, err
		}
		return r.SmoothOut(minSharpAngle, minSmoothness), nil
	})

	builtinRegistry["_refine"] = solidMethod("_refine", func(r *manifold.Solid, args []value) (value, error) {
		const name = "_refine"
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args))
		}
		n, err := requireNumber(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		count, err := requireCount(name, 1, n, maxRefine)
		if err != nil {
			return nil, err
		}
		return r.Refine(count), nil
	})

	builtinRegistry["_simplify"] = solidMethod("_simplify", func(r *manifold.Solid, args []value) (value, error) {
		const name = "_simplify"
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args))
		}
		tol, err := requireLength(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		return r.Simplify(tol), nil
	})

	builtinRegistry["_refine_to_length"] = solidMethod("_refine_to_length", func(r *manifold.Solid, args []value) (value, error) {
		const name = "_refine_to_length"
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args))
		}
		l, err := requireLength(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		return r.RefineToLength(l), nil
	})

	builtinRegistry["_genus"] = solidMethod("_genus", func(r *manifold.Solid, args []value) (value, error) {
		const name = "_genus"
		if len(args) != 0 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args))
		}
		return float64(r.Genus()), nil
	})

	builtinRegistry["_min_gap"] = solidMethod("_min_gap", func(r *manifold.Solid, args []value) (value, error) {
		const name = "_min_gap"
		if len(args) != 2 {
			return nil, fmt.Errorf("%s() expects 2 arguments, got %d", name, len(args))
		}
		other, ok := args[0].(*manifold.Solid)
		if !ok {
			return nil, fmt.Errorf("%s() expects Solid as first argument, got %s", name, typeName(args[0]))
		}
		searchLen, err := requireLength(name, 2, args[1])
		if err != nil {
			return nil, err
		}
		return length{mm: r.MinGap(other, searchLen)}, nil
	})

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

	builtinRegistry["_split_plane"] = solidMethod("_split_plane", func(r *manifold.Solid, args []value) (value, error) {
		const name = "_split_plane"
		if len(args) != 4 {
			return nil, fmt.Errorf("%s() expects 4 arguments, got %d", name, len(args))
		}
		nx, err := requireNumber(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		ny, err := requireNumber(name, 2, args[1])
		if err != nil {
			return nil, err
		}
		nz, err := requireNumber(name, 3, args[2])
		if err != nil {
			return nil, err
		}
		offset, err := requireLength(name, 4, args[3])
		if err != nil {
			return nil, err
		}
		pair := manifold.SplitSolidByPlane(r, nx, ny, nz, offset)
		return array{elems: []value{pair[0], pair[1]}, elemType: "Solid"}, nil
	})

	builtinRegistry["_volume"] = solidMethod("_volume", func(r *manifold.Solid, args []value) (value, error) {
		const name = "_volume"
		if len(args) != 0 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args))
		}
		return float64(r.Volume()), nil
	})

	builtinRegistry["_surface_area"] = solidMethod("_surface_area", func(r *manifold.Solid, args []value) (value, error) {
		const name = "_surface_area"
		if len(args) != 0 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args))
		}
		return float64(r.SurfaceArea()), nil
	})

	builtinRegistry["_slice"] = solidMethod("_slice", func(r *manifold.Solid, args []value) (value, error) {
		const name = "_slice"
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args))
		}
		height, err := requireLength(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		return r.Slice(height), nil
	})

	builtinRegistry["_project"] = solidMethod("_project", func(r *manifold.Solid, args []value) (value, error) {
		const name = "_project"
		if len(args) != 0 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args))
		}
		return r.Project(), nil
	})

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

	builtinRegistry["_color"] = solidMethod("_color", func(r *manifold.Solid, args []value) (value, error) {
		const name = "_color"
		if len(args) != 4 {
			return nil, fmt.Errorf("%s() expects 4 arguments (r, g, b, a), got %d", name, len(args))
		}
		rv, err := requireNumber(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		g, err := requireNumber(name, 2, args[1])
		if err != nil {
			return nil, err
		}
		b, err := requireNumber(name, 3, args[2])
		if err != nil {
			return nil, err
		}
		a, err := requireNumber(name, 4, args[3])
		if err != nil {
			return nil, err
		}
		return r.SetColor(rv, g, b, a), nil
	})

	builtinRegistry["_color_hex"] = solidMethod("_color_hex", func(r *manifold.Solid, args []value) (value, error) {
		const name = "_color_hex"
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 1 argument (hex string), got %d", name, len(args))
		}
		hex, err := requireString(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		rv, g, b, a, err := parseHexColorRGBA(hex)
		if err != nil {
			return nil, fmt.Errorf("%s(): %w", name, err)
		}
		return r.SetColor(rv, g, b, a), nil
	})

	builtinRegistry["_polymesh"] = solidMethod("_polymesh", func(r *manifold.Solid, args []value) (value, error) {
		const name = "_polymesh"
		if len(args) != 0 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args))
		}
		return polyMeshToStructVal(manifold.ExtractPolyMesh(r)), nil
	})

	builtinRegistry["_mesh"] = solidMethod("_mesh", func(r *manifold.Solid, args []value) (value, error) {
		const name = "_mesh"
		if len(args) != 0 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args))
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
	})
}
