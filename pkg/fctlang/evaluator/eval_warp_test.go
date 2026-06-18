package evaluator

import (
	"context"
	"testing"
)

// The _warp builtin reuses a single scratch Vec3 + arg map across all vertices
// (safe because warpMu serializes callbacks and callFunctionVal deep-copies the
// argument before the body runs). These tests guard that reuse: if the scratch
// were aliased or left stale between vertices, the per-vertex transform would
// collapse and the resulting bounds would be wrong.

// A per-axis scale must apply independently to every vertex. With a 10mm cube
// and x*2, the result is a clean 20×10×10 box only if each vertex sees its own
// coordinates — a stale/aliased scratch would smear every vertex to one point.
func TestEvalWarpScalesEachVertexIndependently(t *testing.T) {
	src := `
fn Main() Solid {
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).Warp(f: fn(p Vec3) Vec3 {
        return Vec3{x: p.x * 2, y: p.y, z: p.z}
    });
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 20, 10, 10, 0.01)
}

// The identity warp must leave geometry untouched.
func TestEvalWarpIdentity(t *testing.T) {
	src := `
fn Main() Solid {
    return Cube(s: Vec3{x: 10 mm, y: 8 mm, z: 6 mm}).Warp(f: fn(p Vec3) Vec3 { return p });
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 10, 8, 6, 0.01)
}

// A warp whose callback reads a captured local and a global function exercises
// the full callFunctionVal path (the same one the benchmark stresses). Adding a
// constant offset per axis shifts the bbox but preserves its size.
func TestEvalWarpWithCapturedAndGlobalRefs(t *testing.T) {
	src := `
fn Bump(n Number) Number { return n + 5 }

fn Main() Solid {
    var shift = 3.0;
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}).Warp(f: fn(p Vec3) Vec3 {
        return Vec3{
            x: (Number(from: p.x) + shift) mm,
            y: Bump(n: Number(from: p.y)) mm,
            z: p.z,
        }
    });
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	// Pure translation per axis → size is unchanged, still a 10mm cube.
	assertMeshSize(t, mesh, 10, 10, 10, 0.01)
	minX, minY, _, _, _, _ := meshBounds(mesh)
	if minX < 2.99 || minX > 3.01 {
		t.Errorf("x shifted by captured `shift`: min x = %.3f, want ~3", minX)
	}
	if minY < 4.99 || minY > 5.01 {
		t.Errorf("y shifted by global Bump: min y = %.3f, want ~5", minY)
	}
}
