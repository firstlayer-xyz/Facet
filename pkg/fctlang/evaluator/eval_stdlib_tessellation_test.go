package evaluator

import (
	"context"
	"strings"
	"testing"

	"facet/share/examples"
)

// ── Grid (count + region forms) ──────────────────────────────────────────────

// Count form: 10×10 of a 10mm cube, gap 0, merges into a 100×100×10 slab.
func TestEvalSolidGridPatternCount(t *testing.T) {
	src := `
fn Main() Solid {
    return Cube(s: 10 mm).GridPattern(countX: 10, countY: 10);
}
`
	mesh, err := evalMerged(context.Background(), parseTestProg(t, src), nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 100, 100, 10, 0.5)
}

// Region form fills the area with the derived count — same 100×100×10 result.
func TestEvalSolidGridPatternRegion(t *testing.T) {
	src := `
fn Main() Solid {
    return Cube(s: 10 mm).GridPattern(width: 100 mm, depth: 100 mm);
}
`
	mesh, err := evalMerged(context.Background(), parseTestProg(t, src), nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 100, 100, 10, 0.5)
}

// rowOffset 0.5 shifts odd rows half a pitch (5mm), widening X from 100 to 105.
func TestEvalSolidGridPatternRowOffset(t *testing.T) {
	src := `
fn Main() Solid {
    return Cube(s: 10 mm).GridPattern(countX: 10, countY: 10, rowOffset: 0.5);
}
`
	mesh, err := evalMerged(context.Background(), parseTestProg(t, src), nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 105, 100, 10, 0.5)
}

// A positive gap reduces how many cells fit in a fixed region (pitch 12 → 8 cells).
func TestEvalSolidGridPatternGap(t *testing.T) {
	src := `
fn Main() Solid {
    return Cube(s: 10 mm).GridPattern(width: 100 mm, depth: 100 mm, gap: Vec2{x: 2 mm, y: 2 mm});
}
`
	mesh, err := evalMerged(context.Background(), parseTestProg(t, src), nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 94, 94, 10, 0.5)
}

func TestEvalGridPatternTooSmall(t *testing.T) {
	src := `
fn Main() Solid {
    return Cube(s: 10 mm).GridPattern(width: 5 mm, depth: 100 mm);
}
`
	if msg := evalErr(t, src); !strings.Contains(msg, "GridPattern: width too small to fit one copy") {
		t.Errorf("got: %s\nwant substring about width too small", msg)
	}
}

func TestEvalSketchGridPatternRegion(t *testing.T) {
	src := `
fn Main() Solid {
    return Square(s: 10 mm).GridPattern(width: 100 mm, depth: 100 mm).Extrude(z: 1 mm);
}
`
	mesh, err := evalMerged(context.Background(), parseTestProg(t, src), nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 100, 100, 1, 0.5)
}

// ── Hex (count + region forms) ───────────────────────────────────────────────

// Hex packing of a 10mm cube: horizontal pitch 10 → odd rows offset +5 widen X
// to 105; row pitch 10·√3/2 ≈ 8.66 over 11 rows → Y ≈ 96.6.
func TestEvalSolidHexPatternCount(t *testing.T) {
	src := `
fn Main() Solid {
    return Cube(s: 10 mm).HexPattern(countX: 10, countY: 11);
}
`
	mesh, err := evalMerged(context.Background(), parseTestProg(t, src), nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 105, 96.6, 10, 0.6)
}

// Region form derives the same 10×11 lattice from a 100×100 area.
func TestEvalSolidHexPatternRegion(t *testing.T) {
	src := `
fn Main() Solid {
    return Cube(s: 10 mm).HexPattern(width: 100 mm, depth: 100 mm);
}
`
	mesh, err := evalMerged(context.Background(), parseTestProg(t, src), nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 105, 96.6, 10, 0.6)
}

func TestEvalHexPatternTooSmall(t *testing.T) {
	src := `
fn Main() Solid {
    return Cube(s: 10 mm).HexPattern(width: 5 mm, depth: 100 mm);
}
`
	if msg := evalErr(t, src); !strings.Contains(msg, "HexPattern: width too small to fit one copy") {
		t.Errorf("got: %s\nwant substring about width too small", msg)
	}
}

func TestEvalSketchHexPatternCount(t *testing.T) {
	src := `
fn Main() Solid {
    return Square(s: 10 mm).HexPattern(countX: 10, countY: 11).Extrude(z: 1 mm);
}
`
	mesh, err := evalMerged(context.Background(), parseTestProg(t, src), nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 105, 96.6, 1, 0.6)
}

// ── Linear (size-aware gap + axis) ───────────────────────────────────────────

// Count form along the default +X axis: pitch = cell width (10) + gap (5) = 15;
// 3 copies span to 40.
func TestEvalSolidLinearPatternGapX(t *testing.T) {
	src := `
fn Main() Solid {
    return Cube(s: 10 mm).LinearPattern(count: 3, gap: 5 mm);
}
`
	mesh, err := evalMerged(context.Background(), parseTestProg(t, src), nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 40, 10, 10, 0.5)
}

// axis selects the stacking direction — here +Z, so the array grows in height.
func TestEvalSolidLinearPatternAxisZ(t *testing.T) {
	src := `
fn Main() Solid {
    return Cube(s: 10 mm).LinearPattern(count: 3, gap: 5 mm, axis: Vec3{z: 1 mm});
}
`
	mesh, err := evalMerged(context.Background(), parseTestProg(t, src), nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 10, 10, 40, 0.5)
}

// Region form derives the count to fill a length.
func TestEvalSolidLinearPatternRegion(t *testing.T) {
	src := `
fn Main() Solid {
    return Cube(s: 10 mm).LinearPattern(length: 40 mm, gap: 5 mm);
}
`
	mesh, err := evalMerged(context.Background(), parseTestProg(t, src), nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 40, 10, 10, 0.5)
}

func TestEvalLinearPatternTooSmall(t *testing.T) {
	src := `
fn Main() Solid {
    return Cube(s: 10 mm).LinearPattern(length: 5 mm);
}
`
	if msg := evalErr(t, src); !strings.Contains(msg, "LinearPattern: length too small to fit one copy") {
		t.Errorf("got: %s\nwant substring about length too small", msg)
	}
}

// Sketch linear along +Y.
func TestEvalSketchLinearPatternAxisY(t *testing.T) {
	src := `
fn Main() Solid {
    return Square(s: 10 mm).LinearPattern(count: 3, gap: 5 mm, axis: Vec2{y: 1 mm}).Extrude(z: 1 mm);
}
`
	mesh, err := evalMerged(context.Background(), parseTestProg(t, src), nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 10, 40, 1, 0.5)
}

// ── Circular center point ────────────────────────────────────────────────────

// Rotating a corner-anchored cube around (50,0) by 0/90/180/270° lays out a ring
// whose bbox spans 100×100 — proving the center offset is applied.
func TestEvalSolidCircularPatternCenter(t *testing.T) {
	src := `
fn Main() Solid {
    return Cube(s: 10 mm).CircularPattern(count: 4, center: Vec3{x: 50 mm, y: 0 mm});
}
`
	mesh, err := evalMerged(context.Background(), parseTestProg(t, src), nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 100, 100, 10, 1.0)
}

// ── Bundled example ──────────────────────────────────────────────────────────

func TestEvalAirFilterExample(t *testing.T) {
	src, err := examples.FS.ReadFile("Air Filter.fct")
	if err != nil {
		t.Fatalf("read example: %v", err)
	}
	mesh, err := evalMerged(context.Background(), parseTestProg(t, string(src)), nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil || len(mesh.Vertices) == 0 {
		t.Fatal("expected non-empty Air Filter mesh")
	}
}
