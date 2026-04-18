package evaluator

import (
	"context"
	"testing"
)

func TestEvalSquareExtrude(t *testing.T) {
	src := `
fn Main() {
    var p = Square(x: 10 mm, y: 10 mm);
    return p.Extrude(z: 5 mm);
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
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalCircleRevolve(t *testing.T) {
	src := `
fn Main() {
    var c = Circle(r: 5 mm);
    var moved = c.Move(v: Vec2 { x: 10 mm, y: 0 mm });
    return moved.Revolve();
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
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalMoveSolid(t *testing.T) {
	src := `
fn Main() {
    var box = Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    return box.Move(v: Vec3 { x: 5 mm, y: 5 mm, z: 5 mm });
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
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalRotateSolid(t *testing.T) {
	src := `
fn Main() {
    var box = Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    return box.Rotate(x: 45 deg, y: 0 deg, z: 0 deg, around: Vec3{});
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
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalSketchMoveExtrude(t *testing.T) {
	src := `
fn Main() {
    var p = Square(x: 10 mm, y: 10 mm);
    var moved = p.Move(v: Vec2 { x: 5 mm, y: 5 mm });
    return moved.Extrude(z: 10 mm);
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
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalHullSolid(t *testing.T) {
	src := `
fn Main() {
    var a = Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    var b = Sphere(r: 5 mm).Move(v: Vec3 { x: 20 mm, y: 0 mm, z: 0 mm });
    return Hull(arr: [a, b]);
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
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalBatchHullSolid(t *testing.T) {
	src := `
fn Main() {
    var a = Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    var b = Cube(s: Vec3{x: 5 mm, y: 5 mm, z: 5 mm}).Move(v: Vec3 { x: 20 mm, y: 0 mm, z: 0 mm });
    return Hull(arr: [a, b]);
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
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalBatchHullSketch(t *testing.T) {
	src := `
fn Main() {
    var a = Square(x: 10 mm, y: 10 mm);
    var b = Square(x: 5 mm, y: 5 mm).Move(v: Vec2 { x: 20 mm, y: 0 mm });
    return Hull(arr: [a, b]).Extrude(z: 5 mm);
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
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalScaleMirror(t *testing.T) {
	src := `
fn Main() {
    var box = Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    var scaled = box.Scale(x: 2, y: 1, z: 1, around: Vec3{});
    var mirrored = scaled.Mirror(x: 1);
    return scaled + mirrored;
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
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalExtrudeWithTwist(t *testing.T) {
	src := `
fn Main() {
    var p = Square(x: 10 mm, y: 10 mm);
    return p.Extrude(z: 20 mm, slices: 20, twist: 90 deg, taperX: 1, taperY: 1);
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
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalRevolvePartial(t *testing.T) {
	src := `
fn Main() {
    var c = Circle(r: 3 mm);
    var moved = c.Move(v: Vec2 { x: 10 mm, y: 0 mm });
    return moved.Revolve(a: 180 deg);
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
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalRefine(t *testing.T) {
	src := `
fn Main() {
    var box = Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    return box.Refine(n: 2);
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
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalMethodChaining(t *testing.T) {
	src := `
fn Main() {
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).Move(v: Vec3 { x: 5 mm, y: 0 mm, z: 0 mm }).Rotate(x: 0 deg, y: 0 deg, z: 45 deg, around: Vec3{});
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
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}


func TestEvalMethodOnParenBoolean(t *testing.T) {
	src := `
fn Main() {
    var a = Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    var b = Sphere(r: 8 mm);
    return Hull(arr: [a + b]);
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
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

func TestEvalSketchChain(t *testing.T) {
	src := `
fn Main() {
    return Circle(r: 5 mm).Move(v: Vec2 { x: 10 mm, y: 0 mm }).Revolve();
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
	if len(mesh.Vertices) == 0 {
		t.Error("expected non-empty vertices")
	}
}

// TestEvalStructReceiverMutationPersists checks whether a void receiver method
// that modifies self actually mutates the struct instance at the call site.
func TestEvalStructReceiverMutationPersists(t *testing.T) {
	src := `
type Counter {
    n Number;
}

fn Counter.Inc() Counter {
    self.n = self.n + 1;
    return self;
}

fn Main() {
    var c = Counter { n: 10 };
    var c = c.Inc();
    # If mutation persists, c.n == 11 → cube is 11mm wide.
    # If not, c.n == 10 → cube is 10mm wide.
    return Cube(s: Vec3{x: c.n * 1 mm, y: 1 mm, z: 1 mm});
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
	if len(mesh.Vertices) == 0 {
		t.Fatal("expected non-empty vertices")
	}

	// Find max X vertex to determine cube width.
	var maxX float32
	for i := 0; i < len(mesh.Vertices); i += 3 {
		if mesh.Vertices[i] > maxX {
			maxX = mesh.Vertices[i]
		}
	}
	t.Logf("max X vertex = %f (expect 11 if mutation persists, 10 if not)", maxX)
	if maxX < 10.5 {
		t.Error("struct mutation in receiver method did not persist: expected cube width ~11, got ~10")
	}
}
