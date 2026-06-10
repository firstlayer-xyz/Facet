package evaluator

import (
	"context"
	"strings"
	"testing"
)

// Deep-review regressions: struct value semantics. Structs copy at every
// binding boundary; the genuinely-aliased receivers (array elements, module
// globals) reject field assignment instead.

// A struct argument binds a copy: a callee field assignment is local to the
// parameter and never reaches the caller's variable.
func TestEvalParamMutationStaysLocal(t *testing.T) {
	src := `
fn Tweak(v Vec3) Number {
    v.x = 99 mm;
    return 0;
}
fn Main() {
    var p = Vec3{x: 5 mm, y: 1 mm, z: 1 mm};
    var unused = Tweak(v: p);
    return Cube(s: Vec3{x: p.x, y: 1 mm, z: 1 mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 5, 1, 1, 0.1)
}

// Mutating a module-level struct from inside a function is rejected (it
// reaches the function by reference — action at a distance).
func TestEvalGlobalStructMutationRejected(t *testing.T) {
	src := `
var cfg = Vec3{x: 1 mm, y: 1 mm, z: 1 mm};
fn Poke() Number {
    cfg.x = 99 mm;
    return 0;
}
fn Main() {
    var unused = Poke();
    return Cube(s: cfg);
}
`
	prog := parseTestProg(t, src)
	_, err := evalMerged(context.Background(), prog, nil)
	if err == nil || !strings.Contains(err.Error(), "module-level") {
		t.Fatalf("expected a module-level mutation error, got: %v", err)
	}
}

// Field assignment through an array element is rejected — the backing slice
// is shared across bindings, so the write would co-mutate every copy.
func TestEvalArrayElementMutationRejected(t *testing.T) {
	src := `
fn Main() {
    var pts = [Vec3{x: 1 mm, y: 1 mm, z: 1 mm}];
    pts[0].x = 99 mm;
    return Cube(s: pts[0]);
}
`
	prog := parseTestProg(t, src)
	_, err := evalMerged(context.Background(), prog, nil)
	if err == nil || !strings.Contains(err.Error(), "array element") {
		t.Fatalf("expected an array-element mutation error, got: %v", err)
	}
}

// The loop variable binds a copy: transforming it in the body must not
// rewrite the source array.
func TestEvalLoopVarMutationDoesNotRewriteSource(t *testing.T) {
	src := `
fn Main() {
    var pts = [Vec3{x: 1 mm, y: 1 mm, z: 1 mm}];
    var moved = for p pts {
        p.x = p.x + 10 mm;
        yield p;
    };
    // moved[0].x == 11mm, while the source pts[0].x stays 1mm.
    return Cube(s: Vec3{x: moved[0].x - pts[0].x + 1 mm, y: 1 mm, z: 1 mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	// 11 - 1 + 1 = 11. (If the source were rewritten: 11 - 11 + 1 = 1.)
	assertMeshSize(t, mesh, 11, 1, 1, 0.1)
}

// Deep const: a field assignment through a nested field of a const binding is
// as much a mutation of the const as a direct one.
func TestEvalDeepConstMutationRejected(t *testing.T) {
	src := `
type Outer {
    inner Vec3;
}
fn Main() {
    const cfg = Outer{inner: Vec3{x: 1 mm, y: 1 mm, z: 1 mm}};
    cfg.inner.x = 99 mm;
    return Cube(s: cfg.inner);
}
`
	prog := parseTestProg(t, src)
	_, err := evalMerged(context.Background(), prog, nil)
	if err == nil || !strings.Contains(err.Error(), "const") {
		t.Fatalf("expected a const mutation error, got: %v", err)
	}
}

// Anonymous-struct coercion must not mutate the input: a rejected overload
// trial used to stamp the target type and its defaults onto the caller's
// value, making overload resolution declaration-order dependent.
func TestEvalAnonymousCoercionLeavesInputUntouched(t *testing.T) {
	src := `
type TCfg {
    x Length;
    pad Length = 7 mm;
}
fn UseCfg(c TCfg) Length {
    return c.pad;
}
fn Main() {
    var s = {x: 1 mm};
    var pad = UseCfg(c: s);
    // s must still be the bare anonymous {x: 1mm}: passing it to a SECOND
    // type boundary keeps working, and it gained no pad field.
    var pad2 = UseCfg(c: s);
    return Cube(s: Vec3{x: pad + pad2 - 13 mm, y: 1 mm, z: 1 mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	// 7 + 7 - 13 = 1.
	assertMeshSize(t, mesh, 1, 1, 1, 0.1)
}

// Each lambda call gets fresh copies of the captured snapshot: a body field
// assignment must not persist into the next invocation (Animation frames
// depend on call purity).
func TestEvalLambdaCapturePerCallPurity(t *testing.T) {
	src := `
type Counter {
    n Number;
}
fn Main() {
    var c = Counter{n: 1};
    var bump = fn() Number {
        c.n = c.n + 1;
        return c.n;
    };
    var a = bump();
    var b = bump();
    // Both calls see the pristine capture: a == b == 2.
    return Cube(s: Vec3{x: (a + b) * 1 mm, y: 1 mm, z: 1 mm});
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	assertMeshSize(t, mesh, 4, 1, 1, 0.1)
}
