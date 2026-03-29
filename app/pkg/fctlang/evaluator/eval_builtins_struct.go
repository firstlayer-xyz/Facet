package evaluator

import "fmt"

func init() {
	// PolyMesh methods

	polyMeshMethod := func(name string, body func(*structVal, []value) (value, error)) builtinFn {
		return func(e *evaluator, args []value) (value, error) {
			sv, ok := args[0].(*structVal)
			if !ok {
				return nil, fmt.Errorf("%s: expected struct, got %s", name, typeName(args[0]))
			}
			return body(sv, args[1:])
		}
	}

	for _, m := range []struct {
		name string
		fn   func(*structVal, []value) (value, error)
	}{
		{"_dual", func(sv *structVal, args []value) (value, error) {
			if len(args) != 0 {
				return nil, fmt.Errorf("_dual() expects 0 arguments, got %d", len(args))
			}
			pm, err := structValToPolyMesh(sv)
			if err != nil {
				return nil, fmt.Errorf("_dual: %w", err)
			}
			return polyMeshToStructVal(pm.Dual()), nil
		}},
		{"_ambo", func(sv *structVal, args []value) (value, error) {
			if len(args) != 0 {
				return nil, fmt.Errorf("_ambo() expects 0 arguments, got %d", len(args))
			}
			pm, err := structValToPolyMesh(sv)
			if err != nil {
				return nil, fmt.Errorf("_ambo: %w", err)
			}
			return polyMeshToStructVal(pm.Ambo()), nil
		}},
		{"_kis", func(sv *structVal, args []value) (value, error) {
			if len(args) != 0 {
				return nil, fmt.Errorf("_kis() expects 0 arguments, got %d", len(args))
			}
			pm, err := structValToPolyMesh(sv)
			if err != nil {
				return nil, fmt.Errorf("_kis: %w", err)
			}
			return polyMeshToStructVal(pm.Kis()), nil
		}},
		{"_truncate", func(sv *structVal, args []value) (value, error) {
			if len(args) != 0 {
				return nil, fmt.Errorf("_truncate() expects 0 arguments, got %d", len(args))
			}
			pm, err := structValToPolyMesh(sv)
			if err != nil {
				return nil, fmt.Errorf("_truncate: %w", err)
			}
			return polyMeshToStructVal(pm.Truncate()), nil
		}},
		{"_expand", func(sv *structVal, args []value) (value, error) {
			if len(args) != 0 {
				return nil, fmt.Errorf("_expand() expects 0 arguments, got %d", len(args))
			}
			pm, err := structValToPolyMesh(sv)
			if err != nil {
				return nil, fmt.Errorf("_expand: %w", err)
			}
			return polyMeshToStructVal(pm.Expand()), nil
		}},
		{"_snub", func(sv *structVal, args []value) (value, error) {
			if len(args) != 0 {
				return nil, fmt.Errorf("_snub() expects 0 arguments, got %d", len(args))
			}
			pm, err := structValToPolyMesh(sv)
			if err != nil {
				return nil, fmt.Errorf("_snub: %w", err)
			}
			return polyMeshToStructVal(pm.Snub()), nil
		}},
		{"_canonicalize", func(sv *structVal, args []value) (value, error) {
			const name = "_canonicalize"
			pm, err := structValToPolyMesh(sv)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			maxIter := 200
			tol := 1e-6
			if len(args) > 0 {
				n, err := requireNumber(name, 1, args[0])
				if err != nil {
					return nil, err
				}
				maxIter = int(n)
			}
			if len(args) > 1 {
				n, err := requireNumber(name, 2, args[1])
				if err != nil {
					return nil, err
				}
				tol = n
			}
			return polyMeshToStructVal(pm.Canonicalize(maxIter, tol)), nil
		}},
		{"_scale_to_radius", func(sv *structVal, args []value) (value, error) {
			const name = "_scale_to_radius"
			if len(args) != 1 {
				return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args))
			}
			pm, err := structValToPolyMesh(sv)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			radius, err := requireLength(name, 1, args[0])
			if err != nil {
				return nil, err
			}
			return polyMeshToStructVal(pm.ScaleToRadius(radius)), nil
		}},
		{"_scale_uniform", func(sv *structVal, args []value) (value, error) {
			const name = "_scale_uniform"
			if len(args) != 1 {
				return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args))
			}
			pm, err := structValToPolyMesh(sv)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			factor, err := requireNumber(name, 1, args[0])
			if err != nil {
				return nil, err
			}
			return polyMeshToStructVal(pm.ScaleUniform(factor)), nil
		}},
		{"_solid", func(sv *structVal, args []value) (value, error) {
			const name = "_solid"
			if len(args) != 0 {
				return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args))
			}
			pm, err := structValToPolyMesh(sv)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			sf, err := pm.ToSolid()
			if err != nil {
				return nil, err
			}
			return sf, nil
		}},
		{"_display_mesh", func(sv *structVal, args []value) (value, error) {
			const name = "_display_mesh"
			if len(args) != 0 {
				return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args))
			}
			pm, err := structValToPolyMesh(sv)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			return pm.ToDisplayMesh(), nil
		}},
	} {
		builtinRegistry[m.name] = polyMeshMethod(m.name, m.fn)
	}

	// Mesh methods

	meshMethod := func(name string, body func(*structVal, []value) (value, error)) builtinFn {
		return func(e *evaluator, args []value) (value, error) {
			sv, ok := args[0].(*structVal)
			if !ok {
				return nil, fmt.Errorf("%s: expected struct, got %s", name, typeName(args[0]))
			}
			return body(sv, args[1:])
		}
	}

	builtinRegistry["_face_normals"] = meshMethod("_face_normals", func(sv *structVal, args []value) (value, error) {
		return meshFaceNormals(sv, args)
	})
	builtinRegistry["_vertex_normals"] = meshMethod("_vertex_normals", func(sv *structVal, args []value) (value, error) {
		return meshVertexNormals(sv, args)
	})

	// Color methods

	builtinRegistry["_color_to_hex"] = func(e *evaluator, args []value) (value, error) {
		const name = "_color_to_hex"
		sv, ok := args[0].(*structVal)
		if !ok {
			return nil, fmt.Errorf("%s: expected struct, got %s", name, typeName(args[0]))
		}
		if len(args[1:]) != 0 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args[1:]))
		}
		r, _ := sv.fields["r"].(float64)
		g, _ := sv.fields["g"].(float64)
		b, _ := sv.fields["b"].(float64)
		ri := clampByte(r)
		gi := clampByte(g)
		bi := clampByte(b)
		return fmt.Sprintf("#%02X%02X%02X", ri, gi, bi), nil
	}
}
