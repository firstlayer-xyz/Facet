package checker

import "testing"

// TestCheckNullCoalesceProducesInner confirms `T? ?? T` typechecks as T.
func TestCheckNullCoalesceProducesInner(t *testing.T) {
	expectNoErrors(t, `
fn Maybe() Number? { return 5 }
fn Main() Number { return Maybe() ?? 0 }
`)
}

// TestCheckNullCoalesceRejectsNonOptionalLeft confirms ?? requires the
// left operand to be Optional.
func TestCheckNullCoalesceRejectsNonOptionalLeft(t *testing.T) {
	expectError(t, `
fn Main() Number { return 5 ?? 0 }
`, "must be Optional")
}

// TestCheckNullCoalesceFallbackTypeMatches confirms the fallback's type
// must match the inner type.
func TestCheckNullCoalesceFallbackTypeMatches(t *testing.T) {
	expectError(t, `
fn Maybe() Number? { return 5 }
fn Main() Number { return Maybe() ?? "no" }
`, "fallback must be Number")
}

// TestCheckOptionalChainingFieldLiftsType confirms `opt?.field` returns
// the field's type wrapped in `?`.
func TestCheckOptionalChainingFieldLiftsType(t *testing.T) {
	expectNoErrors(t, `
fn MaybeVec() Vec3? { return Vec3{x: 1 mm, y: 2 mm, z: 3 mm} }
fn Main() Length { return MaybeVec()?.x ?? 0 mm }
`)
}

// TestCheckOptionalChainingRejectsNonOptionalReceiver confirms `?.` on a
// non-Optional receiver errors.
func TestCheckOptionalChainingRejectsNonOptionalReceiver(t *testing.T) {
	expectError(t, `
fn Main() Number { var v = Vec3{x: 1 mm, y: 2 mm, z: 3 mm}; return Number(from: v?.x) }
`, "Optional receiver")
}

// TestCheckIfVarBindNarrows confirms inside `if var x = opt { ... }`,
// x is narrowed to the inner type (so passing it where Number is
// expected works).
func TestCheckIfVarBindNarrows(t *testing.T) {
	expectNoErrors(t, `
fn Maybe() Number? { return 5 }
fn Main() Number {
    if var x = Maybe() { return x }
    return 0
}
`)
}

// TestCheckIfVarBindRejectsNonOptional confirms `if var x = expr` errors
// when expr isn't Optional.
func TestCheckIfVarBindRejectsNonOptional(t *testing.T) {
	expectError(t, `
fn Main() Number {
    if var x = 5 { return x }
    return 0
}
`, "must be Optional")
}

// TestCheckForYieldOverOptionalProducesOptional confirms `for v opt { ... }`
// types as U? — Optional treated as a 0-or-1 element collection.
func TestCheckForYieldOverOptionalProducesOptional(t *testing.T) {
	expectNoErrors(t, `
fn Maybe() Number? { return 5 }
fn Main() Number {
    var doubled = for n Maybe() { yield n * 2 };
    return doubled ?? 0
}
`)
}
