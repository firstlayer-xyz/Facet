package evaluator

import (
	"fmt"
	"math"
	"regexp"
	"strings"

	"facet/app/pkg/manifold"
)

func solidMethod(e *evaluator, sf *manifold.SolidFuture, method string, args []value) (value, error) {
	name := "Solid." + method
	switch method {
	case "_translate":
		if len(args) != 3 {
			return nil, fmt.Errorf("%s() expects 3 arguments, got %d", name, len(args))
		}
		x, err := requireLength(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		y, err := requireLength(name, 2, args[1])
		if err != nil {
			return nil, err
		}
		z, err := requireLength(name, 3, args[2])
		if err != nil {
			return nil, err
		}
		return sf.Translate(x, y, z), nil

	case "_rotate":
		if len(args) != 6 {
			return nil, fmt.Errorf("%s() expects 6 arguments (rx, ry, rz, ox, oy, oz), got %d", name, len(args))
		}
		rx, err := requireAngle(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		ry, err := requireAngle(name, 2, args[1])
		if err != nil {
			return nil, err
		}
		rz, err := requireAngle(name, 3, args[2])
		if err != nil {
			return nil, err
		}
		ox, err := requireLength(name, 4, args[3])
		if err != nil {
			return nil, err
		}
		oy, err := requireLength(name, 5, args[4])
		if err != nil {
			return nil, err
		}
		oz, err := requireLength(name, 6, args[5])
		if err != nil {
			return nil, err
		}
		return sf.RotateAt(rx, ry, rz, ox, oy, oz), nil

	case "_scale":
		if len(args) != 6 {
			return nil, fmt.Errorf("%s() expects 6 arguments (x, y, z, ox, oy, oz), got %d", name, len(args))
		}
		x, err := requireNumber(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		y, err := requireNumber(name, 2, args[1])
		if err != nil {
			return nil, err
		}
		z, err := requireNumber(name, 3, args[2])
		if err != nil {
			return nil, err
		}
		ox, err := requireLength(name, 4, args[3])
		if err != nil {
			return nil, err
		}
		oy, err := requireLength(name, 5, args[4])
		if err != nil {
			return nil, err
		}
		oz, err := requireLength(name, 6, args[5])
		if err != nil {
			return nil, err
		}
		return sf.Scale(x, y, z, ox, oy, oz), nil

	case "_mirror":
		if len(args) != 4 {
			return nil, fmt.Errorf("%s() expects 4 arguments (nx, ny, nz, offset), got %d", name, len(args))
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
		return sf.Mirror(nx, ny, nz, offset), nil

	case "_trim":
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
		return sf.TrimByPlane(nx, ny, nz, offset), nil

	case "_smooth":
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
		return sf.SmoothOut(minSharpAngle, minSmoothness), nil

	case "_refine":
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args))
		}
		n, err := requireNumber(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		return sf.Refine(int(n)), nil

	case "_simplify":
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args))
		}
		tol, err := requireLength(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		return sf.Simplify(tol), nil

	case "_refine_to_length":
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args))
		}
		l, err := requireLength(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		return sf.RefineToLength(l), nil

	case "_genus":
		if len(args) != 0 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args))
		}
		s, err := sf.Resolve()
		if err != nil {
			return nil, err
		}
		return float64(s.Genus()), nil

	case "_min_gap":
		if len(args) != 2 {
			return nil, fmt.Errorf("%s() expects 2 arguments, got %d", name, len(args))
		}
		other, ok := args[0].(*manifold.SolidFuture)
		if !ok {
			return nil, fmt.Errorf("%s() expects Solid as first argument, got %s", name, typeName(args[0]))
		}
		searchLen, err := requireLength(name, 2, args[1])
		if err != nil {
			return nil, err
		}
		sa, err := sf.Resolve()
		if err != nil {
			return nil, err
		}
		sb, err := other.Resolve()
		if err != nil {
			return nil, err
		}
		return length{mm: sa.MinGap(sb, searchLen)}, nil

	case "_split":
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args))
		}
		cutter, ok := args[0].(*manifold.SolidFuture)
		if !ok {
			return nil, fmt.Errorf("%s() expects Solid, got %s", name, typeName(args[0]))
		}
		futures, err := manifold.SplitSolid(sf, cutter)
		if err != nil {
			return nil, err
		}
		elems := make([]value, len(futures))
		for i, f := range futures {
			elems[i] = f
		}
		return array{elems: elems, elemType: "Solid"}, nil

	case "_split_plane":
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
		futures, err := manifold.SplitSolidByPlane(sf, nx, ny, nz, offset)
		if err != nil {
			return nil, err
		}
		elems := make([]value, len(futures))
		for i, f := range futures {
			elems[i] = f
		}
		return array{elems: elems, elemType: "Solid"}, nil

	case "_volume":
		if len(args) != 0 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args))
		}
		s, err := sf.Resolve()
		if err != nil {
			return nil, err
		}
		return float64(s.Volume()), nil

	case "_surface_area":
		if len(args) != 0 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args))
		}
		s, err := sf.Resolve()
		if err != nil {
			return nil, err
		}
		return float64(s.SurfaceArea()), nil

	case "_slice":
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args))
		}
		height, err := requireLength(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		return sf.Slice(height), nil

	case "_project":
		if len(args) != 0 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args))
		}
		return sf.Project(), nil

	case "_bounding_box":
		if len(args) != 0 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args))
		}
		s, err := sf.Resolve()
		if err != nil {
			return nil, err
		}
		minX, minY, minZ, maxX, maxY, maxZ := s.BoundingBox()
		// Manifold returns ±Inf for empty solids; clamp to 0 to keep JSON serializable.
		sanitize := func(v float64) float64 {
			if math.IsInf(v, 0) || math.IsNaN(v) {
				return 0
			}
			return v
		}
		minX, minY, minZ = sanitize(minX), sanitize(minY), sanitize(minZ)
		maxX, maxY, maxZ = sanitize(maxX), sanitize(maxY), sanitize(maxZ)
		return &structVal{
			typeName: "Box",
			fields: map[string]value{
				"min": makePtVecStruct3("Vec3", minX, minY, minZ),
				"max": makePtVecStruct3("Vec3", maxX, maxY, maxZ),
			},
		}, nil

	case "_warp":
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args))
		}
		fv, ok := args[0].(*functionVal)
		if !ok {
			return nil, fmt.Errorf("%s() expects parser.Function, got %s", name, typeName(args[0]))
		}
		var warpErr error
		result := sf.Warp(func(x, y, z float64) (float64, float64, float64) {
			if warpErr != nil {
				return x, y, z
			}
			pt := makePtVecStruct3("Vec3", x, y, z)
			result, callErr := e.callFunctionVal(fv, []value{pt})
			if callErr != nil {
				warpErr = callErr
				return x, y, z
			}
			rx, ry, rz, ok := extractVec3(result)
			if !ok {
				warpErr = fmt.Errorf("Warp callback must return Vec3, got %s", typeName(result))
				return x, y, z
			}
			return rx, ry, rz
		})
		if warpErr != nil {
			return nil, warpErr
		}
		return result, nil

	case "_color":
		if len(args) != 3 {
			return nil, fmt.Errorf("%s() expects 3 arguments (r, g, b), got %d", name, len(args))
		}
		r, err := requireNumber(name, 1, args[0])
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
		return sf.SetColor(r, g, b), nil

	case "_color_hex":
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 1 argument (hex string), got %d", name, len(args))
		}
		hex, err := requireString(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		r, g, b, err := parseHexColor(hex)
		if err != nil {
			return nil, fmt.Errorf("%s(): %w", name, err)
		}
		return sf.SetColor(r, g, b), nil

	case "_polymesh":
		if len(args) != 0 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args))
		}
		s, err := sf.Resolve()
		if err != nil {
			return nil, err
		}
		return polyMeshToStructVal(manifold.ExtractPolyMesh(s)), nil

	case "_mesh":
		if len(args) != 0 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args))
		}
		s, err := sf.Resolve()
		if err != nil {
			return nil, err
		}
		m := manifold.ExtractMeshShared(s)
		// Build vertices array of Vec3
		numVerts := len(m.Vertices) / 3
		verts := make([]value, numVerts)
		for i := 0; i < numVerts; i++ {
			verts[i] = makePtVecStruct3("Vec3",
				float64(m.Vertices[i*3+0]),
				float64(m.Vertices[i*3+1]),
				float64(m.Vertices[i*3+2]))
		}
		// Build indices array of Tri structs
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

	default:
		return nil, fmt.Errorf("Solid has no method %q", method)
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

func polyMeshBuiltinMethod(sv *structVal, method string, args []value) (value, error) {
	name := "PolyMesh." + method
	// Convert structVal → Go PolyMesh for operations that need it
	pm, err := structValToPolyMesh(sv)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	switch method {
	case "_dual":
		if len(args) != 0 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args))
		}
		return polyMeshToStructVal(pm.Dual()), nil
	case "_ambo":
		if len(args) != 0 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args))
		}
		return polyMeshToStructVal(pm.Ambo()), nil
	case "_kis":
		if len(args) != 0 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args))
		}
		return polyMeshToStructVal(pm.Kis()), nil
	case "_truncate":
		if len(args) != 0 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args))
		}
		return polyMeshToStructVal(pm.Truncate()), nil
	case "_expand":
		if len(args) != 0 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args))
		}
		return polyMeshToStructVal(pm.Expand()), nil
	case "_snub":
		if len(args) != 0 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args))
		}
		return polyMeshToStructVal(pm.Snub()), nil
	case "_canonicalize":
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
	case "_scale_to_radius":
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args))
		}
		radius, err := requireLength(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		return polyMeshToStructVal(pm.ScaleToRadius(radius)), nil
	case "_scale_uniform":
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args))
		}
		factor, err := requireNumber(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		return polyMeshToStructVal(pm.ScaleUniform(factor)), nil
	case "_solid":
		if len(args) != 0 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args))
		}
		sf, err := pm.ToSolid()
		if err != nil {
			return nil, err
		}
		return sf, nil
	case "_display_mesh":
		if len(args) != 0 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args))
		}
		return pm.ToDisplayMesh(), nil
	default:
		return nil, fmt.Errorf("no builtin method %q on PolyMesh", method)
	}
}

func structBuiltinMethod(sv *structVal, method string, args []value) (value, error) {
	switch sv.typeName {
	case "Mesh":
		return meshBuiltinMethod(sv, method, args)
	case "PolyMesh":
		return polyMeshBuiltinMethod(sv, method, args)
	case "Color":
		return colorBuiltinMethod(sv, method, args)
	}
	return nil, fmt.Errorf("no builtin method %s on struct %s", method, sv.typeName)
}

func meshBuiltinMethod(sv *structVal, method string, args []value) (value, error) {
	name := "Mesh." + method
	switch method {
	case "_face_normals":
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

	case "_vertex_normals":
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

	default:
		return nil, fmt.Errorf("no builtin method %s on struct Mesh", method)
	}
}

func sketchMethod(pf *manifold.SketchFuture, method string, args []value) (value, error) {
	name := "Sketch." + method
	switch method {
	case "_translate":
		if len(args) != 2 {
			return nil, fmt.Errorf("%s() expects 2 arguments, got %d", name, len(args))
		}
		x, err := requireLength(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		y, err := requireLength(name, 2, args[1])
		if err != nil {
			return nil, err
		}
		return pf.Translate(x, y), nil

	case "_rotate":
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args))
		}
		deg, err := requireAngle(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		return pf.Rotate(deg), nil

	case "_rotate_origin":
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args))
		}
		deg, err := requireAngle(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		return pf.RotateOrigin(deg), nil

	case "_scale":
		if len(args) != 4 {
			return nil, fmt.Errorf("%s() expects 4 arguments (x, y, px, py), got %d", name, len(args))
		}
		x, err := requireNumber(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		y, err := requireNumber(name, 2, args[1])
		if err != nil {
			return nil, err
		}
		px, err := requireLength(name, 3, args[2])
		if err != nil {
			return nil, err
		}
		py, err := requireLength(name, 4, args[3])
		if err != nil {
			return nil, err
		}
		return pf.Scale(x, y, px, py), nil

	case "_mirror":
		if len(args) != 3 {
			return nil, fmt.Errorf("%s() expects 3 arguments (ax, ay, offset), got %d", name, len(args))
		}
		ax, err := requireNumber(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		ay, err := requireNumber(name, 2, args[1])
		if err != nil {
			return nil, err
		}
		offset, err := requireLength(name, 3, args[2])
		if err != nil {
			return nil, err
		}
		return pf.Mirror(ax, ay, offset), nil

	case "_offset":
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args))
		}
		delta, err := requireLength(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		return pf.Offset(delta, 0), nil

	case "_fillet":
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args))
		}
		radius, err := requireLength(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		shrunk := pf.Offset(-radius, 0)
		result := shrunk.Offset(radius, 0)
		return result, nil

	case "_chamfer":
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args))
		}
		dist, err := requireLength(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		shrunk := pf.Offset(-dist, 1)
		result := shrunk.Offset(dist, 1)
		return result, nil

	case "_bounding_box":
		if len(args) != 0 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args))
		}
		p, err := pf.Resolve()
		if err != nil {
			return nil, err
		}
		minX, minY, maxX, maxY := p.BoundingBox()
		sanitize := func(v float64) float64 {
			if math.IsInf(v, 0) || math.IsNaN(v) {
				return 0
			}
			return v
		}
		minX, minY = sanitize(minX), sanitize(minY)
		maxX, maxY = sanitize(maxX), sanitize(maxY)
		return &structVal{
			typeName: "Box",
			fields: map[string]value{
				"min": makePtVecStruct3("Vec3", minX, minY, 0),
				"max": makePtVecStruct3("Vec3", maxX, maxY, 0),
			},
		}, nil

	case "_area":
		if len(args) != 0 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args))
		}
		p, err := pf.Resolve()
		if err != nil {
			return nil, err
		}
		return float64(p.Area()), nil

	case "_extrude":
		if len(args) != 1 && len(args) != 5 {
			return nil, fmt.Errorf("%s() expects 1 or 5 arguments, got %d", name, len(args))
		}
		height, err := requireLength(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		if len(args) == 1 {
			return pf.Extrude(height, 0, 0, 1, 1), nil
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
		return pf.Extrude(height, int(slices), twist, scaleX, scaleY), nil

	case "_revolve":
		if len(args) > 1 {
			return nil, fmt.Errorf("%s() expects 0 or 1 arguments, got %d", name, len(args))
		}
		degrees := 360.0
		if len(args) == 1 {
			var err error
			degrees, err = requireAngle(name, 1, args[0])
			if err != nil {
				return nil, err
			}
		}
		return pf.Revolve(0, degrees), nil

	case "_sweep":
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
		return pf.Sweep(path), nil

	default:
		return nil, fmt.Errorf("Sketch has no method %q", method)
	}
}

func stringMethod(s string, method string, args []value) (value, error) {
	name := "String." + method
	switch method {
	case "_sub_str":
		if len(args) != 2 {
			return nil, fmt.Errorf("%s() expects 2 arguments, got %d", name, len(args))
		}
		start, err := requireNumber(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		length, err := requireNumber(name, 2, args[1])
		if err != nil {
			return nil, err
		}
		runes := []rune(s)
		si := int(start)
		li := int(length)
		if si < 0 {
			si = 0
		}
		if si > len(runes) {
			return "", nil
		}
		if li < 0 {
			return "", nil
		}
		end := si + li
		if end > len(runes) {
			end = len(runes)
		}
		return string(runes[si:end]), nil

	case "_has_prefix":
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args))
		}
		prefix, err := requireString(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		return strings.HasPrefix(s, prefix), nil

	case "_has_suffix":
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args))
		}
		suffix, err := requireString(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		return strings.HasSuffix(s, suffix), nil

	case "_split":
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args))
		}
		delim, err := requireString(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		parts := strings.Split(s, delim)
		elems := make([]value, len(parts))
		for i, p := range parts {
			elems[i] = p
		}
		return array{elems: elems, elemType: "String"}, nil

	case "_match":
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args))
		}
		pattern, err := requireString(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("%s(): invalid regex: %v", name, err)
		}
		matches := re.FindStringSubmatch(s)
		if matches == nil {
			return array{elems: []value{}, elemType: "String"}, nil
		}
		if len(matches) > 255 {
			return nil, fmt.Errorf("%s(): too many submatches (max 255)", name)
		}
		elems := make([]value, len(matches))
		for i, m := range matches {
			elems[i] = m
		}
		return array{elems: elems, elemType: "String"}, nil

	case "_to_upper":
		if len(args) != 0 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args))
		}
		return strings.ToUpper(s), nil

	case "_to_lower":
		if len(args) != 0 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args))
		}
		return strings.ToLower(s), nil

	case "_trim_str":
		if len(args) != 0 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args))
		}
		return strings.TrimSpace(s), nil

	case "_replace":
		if len(args) != 2 {
			return nil, fmt.Errorf("%s() expects 2 arguments, got %d", name, len(args))
		}
		old, err := requireString(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		newStr, err := requireString(name, 2, args[1])
		if err != nil {
			return nil, err
		}
		return strings.ReplaceAll(s, old, newStr), nil

	case "_index_of":
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args))
		}
		substr, err := requireString(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		return float64(strings.Index(s, substr)), nil

	case "_contains":
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args))
		}
		substr, err := requireString(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		return strings.Contains(s, substr), nil

	case "_length":
		if len(args) != 0 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args))
		}
		return float64(len([]rune(s))), nil

	default:
		return nil, fmt.Errorf("String has no method %q", method)
	}
}

func colorBuiltinMethod(sv *structVal, method string, args []value) (value, error) {
	name := "Color." + method
	switch method {
	case "_color_to_hex":
		if len(args) != 0 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args))
		}
		r, _ := sv.fields["r"].(float64)
		g, _ := sv.fields["g"].(float64)
		b, _ := sv.fields["b"].(float64)
		ri := clampByte(r)
		gi := clampByte(g)
		bi := clampByte(b)
		return fmt.Sprintf("#%02X%02X%02X", ri, gi, bi), nil
	}
	return nil, fmt.Errorf("no builtin method %s on struct Color", method)
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
