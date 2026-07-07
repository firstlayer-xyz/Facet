package evaluator

import (
	"facet/pkg/manifold"
	"fmt"
)

func init() {
	// Zero-arg PolyMesh → PolyMesh Conway operators.
	// Each entry names a builtin and the corresponding PolyMesh method.
	for _, m := range []struct {
		name string
		op   func(*manifold.PolyMesh) *manifold.PolyMesh
	}{
		{"_dual", (*manifold.PolyMesh).Dual},
		{"_ambo", (*manifold.PolyMesh).Ambo},
		{"_kis", (*manifold.PolyMesh).Kis},
		{"_truncate", (*manifold.PolyMesh).Truncate},
		{"_expand", (*manifold.PolyMesh).Expand},
		{"_snub", (*manifold.PolyMesh).Snub},
	} {
		op := m.op // capture for closure
		name := m.name
		builtinRegistry[name] = structMethod(name, func(sv *structVal, args []value) (value, error) {
			if len(args) != 0 {
				return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args))
			}
			pm, err := structValToPolyMesh(sv)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			return polyMeshToStructVal(op(pm)), nil
		})
	}

	// PolyMesh methods with arguments or different return types.
	for _, m := range []struct {
		name string
		fn   func(*structVal, []value) (value, error)
	}{
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
				// maxIter feeds the kernel's canonicalization loop, which does not
				// clamp — bound it like every other count arg so a huge value can't
				// hang the host.
				maxIter, err = requireCount(name, 1, n, maxRefine)
				if err != nil {
					return nil, err
				}
			}
			if len(args) > 1 {
				n, err := requireNumber(name, 2, args[1])
				if err != nil {
					return nil, err
				}
				if n <= 0 {
					return nil, fmt.Errorf("%s() tolerance must be positive, got %v", name, n)
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
		builtinRegistry[m.name] = structMethod(m.name, m.fn)
	}

	// Mesh methods

	builtinRegistry["_face_normals"] = structMethod("_face_normals", func(sv *structVal, args []value) (value, error) {
		return meshFaceNormals(sv, args)
	})
	builtinRegistry["_vertex_normals"] = structMethod("_vertex_normals", func(sv *structVal, args []value) (value, error) {
		return meshVertexNormals(sv, args)
	})

	// Color methods

	builtinRegistry["_color_to_hex"] = structMethod("_color_to_hex", func(sv *structVal, args []value) (value, error) {
		if len(args) != 0 {
			return nil, fmt.Errorf("_color_to_hex() expects 0 arguments, got %d", len(args))
		}
		r, _ := sv.fields["r"].(float64)
		g, _ := sv.fields["g"].(float64)
		b, _ := sv.fields["b"].(float64)
		return fmt.Sprintf("#%02X%02X%02X", clampByte(r), clampByte(g), clampByte(b)), nil
	})
}
