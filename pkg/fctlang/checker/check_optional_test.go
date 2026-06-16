package checker

import "testing"

// TestCheckOptionalAcceptsDefiniteAtReturn confirms T → T? widening at
// the return-type boundary — `return 5` in a `Number?` function works.
func TestCheckOptionalAcceptsDefiniteAtReturn(t *testing.T) {
	expectNoErrors(t, `fn Lookup() Number? { return 5 }`)
}

// TestCheckOptionalAcceptsNilAtReturn confirms `return nil` works for any T?.
func TestCheckOptionalAcceptsNilAtReturn(t *testing.T) {
	expectNoErrors(t, `fn Lookup() Number? { return nil }`)
}

// TestCheckOptionalRejectsNarrowing pins the core safety guarantee: a
// `T?` value can NOT be implicitly used where `T` is expected.
func TestCheckOptionalRejectsNarrowing(t *testing.T) {
	expectError(t, `
fn Maybe() Number? { return 5 }
fn Main() Number { return Maybe() }
`, "returns Number?")
}

// TestCheckTernaryPreservesOptional pins the soundness fix: when either arm of
// a ternary is optional the result is optional, so `cond ? T : T?` types as T?
// (not T) and can't be used where T is required.
func TestCheckTernaryPreservesOptional(t *testing.T) {
	expectError(t, `
fn Maybe() Number? { return 5 }
fn Main() Number { return true ? 5 : Maybe() }
`, "Number?")
	expectNoErrors(t, `
fn Maybe() Number? { return 5 }
fn Pick() Number? { return true ? 5 : Maybe() }
`)
}

// TestCheckOptionalFallbackReturnsInner confirms `opt ?? default` yields
// the inner type, so the result can be used where the inner type is required.
func TestCheckOptionalFallbackReturnsInner(t *testing.T) {
	expectNoErrors(t, `
fn Maybe() Number? { return 5 }
fn Main() Number { return Maybe() ?? 0 }
`)
}

// TestCheckOptionalFallbackDefaultTypeMustMatch confirms the default's type
// is checked against the inner type.
func TestCheckOptionalFallbackDefaultTypeMustMatch(t *testing.T) {
	expectError(t, `
fn Maybe() Number? { return 5 }
fn Main() Number { return Maybe() ?? "hi" }
`, "fallback")
}

// TestCheckOptionalEqualsNil confirms `opt == nil` typechecks as Bool.
func TestCheckOptionalEqualsNil(t *testing.T) {
	expectNoErrors(t, `
fn Maybe() Number? { return 5 }
fn Main() Number {
    if Maybe() == nil { return 0 }
    return 1
}
`)
}

// TestCheckOptionalHasNoMethods locks in that Optional values have no
// method surface — every method call on a T? is a checker error pointing
// the user at ??, ==/!= nil, or `if var x = opt`.
func TestCheckOptionalHasNoMethods(t *testing.T) {
	expectError(t, `
fn Maybe() Number? { return 5 }
fn Main() Number { return Maybe().Unwrap() }
`, "no methods")
}

// (Double-nesting `Number??` is rejected at the parser layer — see
// TestParseDoubleOptionalRejected in the parser_test package — so the
// checker never sees it.)

// ── Optional-parameter contract ───────────────────────────────────────────
// An Optional-typed param with no default is omittable (binds None); a
// non-optional param with no default stays mandatory. These pin both
// directions of Param.IsRequired so neither can silently flip.

// TestCheckOptionalParamOmittable confirms a `T?` param can be omitted.
func TestCheckOptionalParamOmittable(t *testing.T) {
	expectNoErrors(t, `
fn Foo(x Number?) Number { return x ?? 0 }
fn Main() Number { return Foo() }
`)
}

// TestCheckRequiredParamStillRequired confirms a non-optional param with no
// default still errors when omitted — IsRequired must not treat it as optional
// (if it did, required would be 0 and Foo() would type-check).
func TestCheckRequiredParamStillRequired(t *testing.T) {
	expectError(t, `
fn Foo(x Number) Number { return x }
fn Main() Number { return Foo() }
`, "expects 1 arguments, got 0")
}

// TestCheckOptionalStructFieldTypesAsOptional pins the fix from unifying the
// checker's type resolvers: a struct field declared `T?` is typed as Optional on
// access, so the narrowing safety applies. The former field-access resolver
// dropped the `?` and mistyped the field as Any — `return b.gap` below would
// have type-checked, silently discarding the optional guarantee.
func TestCheckOptionalStructFieldTypesAsOptional(t *testing.T) {
	expectError(t, `
type Box {
    gap Length?
}
fn Main() Length {
    var b = Box{gap: 5 mm}
    return b.gap
}
`, "Length?")
}
