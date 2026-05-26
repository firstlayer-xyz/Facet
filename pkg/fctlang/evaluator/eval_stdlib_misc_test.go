package evaluator

import (
	"context"
	"testing"
)

// ── Mesh.* face-coordinate accessors ──────────────────────────────────────────
// All tested against a 20mm cube → mesh: bounds (0,0,0)..(20,20,20).

const stdlibTestCubeMesh = `var m = Cube(s: Vec3{x: 20 mm, y: 20 mm, z: 20 mm}).Mesh();`

func TestEvalMeshLeft(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, stdlibTestCubeMesh, `m.Left() == 0 mm`)
}

func TestEvalMeshRight(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, stdlibTestCubeMesh, `m.Right() == 20 mm`)
}

func TestEvalMeshFront(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, stdlibTestCubeMesh, `m.Front() == 0 mm`)
}

func TestEvalMeshBack(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, stdlibTestCubeMesh, `m.Back() == 20 mm`)
}

func TestEvalMeshBottom(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, stdlibTestCubeMesh, `m.Bottom() == 0 mm`)
}

func TestEvalMeshTop(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, stdlibTestCubeMesh, `m.Top() == 20 mm`)
}

// ── PolyMesh.* face-coordinate accessors ──────────────────────────────────────

const stdlibTestCubePolyMesh = `var pm = Cube(s: Vec3{x: 20 mm, y: 20 mm, z: 20 mm}).PolyMesh();`

func TestEvalPolyMeshLeft(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, stdlibTestCubePolyMesh, `pm.Left() == 0 mm`)
}

func TestEvalPolyMeshRight(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, stdlibTestCubePolyMesh, `pm.Right() == 20 mm`)
}

func TestEvalPolyMeshFront(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, stdlibTestCubePolyMesh, `pm.Front() == 0 mm`)
}

func TestEvalPolyMeshBack(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, stdlibTestCubePolyMesh, `pm.Back() == 20 mm`)
}

func TestEvalPolyMeshBottom(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, stdlibTestCubePolyMesh, `pm.Bottom() == 0 mm`)
}

func TestEvalPolyMeshTop(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, stdlibTestCubePolyMesh, `pm.Top() == 20 mm`)
}

// ── PolyMesh.ScaleUniform — multiplies all vertex coordinates ─────────────────

func TestEvalPolyMeshScaleUniform(t *testing.T) {
	src := `
fn Main() Solid {
    return Cube(s: Vec3{x: 5 mm, y: 5 mm, z: 5 mm}).PolyMesh().ScaleUniform(factor: 2).Solid();
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	// 5mm × 2 = 10mm.
	assertMeshSize(t, mesh, 10, 10, 10, 0.5)
}

// ── Cone — cylinder with top radius zero ──────────────────────────────────────

func TestEvalCone(t *testing.T) {
	src := `
fn Main() Solid {
    return Cone(r: 10 mm, h: 20 mm);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil || len(mesh.Vertices) == 0 {
		t.Fatal("expected non-empty cone mesh")
	}
	// Cone footprint: 20mm wide (diameter), 20mm tall.
	minX, _, minZ, maxX, _, maxZ := meshBounds(mesh)
	if maxX-minX < 19.5 || maxX-minX > 20.5 {
		t.Errorf("Cone x-extent: %f, want ~20", maxX-minX)
	}
	if maxZ-minZ < 19.5 || maxZ-minZ > 20.5 {
		t.Errorf("Cone z-extent: %f, want ~20", maxZ-minZ)
	}
}

// ── Arc / Arc3d — generate point lists along an arc ───────────────────────────
// We just verify segment count — exact (x,y) positions depend on the Cos/Sin
// implementations covered by the trig tests.

func TestEvalArc(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `
    var pts = Arc(center: Vec2{x: 0 mm, y: 0 mm}, r: 10 mm, startAngle: 0 deg, endAngle: 90 deg, segments: 4);`,
		// Arc uses `for i [0:segments]` (inclusive), so segments=4 yields 5 points.
		`Size(of: pts) == 5`)
}

func TestEvalArc3d(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `
    var pts = Arc3d(center: Vec3{x: 0 mm, y: 0 mm, z: 5 mm}, r: 10 mm, startAngle: 0 deg, endAngle: 90 deg, segments: 4);`,
		// Arc uses `for i [0:segments]` (inclusive), so segments=4 yields 5 points.
		`Size(of: pts) == 5`)
}

// ── Compose / Decompose — solid composition / disconnect ──────────────────────

func TestEvalCompose(t *testing.T) {
	// Composing two non-overlapping cubes should give a single Solid whose
	// bounds span both.
	src := `
fn Main() Solid {
    var a = Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    var b = Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).Move(v: Vec3{x: 20 mm, y: 0 mm, z: 0 mm});
    return Compose(solids: [a, b]);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	minX, _, _, maxX, _, _ := meshBounds(mesh)
	if minX > 0.1 || maxX < 29.9 {
		t.Errorf("Compose bounds: x=[%f,%f], want [~0,~30]", minX, maxX)
	}
}

func TestEvalDecompose(t *testing.T) {
	// Decompose a Compose of two disconnected cubes back to two solids.
	stdlibIfThenCubeWithSetup(t, `
    var a = Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    var b = Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).Move(v: Vec3{x: 20 mm, y: 0 mm, z: 0 mm});
    var combined = Compose(solids: [a, b]);
    var pieces = Decompose(s: combined);`,
		`Size(of: pieces) == 2`)
}

// ── Layout — places solids side by side along x with a gap ────────────────────

func TestEvalLayout(t *testing.T) {
	// Layout returns a []Solid of the same length as its input.
	stdlibIfThenCubeWithSetup(t, `
    var arr = Layout(solids: [
        Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}),
        Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}),
        Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}),
    ], gap: 5 mm);`,
		`Size(of: arr) == 3`)
}

// ── N-ary boolean operations on solid arrays ──────────────────────────────────

func TestEvalDifferenceArraySolid(t *testing.T) {
	// 20mm cube minus 10mm cube minus another 10mm cube — chain difference.
	src := `
fn Main() Solid {
    return Difference(arr: [
        Cube(s: Vec3{x: 20 mm, y: 20 mm, z: 20 mm}),
        Cube(s: Vec3{x: 5 mm, y: 5 mm, z: 5 mm}),
        Cube(s: Vec3{x: 5 mm, y: 5 mm, z: 5 mm}).Move(v: Vec3{x: 10 mm, y: 0 mm, z: 0 mm}),
    ]);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil || len(mesh.Vertices) == 0 {
		t.Fatal("expected non-empty mesh")
	}
}

func TestEvalIntersectionArraySolid(t *testing.T) {
	// Three overlapping cubes — the intersection is the central overlap.
	src := `
fn Main() Solid {
    return Intersection(arr: [
        Cube(s: Vec3{x: 20 mm, y: 20 mm, z: 20 mm}),
        Cube(s: Vec3{x: 20 mm, y: 20 mm, z: 20 mm}).Move(v: Vec3{x: 5 mm, y: 0 mm, z: 0 mm}),
        Cube(s: Vec3{x: 20 mm, y: 20 mm, z: 20 mm}).Move(v: Vec3{x: 0 mm, y: 5 mm, z: 0 mm}),
    ]);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil || len(mesh.Vertices) == 0 {
		t.Fatal("expected non-empty intersection mesh")
	}
}

// ── EvenOdd — N-ary symmetric difference (XOR) ────────────────────────────────

func TestEvalEvenOddArraySolid(t *testing.T) {
	src := `
fn Main() Solid {
    return EvenOdd(solids: [
        Cube(s: Vec3{x: 20 mm, y: 20 mm, z: 20 mm}),
        Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).Move(v: Vec3{x: 5 mm, y: 5 mm, z: 5 mm}),
    ]);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil || len(mesh.Vertices) == 0 {
		t.Fatal("expected non-empty EvenOdd mesh")
	}
}

// ── Face — constructor for a triangle face descriptor ─────────────────────────

func TestEvalFaceConstructor(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `
    var f = Face(v0: 1, v1: 2, v2: 3);`,
		`f.v0 == 1 && f.v1 == 2 && f.v2 == 3`)
}

// ── LevelSet — implicit-surface to solid via SDF ──────────────────────────────

func TestEvalLevelSet(t *testing.T) {
	// SDF for a sphere of radius 10 centered at (10,10,10). Inline lambda
	// because Facet doesn't take top-level fn names as values.
	src := `
fn Main() Solid {
    var bounds = Box{
        min: Vec3{x: 0 mm, y: 0 mm, z: 0 mm},
        max: Vec3{x: 20 mm, y: 20 mm, z: 20 mm},
    };
    return LevelSet(
        f: fn(p Vec3) Number {
            var dx = Number(from: p.x) - 10;
            var dy = Number(from: p.y) - 10;
            var dz = Number(from: p.z) - 10;
            return Sqrt(n: dx * dx + dy * dy + dz * dz) - 10
        },
        bounds: bounds,
        edgeLength: 1 mm
    );
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil || len(mesh.Vertices) == 0 {
		t.Fatal("expected non-empty LevelSet mesh")
	}
}

// ── Color.Hex — convert to hex string ─────────────────────────────────────────

func TestEvalColorHex(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `
    var red = Color(r: 1, g: 0, b: 0, a: 1);`,
		`red.Hex().HasPrefix(prefix: "#")`)
}

// ── UtcTime — current UTC time as ISO-8601 ────────────────────────────────────

func TestEvalUtcTime(t *testing.T) {
	// Just verify it returns a non-empty string. Format detail is left to
	// the underlying _utc_time implementation.
	stdlibIfThenCubeWithSetup(t, `
    var now = UtcTime();`,
		`Size(of: now) > 0`)
}
