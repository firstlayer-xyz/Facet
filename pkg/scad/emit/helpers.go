package emit

// Emitted helper functions bridge the []Number vector model to Facet's typed
// geometry boundary. They are `scad_`-prefixed to avoid colliding with user
// names and are emitted only when referenced (see helperPreamble).

// scadV2Helper converts a runtime list of 2-element number lists into the
// []Vec2 a Sketch primitive expects, treating each coordinate as millimetres
// (OpenSCAD's unit). It is needed when point data is held in a variable rather
// than a literal that the emitter can convert element-by-element.
const scadV2Helper = `fn scad_v2(ps [][]Number) []Vec2 {
	return for p ps {
		yield Vec2{x: p[0] * 1 mm, y: p[1] * 1 mm}
	}
}`

// scadV3Helper is the 3D counterpart of scad_v2, for polyhedron vertices.
const scadV3Helper = `fn scad_v3(ps [][]Number) []Vec3 {
	return for p ps {
		yield Vec3{x: p[0] * 1 mm, y: p[1] * 1 mm, z: p[2] * 1 mm}
	}
}`

// scadFacesHelper converts OpenSCAD polyhedron faces (each a list of vertex
// indices, possibly an n-gon) into the triangle []Face a Mesh requires, by
// fan-triangulating each face: indices [v0,v1,…,vn] become triangles
// (v0,v1,v2), (v0,v2,v3), …. The per-face triangle lists are concatenated.
const scadFacesHelper = `fn scad_faces(fs [][]Number) []Face {
	const perFace = for f fs {
		yield for i [1:<Size(of: f) - 1] {
			yield Face{v0: f[0], v1: f[i], v2: f[i + 1]}
		}
	}
	return fold acc, tris perFace { yield acc + tris }
}`

// scadV2PathHelper picks the indexed subset of a runtime points list and
// returns it as []Vec2 (mm). It is the paths-branch counterpart to scad_v2:
// when polygon(points=…, paths=…) has computed points, the emitter renders
// each path's literal indices as a Number list and lets this helper index
// into the runtime points at evaluation time.
const scadV2PathHelper = `fn scad_v2_path(ps [][]Number, indices []Number) []Vec2 {
	return for i indices {
		yield Vec2{x: ps[Number(from: i)][0] * 1 mm, y: ps[Number(from: i)][1] * 1 mm}
	}
}`

// scadLookupHelper implements OpenSCAD's lookup(key, table): linear interpolation
// of value over a [[key, value], …] table sorted by key, clamped at the ends.
// Each segment [k0, k1) contributes its interpolated value only when the key
// falls inside it (half-open, so an interior knot belongs to exactly one
// segment); summing the per-segment contributions yields the single match.
const scadLookupHelper = `fn scad_lookup(key Number, table [][]Number) Number {
	const n = Size(of: table)
	if key <= table[0][0] { return table[0][1] }
	if key >= table[n - 1][0] { return table[n - 1][1] }
	const contribs = for i [0 : n - 2] {
		const k0 = table[i][0]
		const k1 = table[i + 1][0]
		if key >= k0 && key < k1 {
			yield table[i][1] + (key - k0) / (k1 - k0) * (table[i + 1][1] - table[i][1])
		} else {
			yield 0
		}
	}
	return fold acc, c contribs { yield acc + c }
}`

// scadUnionHelper unions a runtime list of solids whose length isn't known at
// transpile time (conditional geometry — Union(arr: [G] + (cond ? [..] : []))).
// The stdlib Union requires at least two elements; a one-element list is just its
// element. (A zero-element list has no Solid form — Union then reports it.)
const scadUnionHelper = `fn scad_union(arr []Solid) Solid {
	if Size(of: arr) == 1 { return arr[0] }
	return Union(arr: arr)
}`

// scadCrossHelper is OpenSCAD's cross(a, b): the 3D vector cross product, kept in
// the []Number vector model (Facet's Cross takes Vec3, which would need a
// round-trip through Vec3 components and back).
const scadCrossHelper = `fn scad_cross(a, b []Number) []Number {
	return [
		a[1] * b[2] - a[2] * b[1],
		a[2] * b[0] - a[0] * b[2],
		a[0] * b[1] - a[1] * b[0],
	]
}`

// helperPreamble returns the definitions of every emitted helper the program
// references, each followed by a blank line. Unreferenced helpers are omitted.
func (e *Emitter) helperPreamble() string {
	var w writer
	for _, h := range []struct {
		used bool
		src  string
	}{
		{e.usesV2, scadV2Helper},
		{e.usesV3, scadV3Helper},
		{e.usesFaces, scadFacesHelper},
		{e.usesV2Path, scadV2PathHelper},
		{e.usesLookup, scadLookupHelper},
		{e.usesUnion, scadUnionHelper},
		{e.usesCross, scadCrossHelper},
	} {
		if h.used {
			w.write(h.src)
			w.write("\n")
		}
	}
	return w.str()
}
