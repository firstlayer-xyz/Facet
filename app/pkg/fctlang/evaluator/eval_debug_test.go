package evaluator

import (
	"context"
	"testing"
)

func TestEvalDebugSteps(t *testing.T) {
	src := `
fn Main() {
    var box = Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    return box.Translate(v: Vec3{x: 5 mm, y: 0 mm, z: 0 mm});
}
`
	prog := parseTestProg(t, src)
	result, err := EvalDebug(context.Background(), prog, testMainKey, nil, "Main")
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if len(result.Solids) == 0 {
		t.Fatal("expected non-empty solids")
	}
	if len(result.Steps) != 2 {
		t.Fatalf("expected 2 steps (Cube + Translate), got %d", len(result.Steps))
	}

	// Step 0: Cube constructor
	if result.Steps[0].Op != "Cube" {
		t.Errorf("step 0: expected op Cube, got %s", result.Steps[0].Op)
	}
	meshes0 := result.ResolveMeshes(0)
	if len(meshes0) != 1 {
		t.Fatalf("step 0: expected 1 mesh, got %d", len(meshes0))
	}
	if meshes0[0].Role != "result" {
		t.Errorf("step 0: expected role result, got %s", meshes0[0].Role)
	}

	// Step 1: Translate transform
	if result.Steps[1].Op != "Translate" {
		t.Errorf("step 1: expected op Translate, got %s", result.Steps[1].Op)
	}
	meshes1 := result.ResolveMeshes(1)
	if len(meshes1) != 2 {
		t.Fatalf("step 1: expected 2 meshes (input + result), got %d", len(meshes1))
	}
	roles := map[string]bool{}
	for _, m := range meshes1 {
		roles[m.Role] = true
		if m.Mesh == nil || m.Mesh.VertexCount == 0 {
			t.Errorf("step 1: mesh for role %s is empty", m.Role)
		}
	}
	if !roles["input"] || !roles["result"] {
		t.Errorf("step 1: expected roles input and result, got %v", roles)
	}
}

func TestEvalDebugBooleanOps(t *testing.T) {
	src := `
fn Main() {
    return Cube(size: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}) - Sphere(radius: 8 mm);
}
`
	prog := parseTestProg(t, src)
	result, err := EvalDebug(context.Background(), prog, testMainKey, nil, "Main")
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if len(result.Solids) == 0 {
		t.Fatal("expected non-empty solids")
	}
	// Steps: Cube, Sphere, Difference
	if len(result.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(result.Steps))
	}

	if result.Steps[2].Op != "Difference" {
		t.Errorf("step 2: expected op Difference, got %s", result.Steps[2].Op)
	}
	meshes2 := result.ResolveMeshes(2)
	if len(meshes2) != 3 {
		t.Fatalf("step 2: expected 3 meshes (lhs + rhs + result), got %d", len(meshes2))
	}
	roles := map[string]bool{}
	for _, m := range meshes2 {
		roles[m.Role] = true
		if m.Mesh == nil || m.Mesh.VertexCount == 0 {
			t.Errorf("step 2: mesh for role %s is empty", m.Role)
		}
	}
	if !roles["lhs"] || !roles["rhs"] || !roles["result"] {
		t.Errorf("step 2: expected roles lhs, rhs, result — got %v", roles)
	}
}

func TestEvalDebugSketchSteps(t *testing.T) {
	src := `
fn Main() {
    var sq = Square(x: 10 mm, y: 10 mm);
    var circ = Circle(radius: 3 mm);
    var profile = sq - circ;
    return profile.Extrude(height: 5 mm);
}
`
	prog := parseTestProg(t, src)
	result, err := EvalDebug(context.Background(), prog, testMainKey, nil, "Main")
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if len(result.Solids) == 0 {
		t.Fatal("expected non-empty solids")
	}

	// Expected steps: Square, Circle, Difference (sketch), Extrude
	if len(result.Steps) != 4 {
		var ops []string
		for _, s := range result.Steps {
			ops = append(ops, s.Op)
		}
		t.Fatalf("expected 4 steps (Square, Circle, Difference, Extrude), got %d: %v", len(result.Steps), ops)
	}

	// Step 0: Square
	if result.Steps[0].Op != "Square" {
		t.Errorf("step 0: expected op Square, got %s", result.Steps[0].Op)
	}
	meshes0 := result.ResolveMeshes(0)
	if len(meshes0) != 1 {
		t.Fatalf("step 0: expected 1 mesh, got %d", len(meshes0))
	}
	if meshes0[0].Mesh == nil || meshes0[0].Mesh.VertexCount == 0 {
		t.Error("step 0: sketch mesh is empty")
	}

	// Step 1: Circle
	if result.Steps[1].Op != "Circle" {
		t.Errorf("step 1: expected op Circle, got %s", result.Steps[1].Op)
	}
	meshes1 := result.ResolveMeshes(1)
	if len(meshes1) != 1 {
		t.Fatalf("step 1: expected 1 mesh, got %d", len(meshes1))
	}

	// Step 2: Difference (sketch boolean)
	if result.Steps[2].Op != "Difference" {
		t.Errorf("step 2: expected op Difference, got %s", result.Steps[2].Op)
	}
	meshes2 := result.ResolveMeshes(2)
	if len(meshes2) != 3 {
		t.Fatalf("step 2: expected 3 meshes (lhs + rhs + result), got %d", len(meshes2))
	}
	roles := map[string]bool{}
	for _, m := range meshes2 {
		roles[m.Role] = true
		if m.Mesh == nil || m.Mesh.VertexCount == 0 {
			t.Errorf("step 2: mesh for role %s is empty", m.Role)
		}
	}
	if !roles["lhs"] || !roles["rhs"] || !roles["result"] {
		t.Errorf("step 2: expected roles lhs, rhs, result — got %v", roles)
	}

	// Step 3: Extrude (produces a solid from sketch)
	if result.Steps[3].Op != "Extrude" {
		t.Errorf("step 3: expected op Extrude, got %s", result.Steps[3].Op)
	}
	meshes3 := result.ResolveMeshes(3)
	if len(meshes3) < 1 {
		t.Fatalf("step 3: expected at least 1 mesh, got %d", len(meshes3))
	}
}
