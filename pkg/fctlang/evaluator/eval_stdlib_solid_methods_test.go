package evaluator

import (
	"context"
	"testing"
)

// ── Solid.MoveTo ──────────────────────────────────────────────────────────────

func TestEvalSolidMoveToVec3(t *testing.T) {
	// MoveTo positions the bounding-box min corner at pos.
	stdlibIfThenCubeWithSetup(t, `
    var c = Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).MoveTo(pos: Vec3{x: 5 mm, y: 7 mm, z: 9 mm});`,
		`c.LeftFrontBottom() == Vec3{x: 5 mm, y: 7 mm, z: 9 mm}`)
}

func TestEvalSolidMoveToXYZ(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `
    var c = Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).MoveTo(x: 3 mm, y: 4 mm, z: 5 mm);`,
		`c.LeftFrontBottom() == Vec3{x: 3 mm, y: 4 mm, z: 5 mm}`)
}

// ── Solid.Align*  (single-arg position form) ──────────────────────────────────

func TestEvalSolidAlignLeftPos(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `
    var c = Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).AlignLeft(pos: 50 mm);`,
		`c.Left() == 50 mm`)
}

func TestEvalSolidAlignRightPos(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `
    var c = Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).AlignRight(pos: 50 mm);`,
		`c.Right() == 50 mm`)
}

func TestEvalSolidAlignFrontPos(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `
    var c = Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).AlignFront(pos: 50 mm);`,
		`c.Front() == 50 mm`)
}

func TestEvalSolidAlignBackPos(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `
    var c = Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).AlignBack(pos: 50 mm);`,
		`c.Back() == 50 mm`)
}

func TestEvalSolidAlignBottomPos(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `
    var c = Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).AlignBottom(pos: 50 mm);`,
		`c.Bottom() == 50 mm`)
}

func TestEvalSolidAlignTopPos(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `
    var c = Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).AlignTop(pos: 50 mm);`,
		`c.Top() == 50 mm`)
}

// ── Solid.Stack* — places one solid touching a face of another ────────────────

func TestEvalSolidStackOnTop(t *testing.T) {
	// Stack a 10mm cube on top of a 20mm cube. Result min.z of top cube = 20mm.
	stdlibIfThenCubeWithSetup(t, `
    var base = Cube(s: Vec3{x: 20 mm, y: 20 mm, z: 20 mm});
    var top = Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).StackOnTop(of: base);`,
		`top.Bottom() == 20 mm`)
}

func TestEvalSolidStackOnBottom(t *testing.T) {
	// Stack underneath: top of stacked == bottom of base (= 0 for cube at origin).
	stdlibIfThenCubeWithSetup(t, `
    var base = Cube(s: Vec3{x: 20 mm, y: 20 mm, z: 20 mm});
    var below = Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).StackOnBottom(of: base);`,
		`below.Top() == 0 mm`)
}

func TestEvalSolidStackOnRight(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `
    var base = Cube(s: Vec3{x: 20 mm, y: 20 mm, z: 20 mm});
    var r = Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).StackOnRight(of: base);`,
		`r.Left() == 20 mm`)
}

func TestEvalSolidStackOnLeft(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `
    var base = Cube(s: Vec3{x: 20 mm, y: 20 mm, z: 20 mm});
    var l = Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).StackOnLeft(of: base);`,
		`l.Right() == 0 mm`)
}

func TestEvalSolidStackOnBack(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `
    var base = Cube(s: Vec3{x: 20 mm, y: 20 mm, z: 20 mm});
    var b = Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).StackOnBack(of: base);`,
		`b.Front() == 20 mm`)
}

func TestEvalSolidStackOnFront(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `
    var base = Cube(s: Vec3{x: 20 mm, y: 20 mm, z: 20 mm});
    var f = Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).StackOnFront(of: base);`,
		`f.Back() == 0 mm`)
}

// ── Solid.Insert / Solid.Exclude (method form; operator forms tested elsewhere)

func TestEvalSolidInsertMethod(t *testing.T) {
	// Insert is the underlying operation for the `|` operator. With a non-
	// overlapping probe, the bounding box should grow.
	src := `
fn Main() Solid {
    var box = Cube(s: Vec3{x: 20 mm, y: 20 mm, z: 20 mm});
    var probe = Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).Move(v: Vec3{x: 25 mm, y: 0 mm, z: 0 mm});
    return box.Insert(part: probe);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	minX, _, _, maxX, _, _ := meshBounds(mesh)
	if minX > 0.1 || maxX < 34.9 {
		t.Errorf("Insert union bounds wrong: x=[%f, %f] want [~0, ~35]", minX, maxX)
	}
}

func TestEvalSolidExcludeMethod(t *testing.T) {
	// Exclude is the underlying operation for the `^` operator.
	src := `
fn Main() Solid {
    var box = Cube(s: Vec3{x: 20 mm, y: 20 mm, z: 20 mm});
    var probe = Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).Move(v: Vec3{x: 5 mm, y: 5 mm, z: 5 mm});
    return box.Exclude(with: probe);
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

// ── Solid.Resize ──────────────────────────────────────────────────────────────

func TestEvalSolidResize(t *testing.T) {
	// Resize to a target dimension regardless of current size.
	src := `
fn Main() Solid {
    return Cube(s: Vec3{x: 5 mm, y: 5 mm, z: 5 mm}).Resize(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.1)
}

// ── Solid.Trim — chops material on the side of an axis-aligned plane ──────────

func TestEvalSolidTrim(t *testing.T) {
	// Trim with x: 1, offset: 10mm — keeps material below the x=10 plane,
	// so width should be 10mm (half of the original 20mm cube).
	src := `
fn Main() Solid {
    return Cube(s: Vec3{x: 20 mm, y: 20 mm, z: 20 mm}).Trim(x: 1, offset: 10 mm);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	minX, _, _, maxX, _, _ := meshBounds(mesh)
	width := maxX - minX
	if width < 9.5 || width > 10.5 {
		t.Errorf("Trim width: got %f, want ~10", width)
	}
}

// ── Solid.Genus ───────────────────────────────────────────────────────────────

func TestEvalSolidGenus(t *testing.T) {
	// A simple cube has genus 0 (sphere topology — no holes).
	stdlibIfThenCubeWithSetup(t,
		`var c = Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});`,
		`c.Genus() == 0`)
}

// ── Solid.MinGap ──────────────────────────────────────────────────────────────

func TestEvalSolidMinGap(t *testing.T) {
	// Two cubes 5mm apart — closest gap should be 5mm.
	stdlibIfThenCubeWithSetup(t, `
    var a = Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    var b = Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).Move(v: Vec3{x: 15 mm, y: 0 mm, z: 0 mm});
    var gap = a.MinGap(with: b, reach: 20 mm);`,
		`gap > 4.5 mm && gap < 5.5 mm`)
}

// ── Solid.RefineToLength + Simplify + Smooth ──────────────────────────────────
// These are mesh-shape transforms — we just verify they run and return a
// non-empty solid with roughly the original bounds.

func TestEvalSolidRefineToLength(t *testing.T) {
	src := `
fn Main() Solid {
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).RefineToLength(edgeLength: 2 mm);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.1)
}

func TestEvalSolidSimplify(t *testing.T) {
	src := `
fn Main() Solid {
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).Simplify(tolerance: 0.01 mm);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.1)
}

func TestEvalSolidSmooth(t *testing.T) {
	// Smooth a refined cube. We don't assert exact bounds — smoothing
	// rounds corners — just that a non-empty result is produced.
	src := `
fn Main() Solid {
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).RefineToLength(edgeLength: 2 mm).Smooth(minSharpAngle: 30 deg, minSmoothness: 0.5);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil || len(mesh.Vertices) == 0 {
		t.Fatal("expected non-empty smoothed mesh")
	}
}

// ── Solid.Split — yields []Solid ──────────────────────────────────────────────

func TestEvalSolidSplit(t *testing.T) {
	// A cutter that doesn't overlap the body should leave it as a single
	// component.
	stdlibIfThenCubeWithSetup(t, `
    var c = Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    var cutter = Cube(s: Vec3{x: 1 mm, y: 1 mm, z: 1 mm}).Move(v: Vec3{x: 100 mm, y: 0 mm, z: 0 mm});
    var pieces = c.Split(cutter: cutter);`,
		`Size(of: pieces) >= 1`)
}

// ── Solid.PolyMesh — convert to a poly mesh representation ────────────────────

func TestEvalSolidPolyMesh(t *testing.T) {
	// Round-trip: cube → polymesh → solid should preserve bounds.
	src := `
fn Main() Solid {
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).PolyMesh().Solid();
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.5)
}
