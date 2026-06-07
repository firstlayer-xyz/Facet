package evaluator

import (
	"context"
	"testing"
)

// evalBounds parses + evaluates src and returns the merged mesh's bounding box.
func evalBounds(t *testing.T, src string) (minX, minY, minZ, maxX, maxY, maxZ float32) {
	t.Helper()
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	return meshBounds(mesh)
}

// assertClose fails if got and want differ by more than 0.5 mm.
func assertClose(t *testing.T, label string, got, want float32) {
	t.Helper()
	if got-want > 0.5 || want-got > 0.5 {
		t.Errorf("%s: got %f, want ~%f", label, got, want)
	}
}

// Axis-angle rotation about +Z reproduces the euler Rotate(z:) primitive exactly
// — same bounds, not just swapped extents (so a sign error can't slip through).
func TestEvalRotateAxisAngleZ(t *testing.T) {
	box := `Cube(s: Vec3{x: 10 mm, y: 4 mm, z: 2 mm})`
	aMinX, aMinY, aMinZ, aMaxX, aMaxY, aMaxZ := evalBounds(t,
		`fn Main() Solid { return `+box+`.Rotate(axis: Vec3{z: 1 mm}, angle: 90 deg); }`)
	eMinX, eMinY, eMinZ, eMaxX, eMaxY, eMaxZ := evalBounds(t,
		`fn Main() Solid { return `+box+`.Rotate(z: 90 deg); }`)
	assertClose(t, "minX vs euler", aMinX, eMinX)
	assertClose(t, "minY vs euler", aMinY, eMinY)
	assertClose(t, "minZ vs euler", aMinZ, eMinZ)
	assertClose(t, "maxX vs euler", aMaxX, eMaxX)
	assertClose(t, "maxY vs euler", aMaxY, eMaxY)
	assertClose(t, "maxZ vs euler", aMaxZ, eMaxZ)
	// And concretely: +90° about +Z sends x:0..10 to y, y:0..4 to -x.
	assertClose(t, "minX", aMinX, -4)
	assertClose(t, "maxY", aMaxY, 10)
}

// Axis-angle rotation about +X reproduces euler Rotate(x:) exactly.
func TestEvalRotateAxisAngleX(t *testing.T) {
	box := `Cube(s: Vec3{x: 10 mm, y: 4 mm, z: 2 mm})`
	aMinX, aMinY, aMinZ, aMaxX, aMaxY, aMaxZ := evalBounds(t,
		`fn Main() Solid { return `+box+`.Rotate(axis: Vec3{x: 1 mm}, angle: 90 deg); }`)
	eMinX, eMinY, eMinZ, eMaxX, eMaxY, eMaxZ := evalBounds(t,
		`fn Main() Solid { return `+box+`.Rotate(x: 90 deg); }`)
	assertClose(t, "minX vs euler", aMinX, eMinX)
	assertClose(t, "minY vs euler", aMinY, eMinY)
	assertClose(t, "minZ vs euler", aMinZ, eMinZ)
	assertClose(t, "maxX vs euler", aMaxX, eMaxX)
	assertClose(t, "maxY vs euler", aMaxY, eMaxY)
	assertClose(t, "maxZ vs euler", aMaxZ, eMaxZ)
}

// A general (diagonal) axis matches the euler composition it should equal:
// rotating 120° about (1,1,1) cyclically permutes the axes (x->y->z->x).
func TestEvalRotateAxisAngleDiagonal(t *testing.T) {
	// Box offset off-origin so the permutation is unambiguous in the bounds.
	box := `Cube(s: Vec3{x: 6 mm, y: 4 mm, z: 2 mm}).Move(x: 10 mm, y: 20 mm, z: 30 mm)`
	minX, minY, minZ, maxX, maxY, maxZ := evalBounds(t,
		`fn Main() Solid { return `+box+`.Rotate(axis: Vec3{x: 1 mm, y: 1 mm, z: 1 mm}, angle: 120 deg); }`)
	// +120° about (1,1,1) cycles x̂->ŷ->ẑ->x̂, i.e. point (px,py,pz)->(pz,px,py):
	// x-extent(6)->y, y-extent(4)->z, z-extent(2)->x; corner (10,20,30)->(30,10,20).
	assertClose(t, "x-extent", maxX-minX, 2)
	assertClose(t, "y-extent", maxY-minY, 6)
	assertClose(t, "z-extent", maxZ-minZ, 4)
	assertClose(t, "minX", minX, 30)
	assertClose(t, "minY", minY, 10)
	assertClose(t, "minZ", minZ, 20)
}

// from->to rotation aligns +Z onto +X — equivalent to +90° about +Y, so a tall
// box ends up long along X and below the z=0 plane.
func TestEvalRotateFromTo(t *testing.T) {
	minX, _, minZ, maxX, _, maxZ := evalBounds(t,
		`fn Main() Solid { return Cube(s: Vec3{x: 2 mm, y: 2 mm, z: 20 mm}).Rotate(from: Vec3{z: 1 mm}, to: Vec3{x: 1 mm}); }`)
	assertClose(t, "minX", minX, 0)
	assertClose(t, "maxX", maxX, 20)
	assertClose(t, "minZ", minZ, -2)
	assertClose(t, "maxZ", maxZ, 0)
}

// from->to with identical directions is a no-op.
func TestEvalRotateFromToParallel(t *testing.T) {
	minX, minY, minZ, maxX, maxY, maxZ := evalBounds(t,
		`fn Main() Solid { return Cube(s: Vec3{x: 2 mm, y: 2 mm, z: 20 mm}).Rotate(from: Vec3{z: 1 mm}, to: Vec3{z: 1 mm}); }`)
	assertClose(t, "minX", minX, 0)
	assertClose(t, "minY", minY, 0)
	assertClose(t, "minZ", minZ, 0)
	assertClose(t, "maxX", maxX, 2)
	assertClose(t, "maxY", maxY, 2)
	assertClose(t, "maxZ", maxZ, 20)
}

// from->to with opposite directions flips 180° about a perpendicular axis: a
// corner-origin column (z:0..20) ends up at z:-20..0, same height, valid solid.
func TestEvalRotateFromToAntiparallel(t *testing.T) {
	prog := parseTestProg(t, `fn Main() Solid { return Cube(s: Vec3{x: 2 mm, y: 2 mm, z: 20 mm}).Rotate(from: Vec3{z: 1 mm}, to: Vec3{z: -1 mm}); }`)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil || len(mesh.Vertices) == 0 {
		t.Fatal("expected non-empty solid")
	}
	_, _, minZ, _, _, maxZ := meshBounds(mesh)
	assertClose(t, "minZ", minZ, -20)
	assertClose(t, "maxZ", maxZ, 0)
}
