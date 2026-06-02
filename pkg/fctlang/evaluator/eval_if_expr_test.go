package evaluator

import (
	"context"
	"testing"
)

// evalMainSolidBBox evaluates a single-function program's Main and
// returns the merged mesh's axis-aligned bounding box as
// (xMax-xMin, yMax-yMin, zMax-zMin). Failures call t.Fatal.
//
// Many if-expression tests pick between two differently-sized cubes;
// observing the bbox extents is the simplest way to confirm the right
// arm fired without leaking into private evaluator state.
func evalMainSolidBBox(t *testing.T, src string) (dx, dy, dz float32) {
	t.Helper()
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil || len(mesh.Vertices) == 0 {
		t.Fatal("expected non-empty mesh")
	}
	xMin, yMin, zMin := float32(1e9), float32(1e9), float32(1e9)
	xMax, yMax, zMax := float32(-1e9), float32(-1e9), float32(-1e9)
	for i := 0; i < len(mesh.Vertices); i += 3 {
		x, y, z := mesh.Vertices[i], mesh.Vertices[i+1], mesh.Vertices[i+2]
		if x < xMin {
			xMin = x
		}
		if x > xMax {
			xMax = x
		}
		if y < yMin {
			yMin = y
		}
		if y > yMax {
			yMax = y
		}
		if z < zMin {
			zMin = z
		}
		if z > zMax {
			zMax = z
		}
	}
	return xMax - xMin, yMax - yMin, zMax - zMin
}

// approxBBox asserts a bounding-box extent is within 0.01 of expected
// (slop for f32 round-trip and any geometry-engine tolerance).
func approxBBox(t *testing.T, got, want float32, axis string) {
	t.Helper()
	d := got - want
	if d < 0 {
		d = -d
	}
	if d > 0.01 {
		t.Errorf("%s extent: got %.4f, want %.4f", axis, got, want)
	}
}

// TestEvalIfExpressionTrueArm picks the then-arm and confirms its value
// reached the surrounding expression. true → Cube(10mm) → 10x10x10 bbox.
func TestEvalIfExpressionTrueArm(t *testing.T) {
	src := `fn Main() Solid {
		return if 1 > 0 { Cube(s: 10 mm) } else { Cube(s: 20 mm) }
	}`
	dx, dy, dz := evalMainSolidBBox(t, src)
	approxBBox(t, dx, 10, "x")
	approxBBox(t, dy, 10, "y")
	approxBBox(t, dz, 10, "z")
}

// TestEvalIfExpressionFalseArm picks the else-arm. false → Cube(20mm).
func TestEvalIfExpressionFalseArm(t *testing.T) {
	src := `fn Main() Solid {
		return if 1 < 0 { Cube(s: 10 mm) } else { Cube(s: 20 mm) }
	}`
	dx, dy, dz := evalMainSolidBBox(t, src)
	approxBBox(t, dx, 20, "x")
	approxBBox(t, dy, 20, "y")
	approxBBox(t, dz, 20, "z")
}

// TestEvalIfExpressionElseIfPicksMiddleArm walks past the leading if
// and into the second clause. Confirms the middle else-if fires.
func TestEvalIfExpressionElseIfPicksMiddleArm(t *testing.T) {
	src := `fn Main() Solid {
		var x = 5;
		return if x > 10 { Cube(s: 1 mm) }
			else if x > 0 { Cube(s: 7 mm) }
			else { Cube(s: 99 mm) }
	}`
	dx, _, _ := evalMainSolidBBox(t, src)
	approxBBox(t, dx, 7, "x")
}

// TestEvalIfExpressionElseIfFallsThroughToFinalElse confirms the final
// else fires when no preceding cond matched.
func TestEvalIfExpressionElseIfFallsThroughToFinalElse(t *testing.T) {
	src := `fn Main() Solid {
		var x = -5;
		return if x > 10 { Cube(s: 1 mm) }
			else if x > 0 { Cube(s: 7 mm) }
			else { Cube(s: 3 mm) }
	}`
	dx, _, _ := evalMainSolidBBox(t, src)
	approxBBox(t, dx, 3, "x")
}

// TestEvalIfExpressionInsideStructField uses an if-expression as a
// field initializer — the parser must navigate the nested braces
// (outer Vec3 struct lit + inner if-arm braces) correctly, and the
// evaluator must propagate the value through.
func TestEvalIfExpressionInsideStructField(t *testing.T) {
	src := `fn Main() Solid {
		var big = 1 > 0;
		return Cube(s: Vec3{
			x: if big { 12 mm } else { 4 mm },
			y: 5 mm,
			z: 5 mm,
		})
	}`
	dx, dy, dz := evalMainSolidBBox(t, src)
	approxBBox(t, dx, 12, "x")
	approxBBox(t, dy, 5, "y")
	approxBBox(t, dz, 5, "z")
}

// TestEvalIfExpressionAsCallArg uses an if-expression as a named-arg
// value to a function call — confirms the value flows through Facet's
// named-arg mechanism.
func TestEvalIfExpressionAsCallArg(t *testing.T) {
	src := `fn Main() Solid {
		return Cube(s: if 1 > 0 { 8 mm } else { 2 mm })
	}`
	dx, _, _ := evalMainSolidBBox(t, src)
	approxBBox(t, dx, 8, "x")
}

// TestEvalIfExpressionNested confirms an if-expression inside an
// if-expression arm short-circuits correctly through both layers.
func TestEvalIfExpressionNested(t *testing.T) {
	src := `fn Main() Solid {
		var outer = 1 > 0;
		var inner = 1 < 0;
		return if outer {
			if inner { Cube(s: 3 mm) } else { Cube(s: 6 mm) }
		} else {
			Cube(s: 9 mm)
		}
	}`
	dx, _, _ := evalMainSolidBBox(t, src)
	approxBBox(t, dx, 6, "x")
}
