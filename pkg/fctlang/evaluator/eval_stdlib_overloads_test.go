package evaluator

import (
	"context"
	"testing"
)

// Tests for the alternate execution paths inside std.fct — the cases
// where a function dispatches differently based on which overload was
// called, an `if` branch on parameter values, or a default-vs-explicit
// option that flips the implementation.

// ── Cube — three overloads, plus the r==0 vs r>0 internal branch ──────────────

func TestEvalCubeUniformLength(t *testing.T) {
	// Cube(s Length) — convenience: build a uniform NxNxN cube from a
	// single Length, distinct from Cube(s Vec3) which takes per-axis sizes.
	src := `fn Main() Solid { return Cube(s: 12 mm); }`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 12, 12, 12, 0.1)
}

// ── Sphere / Cylinder / Frustum — diameter overloads ──────────────────────────

func TestEvalSphereDiameter(t *testing.T) {
	// Sphere(d: 20mm) is the same as Sphere(r: 10mm) — verify by bounds.
	src := `fn Main() Solid { return Sphere(d: 20 mm); }`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	minX, _, _, maxX, _, _ := meshBounds(mesh)
	if maxX-minX < 19 || maxX-minX > 21 {
		t.Errorf("Sphere(d:20mm) x-extent: got %f, want ~20", maxX-minX)
	}
}

func TestEvalCylinderDiameter(t *testing.T) {
	src := `fn Main() Solid { return Cylinder(d: 20 mm, h: 10 mm); }`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	minX, _, minZ, maxX, _, maxZ := meshBounds(mesh)
	if maxX-minX < 19 || maxX-minX > 21 {
		t.Errorf("Cylinder(d:20) x-extent: got %f, want ~20", maxX-minX)
	}
	if maxZ-minZ < 9.5 || maxZ-minZ > 10.5 {
		t.Errorf("Cylinder(d:20) z-extent: got %f, want ~10", maxZ-minZ)
	}
}

func TestEvalFrustumDiameter(t *testing.T) {
	// Frustum diameter form: bottom diameter 20mm, top 10mm, height 15mm.
	src := `fn Main() Solid { return Frustum(d1: 20 mm, d2: 10 mm, h: 15 mm); }`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	minX, _, minZ, maxX, _, maxZ := meshBounds(mesh)
	if maxX-minX < 19 || maxX-minX > 21 {
		t.Errorf("Frustum bottom diameter: got %f, want ~20", maxX-minX)
	}
	if maxZ-minZ < 14.5 || maxZ-minZ > 15.5 {
		t.Errorf("Frustum height: got %f, want ~15", maxZ-minZ)
	}
}

// ── Sphere / Cylinder / Frustum — explicit segment count ──────────────────────
// Higher segment counts produce more triangles. Just verify the call doesn't
// fail and the output bounds match.

func TestEvalSphereExplicitSegments(t *testing.T) {
	src := `fn Main() Solid { return Sphere(r: 10 mm, segments: 12); }`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil || len(mesh.Vertices) == 0 {
		t.Fatal("expected non-empty mesh")
	}
}

func TestEvalCylinderHexagonal(t *testing.T) {
	// segments: 6 makes Cylinder a hexagonal prism.
	src := `fn Main() Solid { return Cylinder(r: 10 mm, h: 5 mm, segments: 6); }`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil || len(mesh.Vertices) == 0 {
		t.Fatal("expected non-empty hex prism mesh")
	}
}

func TestEvalFrustumExplicitSegments(t *testing.T) {
	src := `fn Main() Solid { return Frustum(r1: 10 mm, r2: 5 mm, h: 10 mm, segments: 8); }`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil || len(mesh.Vertices) == 0 {
		t.Fatal("expected non-empty frustum mesh")
	}
}

// (Prism overloads — both regular and arbitrary-triangle — are covered by
// TestEvalPrismRegular / TestEvalPrismHexagonal / TestEvalPrismArbitraryTriangle
// in eval_test.go.)

// ── Hull — three overloads on different array element types ───────────────────

func TestEvalHullOfSolids(t *testing.T) {
	// Convex hull of two cubes at separated positions.
	src := `
fn Main() Solid {
    return Hull(arr: [
        Cube(s: Vec3{x: 5 mm, y: 5 mm, z: 5 mm}),
        Cube(s: Vec3{x: 5 mm, y: 5 mm, z: 5 mm}).Move(v: Vec3{x: 20 mm, y: 0 mm, z: 0 mm}),
    ]);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	minX, _, _, maxX, _, _ := meshBounds(mesh)
	if maxX-minX < 24 || maxX-minX > 26 {
		t.Errorf("Hull-of-solids x-extent: got %f, want ~25", maxX-minX)
	}
}

func TestEvalHullOfSketches(t *testing.T) {
	src := `
fn Main() Solid {
    return Hull(arr: [
        Square(s: 5 mm),
        Square(s: 5 mm).Move(v: Vec2{x: 20 mm, y: 0 mm}),
    ]).Extrude(z: 5 mm);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil || len(mesh.Vertices) == 0 {
		t.Fatal("expected non-empty hull-of-sketches mesh")
	}
}

func TestEvalHullOfPoints(t *testing.T) {
	// Hull of an array of Vec3 points → Solid (the convex polytope).
	src := `
fn Main() Solid {
    return Hull(arr: [
        Vec3{x: 0 mm, y: 0 mm, z: 0 mm},
        Vec3{x: 20 mm, y: 0 mm, z: 0 mm},
        Vec3{x: 0 mm, y: 20 mm, z: 0 mm},
        Vec3{x: 0 mm, y: 0 mm, z: 20 mm},
    ]);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil || len(mesh.Vertices) == 0 {
		t.Fatal("expected non-empty tetrahedron hull")
	}
}

// ── Solid.AlignCenter — two overloads, plus axis-toggle branches ──────────────

func TestEvalSolidAlignCenterWithSolid(t *testing.T) {
	// Default: align all three axes — small cube centered on big cube.
	stdlibIfThenCubeWithSetup(t, `
    var big = Cube(s: Vec3{x: 20 mm, y: 20 mm, z: 20 mm});
    var small = Cube(s: Vec3{x: 4 mm, y: 4 mm, z: 4 mm}).AlignCenter(with: big);`,
		`small.Center() == big.Center()`)
}

func TestEvalSolidAlignCenterWithPos(t *testing.T) {
	// Align all three axes to a fixed point.
	stdlibIfThenCubeWithSetup(t, `
    var c = Cube(s: Vec3{x: 4 mm, y: 4 mm, z: 4 mm}).AlignCenter(pos: Vec3{x: 50 mm, y: 50 mm, z: 50 mm});`,
		`c.Center() == Vec3{x: 50 mm, y: 50 mm, z: 50 mm}`)
}

func TestEvalSolidAlignCenterAxisToggleX(t *testing.T) {
	// y=false, z=false: only x is aligned. Original y/z are preserved.
	stdlibIfThenCubeWithSetup(t, `
    var c = Cube(s: Vec3{x: 4 mm, y: 4 mm, z: 4 mm}).AlignCenter(pos: Vec3{x: 50 mm, y: 50 mm, z: 50 mm}, x: true, y: false, z: false);`,
		`c.Center().x == 50 mm && c.Center().y == 2 mm && c.Center().z == 2 mm`)
}

func TestEvalSolidAlignCenterAxisToggleY(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `
    var c = Cube(s: Vec3{x: 4 mm, y: 4 mm, z: 4 mm}).AlignCenter(pos: Vec3{x: 50 mm, y: 50 mm, z: 50 mm}, x: false, y: true, z: false);`,
		`c.Center().y == 50 mm && c.Center().x == 2 mm`)
}

// ── Solid.Color — two overloads ───────────────────────────────────────────────

func TestEvalSolidColorByColor(t *testing.T) {
	// Solid.Color(c: Color) — the typed-Color overload.
	src := `
fn Main() Solid {
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).Color(c: Color(r: 0.8, g: 0.2, b: 0.1, a: 1));
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

func TestEvalSolidColorByHex(t *testing.T) {
	// Solid.Color(hex: String) — the hex-string overload.
	src := `
fn Main() Solid {
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).Color(hex: "#ff8800");
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

// (Solid.Resize's early-return-on-zero-source-dimension branch only fires
// for solids whose Bounds() has a zero extent — pathological state that
// can't be produced by the public API. The non-degenerate path is covered
// by TestEvalSolidResize in eval_stdlib_solid_methods_test.go.)

// ── Sketch.CircularPattern — both branches ────────────────────────────────────

func TestEvalSketchCircularPatternFullRevolution(t *testing.T) {
	// span defaults to 0deg → full 360° revolution path. 8 squares spread
	// uniformly around the origin.
	src := `
fn Main() Solid {
    var pat = Square(s: 4 mm).Move(v: Vec2{x: 20 mm, y: 0 mm}).CircularPattern(count: 8);
    return pat.Extrude(z: 1 mm);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	// Bounding box should be roughly symmetric ±~22mm in xy (the offset
	// circle plus square half-width).
	minX, minY, _, maxX, maxY, _ := meshBounds(mesh)
	width := maxX - minX
	depth := maxY - minY
	if width < 38 || width > 50 {
		t.Errorf("CircularPattern full x-extent: got %f, want ~44", width)
	}
	if depth < 38 || depth > 50 {
		t.Errorf("CircularPattern full y-extent: got %f, want ~44", depth)
	}
}

func TestEvalSketchCircularPatternPartialSpan(t *testing.T) {
	// span > 0 → partial-revolution branch.
	src := `
fn Main() Solid {
    var pat = Square(s: 4 mm).Move(v: Vec2{x: 20 mm, y: 0 mm}).CircularPattern(count: 4, span: 90 deg);
    return pat.Extrude(z: 1 mm);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil || len(mesh.Vertices) == 0 {
		t.Fatal("expected non-empty partial CircularPattern mesh")
	}
}

// ── Pattern count==1 — the single-copy boundary of the batch-union path ───────

// A pattern of count 1 yields one copy. The aggregate helpers fold it with
// Union(copies), which must return that single copy unchanged rather than
// erroring on "too few operands" — the guard now accepts a 1-element batch.
func TestEvalLinearPatternSingleCopy(t *testing.T) {
	src := `
fn Main() Solid {
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).LinearPattern(count: 1, gap: 10 mm);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	// One copy → a plain 10mm cube, no second instance 20mm away.
	assertMeshSize(t, mesh, 10, 10, 10, 0.01)
}

// Three copies at pitch = cell width + gap = 10 + 10 = 20mm span from the first
// cube's near edge to the third cube's far edge: 2*20 + 10 = 50mm. Exercises the
// N>=3 batch-union path through the stdlib pattern helper.
func TestEvalLinearPatternThreeCopies(t *testing.T) {
	src := `
fn Main() Solid {
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).LinearPattern(count: 3, gap: 10 mm);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 50, 10, 10, 0.01)
}
