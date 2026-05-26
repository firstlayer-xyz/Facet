package evaluator

import (
	"context"
	"testing"
)

// All Sketch.* face-coordinate accessors are tested against a 20×20mm
// square placed at the origin: bounds are (0,0)..(20,20) in xy.

const stdlibTestSquare20 = `var s = Square(s: 20 mm);`

// ── Sketch.* face-coordinate accessors ────────────────────────────────────────

func TestEvalSketchLeft(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, stdlibTestSquare20, `s.Left() == 0 mm`)
}

func TestEvalSketchRight(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, stdlibTestSquare20, `s.Right() == 20 mm`)
}

func TestEvalSketchFront(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, stdlibTestSquare20, `s.Front() == 0 mm`)
}

func TestEvalSketchBack(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, stdlibTestSquare20, `s.Back() == 20 mm`)
}

// ── Sketch.MoveTo ─────────────────────────────────────────────────────────────

func TestEvalSketchMoveToVec2(t *testing.T) {
	// MoveTo positions the bounds min corner at pos.
	stdlibIfThenCubeWithSetup(t, `
    var s = Square(s: 10 mm).MoveTo(pos: Vec2{x: 5 mm, y: 7 mm});`,
		`s.Left() == 5 mm && s.Front() == 7 mm`)
}

func TestEvalSketchMoveToXY(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `
    var s = Square(s: 10 mm).MoveTo(x: 3 mm, y: 4 mm);`,
		`s.Left() == 3 mm && s.Front() == 4 mm`)
}

// ── Sketch.Offset ─────────────────────────────────────────────────────────────

func TestEvalSketchOffset(t *testing.T) {
	// Offset by 2mm grows a square by 4mm in each axis (2mm each side).
	// Verify by extruding and checking the resulting solid's footprint.
	src := `
fn Main() Solid {
    return Square(s: 10 mm).Offset(delta: 2 mm).Extrude(z: 5 mm);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	minX, minY, _, maxX, maxY, _ := meshBounds(mesh)
	width := maxX - minX
	depth := maxY - minY
	// 10mm square + 2mm offset on each side = 14mm wide.
	if width < 13.5 || width > 14.5 {
		t.Errorf("Offset width: got %f, want ~14", width)
	}
	if depth < 13.5 || depth > 14.5 {
		t.Errorf("Offset depth: got %f, want ~14", depth)
	}
}

// ── Sketch.Sweep — extrude a 2D profile along a 3D path ───────────────────────

func TestEvalSketchSweep(t *testing.T) {
	// Sweep a small square along a straight line of length 30mm.
	// The result should occupy roughly that much length along the path.
	src := `
fn Main() Solid {
    var path = [
        Vec3{x: 0 mm, y: 0 mm, z: 0 mm},
        Vec3{x: 30 mm, y: 0 mm, z: 0 mm},
    ];
    return Square(s: 5 mm).Sweep(path: path);
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil || len(mesh.Vertices) == 0 {
		t.Fatal("expected non-empty swept mesh")
	}
	minX, _, _, maxX, _, _ := meshBounds(mesh)
	length := maxX - minX
	if length < 25 || length > 35 {
		t.Errorf("Sweep x-extent: got %f, want ~30", length)
	}
}
