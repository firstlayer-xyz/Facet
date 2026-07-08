package evaluator

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// infLen is a Facet expression that evaluates to a non-finite (+Inf) Length.
// Facet forbids non-finite number literals (the lexer rejects them) and guards
// division/pow/sqrt/Number(from:), so overflowing Length multiplication is the
// only way to mint a non-finite coordinate: 1e200 mm * 1e200 = 1e400 = +Inf.
func infLen() string {
	big := "1" + strings.Repeat("0", 200) // 1e200, parses finite
	return fmt.Sprintf("%s mm * %s", big, big)
}

// Vertex coordinates flow straight into the C++ geometry kernel, which treats
// NaN/Inf as pass-through and silently corrupts the whole mesh. Each Vec-taking
// kernel boundary must reject non-finite coordinates, mirroring requireFinite
// for scalar arguments.
func TestEvalNonFiniteVecCoordinatesError(t *testing.T) {
	inf := infLen()
	cases := []struct{ name, src string }{
		{"polygon", fmt.Sprintf(`fn Main() Solid {
    return Polygon(points: [Vec2{x: 0 mm, y: 0 mm}, Vec2{x: %s, y: 0 mm}, Vec2{x: 1 mm, y: 1 mm}]).Extrude(z: 5 mm);
}`, inf)},
		{"hull", fmt.Sprintf(`fn Main() Solid {
    return Hull(arr: [Vec3{x: 0 mm, y: 0 mm, z: 0 mm}, Vec3{x: %s, y: 0 mm, z: 0 mm}, Vec3{x: 1 mm, y: 1 mm, z: 0 mm}, Vec3{x: 0 mm, y: 0 mm, z: 5 mm}]);
}`, inf)},
		{"sweep", fmt.Sprintf(`fn Main() Solid {
    return Circle(r: 5 mm).Sweep(path: [Vec3{x: 0 mm, y: 0 mm, z: 0 mm}, Vec3{x: %s, y: 0 mm, z: 10 mm}]);
}`, inf)},
		{"polymesh", fmt.Sprintf(`fn Main() PolyMesh {
    return PolyMesh{
        vertices: [Vec3{x: 0 mm, y: 0 mm, z: 0 mm}, Vec3{x: %s, y: 0 mm, z: 0 mm}, Vec3{x: 0 mm, y: 10 mm, z: 0 mm}, Vec3{x: 0 mm, y: 0 mm, z: 10 mm}],
        faces: [[0, 2, 1], [0, 1, 3], [0, 3, 2], [1, 2, 3]],
    };
}`, inf)},
		{"warp", fmt.Sprintf(`fn Main() Solid {
    return Cube(s: 10 mm).Warp(f: fn(p Vec3) Vec3 { return Vec3{x: %s, y: p.y, z: p.z}; });
}`, inf)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			geomExpectError(t, tc.src, "finite coordinates")
		})
	}
}

// The finite-coordinate guard must not reject legitimate finite geometry.
func TestEvalFiniteVecCoordinatesOK(t *testing.T) {
	s := evalSolid(t, `fn Main() Solid {
    return Polygon(points: [Vec2{x: 0 mm, y: 0 mm}, Vec2{x: 10 mm, y: 0 mm}, Vec2{x: 5 mm, y: 10 mm}]).Extrude(z: 5 mm);
}`)
	if v := s.Volume(); v <= 0 {
		t.Fatalf("finite polygon extrude: want positive volume, got %v", v)
	}
}

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
