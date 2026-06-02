package evaluator

import "testing"

// ── ?? (nullish coalescing) ────────────────────────────────────────────────

func TestEvalNullCoalesceUsesFallbackWhenAbsent(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `
    var maybe = fn() Number? { return nil };
    var x = maybe() ?? 99;`,
		`x == 99`)
}

func TestEvalNullCoalesceUsesValueWhenPresent(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `
    var maybe = fn() Number? { return 7 };
    var x = maybe() ?? 99;`,
		`x == 7`)
}

func TestEvalNullCoalesceShortCircuits(t *testing.T) {
	// If left is Some, right is never evaluated — verified by parking a
	// division by zero on the right side that would error if executed.
	stdlibIfThenCubeWithSetup(t, `
    var maybe = fn() Number? { return 5 };
    var x = maybe() ?? (1 / 0);`,
		`x == 5`)
}

// ── ?. (optional chaining, field) ──────────────────────────────────────────

func TestEvalOptionalChainFieldPresent(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `
    var v = fn() Vec3? { return Vec3{x: 5 mm, y: 0 mm, z: 0 mm} };
    var width = v()?.x;`,
		`width != nil`)
}

func TestEvalOptionalChainFieldNoneShortCircuits(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `
    var v = fn() Vec3? { return nil };
    var width = v()?.x;`,
		`width == nil`)
}

// ── for-yield over Optional ────────────────────────────────────────────────

func TestEvalForYieldOptionalMap(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `
    var maybe = fn() Number? { return 5 };
    var doubled = for n maybe() { yield n * 2 };`,
		`(doubled ?? 0) == 10`)
}

func TestEvalForYieldOptionalNoneStaysNone(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `
    var maybe = fn() Number? { return nil };
    var doubled = for n maybe() { yield n * 2 };`,
		`doubled == nil`)
}

func TestEvalForYieldOptionalFilter(t *testing.T) {
	// Conditional yield: keep only if predicate holds.
	stdlibIfThenCubeWithSetup(t, `
    var maybe = fn() Number? { return 5 };
    var positive = for n maybe() { if n > 0 { yield n } };`,
		`(positive ?? -1) == 5`)
}

func TestEvalForYieldOptionalFilterDropsToNone(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `
    var maybe = fn() Number? { return -3 };
    var positive = for n maybe() { if n > 0 { yield n } };`,
		`positive == nil`)
}

// ── if var i = opt ─────────────────────────────────────────────────────────

func TestEvalIfVarBindEntersWhenPresent(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `
    var maybe = fn() Number? { return 5 };
    var x = 0;
    if var v = maybe() { x = v * 2 };`,
		`x == 10`)
}

func TestEvalIfVarBindSkipsWhenAbsent(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `
    var maybe = fn() Number? { return nil };
    var x = 7;
    if var v = maybe() { x = v * 2 };`,
		`x == 7`)
}

// TestEvalIfVarBindFallsThroughToElse pairs the bind form with an else
// branch and verifies the else runs when the Optional is None.
func TestEvalIfVarBindFallsThroughToElse(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `
    var maybe = fn() Number? { return nil };
    var x = 0;
    if var v = maybe() { x = v } else { x = 42 };`,
		`x == 42`)
}

// TestEvalOptionalChainMethodPresent exercises the Some path through
// dispatchMethodCall: receiver unwrapped, method dispatched on the inner
// value, result re-wrapped as Some.
func TestEvalOptionalChainMethodPresent(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `
    var s = fn() String? { return "hello" };
    var contains = s()?.Contains(substr: "ell") ?? false;`,
		`contains == true`)
}

// TestEvalForYieldOptionalMultiYieldErrors confirms the runtime guard:
// an Optional can hold at most one value, so a body that yields twice
// during the single Some iteration is a hard error rather than silently
// taking the first or last.
func TestEvalForYieldOptionalMultiYieldErrors(t *testing.T) {
	src := `
fn Main() Solid {
    var maybe = fn() Number? { return 5 };
    var bad = for n maybe() { yield n; yield n * 2 };
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm})
}
`
	prog := parseTestProg(t, src)
	_, err := Eval(t.Context(), prog, testMainKey, nil, "Main")
	if err == nil {
		t.Fatal("expected error from for-yield over Optional with two yields")
	}
}
