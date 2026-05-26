package evaluator

import (
	"context"
	"testing"
)

// stdlibPredicateTest evaluates `fn Main() { if cond { 10mm cube } else { 5mm cube } }`
// and asserts the predicate held (10mm cube). Used by every operator and
// receiver-method test here that wants a yes/no answer about a piece of
// stdlib behavior.
func stdlibPredicateTest(t *testing.T, body string) {
	t.Helper()
	src := `
fn Main() {
` + body + `
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.1)
}

// stdlibIfThenCube wraps a single boolean expression in the
// "if cond { 10mm cube } else { 5mm cube }" pattern and asserts the
// predicate held. The thinnest possible call site for predicate tests.
func stdlibIfThenCube(t *testing.T, cond string) {
	t.Helper()
	stdlibIfThenCubeWithSetup(t, "", cond)
}

// stdlibIfThenCubeWithSetup is the same pattern but takes a `var` /
// `let` setup block that runs before the if. Use when the predicate
// needs to reference a variable declared on a previous line.
func stdlibIfThenCubeWithSetup(t *testing.T, setup, cond string) {
	t.Helper()
	stdlibPredicateTest(t, `
    `+setup+`
    if `+cond+` {
        return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    } else {
        return Cube(s: Vec3{x: 5 mm, y: 5 mm, z: 5 mm});
    }`)
}

// ── Vec3 arithmetic operators ─────────────────────────────────────────────────

func TestEvalVec3Add(t *testing.T) {
	stdlibIfThenCube(t, `(Vec3{x: 1 mm, y: 2 mm, z: 3 mm} + Vec3{x: 4 mm, y: 5 mm, z: 6 mm}).x == 5 mm`)
	stdlibIfThenCube(t, `(Vec3{x: 1 mm, y: 2 mm, z: 3 mm} + Vec3{x: 4 mm, y: 5 mm, z: 6 mm}).y == 7 mm`)
	stdlibIfThenCube(t, `(Vec3{x: 1 mm, y: 2 mm, z: 3 mm} + Vec3{x: 4 mm, y: 5 mm, z: 6 mm}).z == 9 mm`)
}

func TestEvalVec3Sub(t *testing.T) {
	stdlibIfThenCube(t, `(Vec3{x: 10 mm, y: 8 mm, z: 6 mm} - Vec3{x: 1 mm, y: 2 mm, z: 3 mm}).x == 9 mm`)
	stdlibIfThenCube(t, `(Vec3{x: 10 mm, y: 8 mm, z: 6 mm} - Vec3{x: 1 mm, y: 2 mm, z: 3 mm}).y == 6 mm`)
	stdlibIfThenCube(t, `(Vec3{x: 10 mm, y: 8 mm, z: 6 mm} - Vec3{x: 1 mm, y: 2 mm, z: 3 mm}).z == 3 mm`)
}

func TestEvalVec3MulComponentwise(t *testing.T) {
	stdlibIfThenCube(t, `(Vec3{x: 2 mm, y: 3 mm, z: 4 mm} * Vec3{x: 5, y: 6, z: 7}).x == 10 mm`)
	stdlibIfThenCube(t, `(Vec3{x: 2 mm, y: 3 mm, z: 4 mm} * Vec3{x: 5, y: 6, z: 7}).y == 18 mm`)
	stdlibIfThenCube(t, `(Vec3{x: 2 mm, y: 3 mm, z: 4 mm} * Vec3{x: 5, y: 6, z: 7}).z == 28 mm`)
}

func TestEvalVec3MulScalar(t *testing.T) {
	stdlibIfThenCube(t, `(Vec3{x: 1 mm, y: 2 mm, z: 3 mm} * 2).x == 2 mm`)
	stdlibIfThenCube(t, `(2 * Vec3{x: 1 mm, y: 2 mm, z: 3 mm}).y == 4 mm`)
	stdlibIfThenCube(t, `(Vec3{x: 1 mm, y: 2 mm, z: 3 mm} * 2).z == 6 mm`)
}

func TestEvalVec3DivScalar(t *testing.T) {
	stdlibIfThenCube(t, `(Vec3{x: 10 mm, y: 20 mm, z: 30 mm} / 2).x == 5 mm`)
	stdlibIfThenCube(t, `(Vec3{x: 10 mm, y: 20 mm, z: 30 mm} / 2).y == 10 mm`)
	stdlibIfThenCube(t, `(Vec3{x: 10 mm, y: 20 mm, z: 30 mm} / 2).z == 15 mm`)
}

func TestEvalVec3UnaryNeg(t *testing.T) {
	stdlibIfThenCube(t, `(-Vec3{x: 1 mm, y: -2 mm, z: 3 mm}).x == -1 mm`)
	stdlibIfThenCube(t, `(-Vec3{x: 1 mm, y: -2 mm, z: 3 mm}).y == 2 mm`)
	stdlibIfThenCube(t, `(-Vec3{x: 1 mm, y: -2 mm, z: 3 mm}).z == -3 mm`)
}

func TestEvalVec3Equality(t *testing.T) {
	stdlibIfThenCube(t, `Vec3{x: 1 mm, y: 2 mm, z: 3 mm} == Vec3{x: 1 mm, y: 2 mm, z: 3 mm}`)
	stdlibIfThenCube(t, `Vec3{x: 1 mm, y: 2 mm, z: 3 mm} != Vec3{x: 1 mm, y: 2 mm, z: 4 mm}`)
	stdlibIfThenCube(t, `!(Vec3{x: 1 mm, y: 2 mm, z: 3 mm} == Vec3{x: 1 mm, y: 2 mm, z: 4 mm})`)
}

// ── Vec2 arithmetic operators ─────────────────────────────────────────────────

func TestEvalVec2Add(t *testing.T) {
	stdlibIfThenCube(t, `(Vec2{x: 1 mm, y: 2 mm} + Vec2{x: 4 mm, y: 5 mm}).x == 5 mm`)
	stdlibIfThenCube(t, `(Vec2{x: 1 mm, y: 2 mm} + Vec2{x: 4 mm, y: 5 mm}).y == 7 mm`)
}

func TestEvalVec2Sub(t *testing.T) {
	stdlibIfThenCube(t, `(Vec2{x: 10 mm, y: 8 mm} - Vec2{x: 1 mm, y: 2 mm}).x == 9 mm`)
	stdlibIfThenCube(t, `(Vec2{x: 10 mm, y: 8 mm} - Vec2{x: 1 mm, y: 2 mm}).y == 6 mm`)
}

func TestEvalVec2MulComponentwise(t *testing.T) {
	stdlibIfThenCube(t, `(Vec2{x: 2 mm, y: 3 mm} * Vec2{x: 5, y: 6}).x == 10 mm`)
	stdlibIfThenCube(t, `(Vec2{x: 2 mm, y: 3 mm} * Vec2{x: 5, y: 6}).y == 18 mm`)
}

func TestEvalVec2MulScalar(t *testing.T) {
	stdlibIfThenCube(t, `(Vec2{x: 1 mm, y: 2 mm} * 3).x == 3 mm`)
	stdlibIfThenCube(t, `(3 * Vec2{x: 1 mm, y: 2 mm}).y == 6 mm`)
}

func TestEvalVec2DivScalar(t *testing.T) {
	stdlibIfThenCube(t, `(Vec2{x: 10 mm, y: 20 mm} / 2).x == 5 mm`)
	stdlibIfThenCube(t, `(Vec2{x: 10 mm, y: 20 mm} / 2).y == 10 mm`)
}

func TestEvalVec2UnaryNeg(t *testing.T) {
	stdlibIfThenCube(t, `(-Vec2{x: 1 mm, y: -2 mm}).x == -1 mm`)
	stdlibIfThenCube(t, `(-Vec2{x: 1 mm, y: -2 mm}).y == 2 mm`)
}

func TestEvalVec2Equality(t *testing.T) {
	stdlibIfThenCube(t, `Vec2{x: 1 mm, y: 2 mm} == Vec2{x: 1 mm, y: 2 mm}`)
	stdlibIfThenCube(t, `Vec2{x: 1 mm, y: 2 mm} != Vec2{x: 1 mm, y: 3 mm}`)
	stdlibIfThenCube(t, `!(Vec2{x: 1 mm, y: 2 mm} == Vec2{x: 1 mm, y: 3 mm})`)
}

// ── Vec convenience constructors ──────────────────────────────────────────────

func TestEvalVec3UniformConstructor(t *testing.T) {
	// Vec3(v) → {v, v, v}
	stdlibIfThenCube(t, `Vec3(v: 5 mm).x == 5 mm`)
	stdlibIfThenCube(t, `Vec3(v: 5 mm).y == 5 mm`)
	stdlibIfThenCube(t, `Vec3(v: 5 mm).z == 5 mm`)
}

func TestEvalVec3FromVec2AndZ(t *testing.T) {
	// Vec3(xy: Vec2, z) → {xy.x, xy.y, z}
	stdlibIfThenCube(t, `Vec3(xy: Vec2{x: 3 mm, y: 4 mm}, z: 5 mm).x == 3 mm`)
	stdlibIfThenCube(t, `Vec3(xy: Vec2{x: 3 mm, y: 4 mm}, z: 5 mm).y == 4 mm`)
	stdlibIfThenCube(t, `Vec3(xy: Vec2{x: 3 mm, y: 4 mm}, z: 5 mm).z == 5 mm`)
}

func TestEvalVec2UniformConstructor(t *testing.T) {
	stdlibIfThenCube(t, `Vec2(v: 7 mm).x == 7 mm`)
	stdlibIfThenCube(t, `Vec2(v: 7 mm).y == 7 mm`)
}

// ── Vec products: Cross, Dot ──────────────────────────────────────────────────

func TestEvalVec3Cross(t *testing.T) {
	// Cross strips units via Number(from:), so the result Vec3 holds bare
	// numeric components. e_x × e_y = e_z.
	stdlibIfThenCube(t, `Cross(a: Vec3{x: 1 mm, y: 0 mm, z: 0 mm}, b: Vec3{x: 0 mm, y: 1 mm, z: 0 mm}).z == 1 mm`)
	// Anti-commutative: a × b = -(b × a). Test x-component for e_y × e_z.
	stdlibIfThenCube(t, `Cross(a: Vec3{x: 0 mm, y: 1 mm, z: 0 mm}, b: Vec3{x: 0 mm, y: 0 mm, z: 1 mm}).x == 1 mm`)
}

func TestEvalVec3Dot(t *testing.T) {
	// Dot returns a Number (units stripped via Number(from:)). (1,2,3)·(4,5,6) = 32.
	stdlibIfThenCube(t, `Dot(a: Vec3{x: 1 mm, y: 2 mm, z: 3 mm}, b: Vec3{x: 4 mm, y: 5 mm, z: 6 mm}) == 32`)
	// Orthogonal: e_x · e_y = 0.
	stdlibIfThenCube(t, `Dot(a: Vec3{x: 1 mm, y: 0 mm, z: 0 mm}, b: Vec3{x: 0 mm, y: 1 mm, z: 0 mm}) == 0`)
}

func TestEvalVec2Dot(t *testing.T) {
	// (3,4)·(1,2) = 3 + 8 = 11.
	stdlibIfThenCube(t, `Dot(a: Vec2{x: 3 mm, y: 4 mm}, b: Vec2{x: 1 mm, y: 2 mm}) == 11`)
}

// ── Solid boolean operators ───────────────────────────────────────────────────
// + (union), - (difference), & (intersection) are exercised elsewhere in
// eval_operators_test.go. The two below — | (Insert) and ^ (Exclude) — are
// the operator forms of Solid.Insert and Solid.Exclude respectively.

func TestEvalSolidInsertOperator(t *testing.T) {
	// `a | b` ≡ a.Insert(part: b) — material from b takes priority where
	// they overlap. The result's bounding box should match the union of the
	// two operands.
	src := `
fn Main() Solid {
    var box = Cube(s: Vec3{x: 20 mm, y: 20 mm, z: 20 mm});
    var probe = Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).Move(v: Vec3{x: 15 mm, y: 0 mm, z: 0 mm});
    return box | probe;
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
	// Box is [0,20]³; probe covers [15,25]×[0,10]×[0,10]. Union extends X to 25.
	minX, _, _, maxX, _, _ := meshBounds(mesh)
	if minX > 0.1 || maxX < 24.9 {
		t.Errorf("Insert union bounds wrong: x=[%f, %f] want [~0, ~25]", minX, maxX)
	}
}

func TestEvalSolidExcludeOperator(t *testing.T) {
	// `a ^ b` ≡ a.Exclude(with: b) — symmetric difference; keeps material
	// where exactly one of a, b is present. With identical solids, result
	// should be empty (or near-empty by manifold simplification).
	src := `
fn Main() Solid {
    var box = Cube(s: Vec3{x: 20 mm, y: 20 mm, z: 20 mm});
    var probe = Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).Move(v: Vec3{x: 5 mm, y: 5 mm, z: 5 mm});
    return box ^ probe;
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil || len(mesh.Vertices) == 0 {
		t.Fatal("expected non-empty mesh — exclude with non-overlapping-by-shape probe should leave material")
	}
}

// ── Sketch boolean operators ──────────────────────────────────────────────────

func TestEvalSketchAddOperator(t *testing.T) {
	// `a + b` on Sketch → Sketch union. Build two non-overlapping squares,
	// extrude the union, verify the resulting solid covers both footprints.
	src := `
fn Main() Solid {
    var a = Square(s: 10 mm);
    var b = Square(s: 10 mm).Move(v: Vec2{x: 20 mm, y: 0 mm});
    return (a + b).Extrude(z: 5 mm);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	minX, _, _, maxX, _, _ := meshBounds(mesh)
	if minX > 0.1 || maxX < 29.9 {
		t.Errorf("Sketch + bounds wrong: x=[%f, %f] want [~0, ~30]", minX, maxX)
	}
}

func TestEvalSketchSubOperator(t *testing.T) {
	// `a - b` on Sketch → Sketch difference. Punch a smaller square out of
	// a larger one; extrude; the result should be a hollow extruded frame.
	src := `
fn Main() Solid {
    var outer = Square(s: 20 mm);
    var inner = Square(s: 10 mm).Move(v: Vec2{x: 5 mm, y: 5 mm});
    return (outer - inner).Extrude(z: 5 mm);
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
	// Outer bounds preserved.
	minX, minY, _, maxX, maxY, _ := meshBounds(mesh)
	if minX > 0.1 || maxX < 19.9 || minY > 0.1 || maxY < 19.9 {
		t.Errorf("Sketch - outer bounds wrong: x=[%f,%f], y=[%f,%f]", minX, maxX, minY, maxY)
	}
}
