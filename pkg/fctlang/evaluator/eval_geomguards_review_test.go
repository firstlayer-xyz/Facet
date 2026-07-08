package evaluator

import (
	"context"
	"strings"
	"testing"
)

// Deep-review regressions: domain guards at the geometry boundary. Each input
// previously reached the C++ kernel unguarded — OOM/abort, memory corruption,
// or a silently-empty model.

func geomExpectError(t *testing.T, src, wantSubstr string) {
	t.Helper()
	prog := parseTestProg(t, src)
	_, err := evalMerged(context.Background(), prog, nil)
	if err == nil {
		t.Fatalf("expected an error containing %q, got success", wantSubstr)
	}
	if !strings.Contains(err.Error(), wantSubstr) {
		t.Fatalf("expected error containing %q, got: %v", wantSubstr, err)
	}
}

// LevelSet with a zero edge length was a division-by-zero in the kernel's
// grid-size computation (bad_alloc through cgo kills the process).
func TestEvalLevelSetZeroEdgeLengthErrors(t *testing.T) {
	geomExpectError(t, `
fn Main() {
    var b = Box{min: Vec3{x: 0 mm, y: 0 mm, z: 0 mm}, max: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}};
    return LevelSet(f: fn(p Vec3) Number { return 1; }, bounds: b, edgeLength: 0 mm);
}
`, "positive")
}

// A tiny edge length over a large box is an unbounded voxel allocation.
func TestEvalLevelSetVoxelCap(t *testing.T) {
	geomExpectError(t, `
fn Main() {
    var b = Box{min: Vec3{x: 0 mm, y: 0 mm, z: 0 mm}, max: Vec3{x: 1000 mm, y: 1000 mm, z: 1000 mm}};
    return LevelSet(f: fn(p Vec3) Number { return 1; }, bounds: b, edgeLength: 0.01 mm);
}
`, "voxels")
}

// RefineToLength(0) saturated to INT_MAX subdivisions per edge in the kernel.
func TestEvalRefineToLengthZeroErrors(t *testing.T) {
	geomExpectError(t, `
fn Main() {
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).RefineToLength(edgeLength: 0 mm);
}
`, "positive")
}

// Revolve with a non-positive angle drove the kernel's slice count negative
// while cap triangles still indexed the (empty) vertex array; an angle above
// 360 is silently clamped to a full revolution. Both are out of the (0, 360] domain.
func TestEvalRevolveNonPositiveAngleErrors(t *testing.T) {
	geomExpectError(t, `
fn Main() {
    return Circle(r: 5 mm).Move(x: 10 mm).Revolve(a: -90 deg);
}
`, "(0, 360]")
}

func TestEvalRevolveAngleAbove360Errors(t *testing.T) {
	geomExpectError(t, `
fn Main() {
    return Circle(r: 5 mm).Move(x: 10 mm).Revolve(a: 400 deg);
}
`, "(0, 360]")
}

// A negative taper is silently clamped to 0 by the kernel; reject it.
func TestEvalNegativeTaperErrors(t *testing.T) {
	geomExpectError(t, `
fn Main() {
    return Square(x: 10 mm, y: 10 mm).Extrude(z: 5 mm, taperX: -0.5)
}
`, "taper must be non-negative")
}

// A zero plane normal normalized to NaN in the kernel — silent garbage.
func TestEvalTrimZeroNormalErrors(t *testing.T) {
	geomExpectError(t, `
fn Main() {
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).Trim(x: 0, y: 0, z: 0);
}
`, "non-zero")
}

// A negative extrude height silently produced an empty solid (the kernel
// returns Invalid for height <= 0 without erroring).
func TestEvalNegativeExtrudeErrors(t *testing.T) {
	geomExpectError(t, `
fn Main() {
    return Square(x: 10 mm, y: 10 mm).Extrude(z: -5 mm);
}
`, "positive")
}

// An open (non-closed) PolyMesh silently rendered as nothing; it must error
// like the Mesh path does. Out-of-range face indices error too.
func TestEvalOpenPolyMeshErrors(t *testing.T) {
	geomExpectError(t, `
fn Main() PolyMesh {
    return PolyMesh{
        vertices: [Vec3{x: 0 mm, y: 0 mm, z: 0 mm}, Vec3{x: 10 mm, y: 0 mm, z: 0 mm}, Vec3{x: 0 mm, y: 10 mm, z: 0 mm}],
        faces: [[0, 1, 2]],
    };
}
`, "closed manifold")

	geomExpectError(t, `
fn Main() PolyMesh {
    return PolyMesh{
        vertices: [Vec3{x: 0 mm, y: 0 mm, z: 0 mm}, Vec3{x: 10 mm, y: 0 mm, z: 0 mm}, Vec3{x: 0 mm, y: 10 mm, z: 0 mm}],
        faces: [[0, 1, 7]],
    };
}
`, "vertex index")
}
