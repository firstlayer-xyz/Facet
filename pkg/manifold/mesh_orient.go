package manifold

// orientOutward returns the triangle index list wound so the mesh's faces point
// OUTWARD (non-negative signed volume). A Solid is a bounded, outward-facing
// region: like extrude, revolve, and the primitives, a hand-built Mesh must not
// be able to come out inside-out. A mesh whose signed volume is negative (for
// example OpenSCAD's clockwise polyhedron winding) has every triangle reversed;
// one already outward — or degenerate/zero — is returned unchanged.
//
// Orientation is a single global sign for any valid closed 2-manifold (a
// non-orientable or non-manifold mesh isn't a Solid at all — the kernel yields an
// empty result either way), so this only ever flips a wholly-inverted mesh and
// preserves a hollow shape's relative inner/outer winding.
func orientOutward(vertices []float32, indices []uint32) []uint32 {
	if signedVolume6(vertices, indices) >= 0 {
		return indices
	}
	rev := make([]uint32, len(indices))
	copy(rev, indices)
	for i := 0; i+2 < len(rev); i += 3 {
		rev[i+1], rev[i+2] = rev[i+2], rev[i+1]
	}
	return rev
}

// signedVolume6 is six times the signed volume of the triangle soup
// (Σ v0·(v1×v2)); only its sign is used to decide orientation. vertices is a flat
// x,y,z list; indices are triangle vertex indices. An out-of-range index (a
// malformed mesh, which the kernel will reject anyway) stops the sum early rather
// than panicking.
func signedVolume6(vertices []float32, indices []uint32) float64 {
	var sum float64
	for i := 0; i+2 < len(indices); i += 3 {
		a, b, c := int(indices[i]), int(indices[i+1]), int(indices[i+2])
		if 3*a+2 >= len(vertices) || 3*b+2 >= len(vertices) || 3*c+2 >= len(vertices) {
			return sum
		}
		ax, ay, az := float64(vertices[3*a]), float64(vertices[3*a+1]), float64(vertices[3*a+2])
		bx, by, bz := float64(vertices[3*b]), float64(vertices[3*b+1]), float64(vertices[3*b+2])
		cx, cy, cz := float64(vertices[3*c]), float64(vertices[3*c+1]), float64(vertices[3*c+2])
		sum += ax*(by*cz-bz*cy) - ay*(bx*cz-bz*cx) + az*(bx*cy-by*cx)
	}
	return sum
}
