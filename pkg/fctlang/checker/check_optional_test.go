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

// TestCheckOptionalOrReturnsInner confirms .Or(default:) yields the inner
// type, so the result can be used where the inner type is required.
func TestCheckOptionalOrReturnsInner(t *testing.T) {
	expectNoErrors(t, `
fn Maybe() Number? { return 5 }
fn Main() Number { return Maybe().Or(default: 0) }
`)
}

// TestCheckOptionalOrDefaultTypeMustMatch confirms the default's type is
// checked against the inner type.
func TestCheckOptionalOrDefaultTypeMustMatch(t *testing.T) {
	expectError(t, `
fn Maybe() Number? { return 5 }
fn Main() Number { return Maybe().Or(default: "hi") }
`, "Or() default must be Number")
}

// TestCheckOptionalIsSomeReturnsBool confirms .IsSome() and .IsNone()
// return Bool — usable in if conditions.
func TestCheckOptionalIsSomeReturnsBool(t *testing.T) {
	expectNoErrors(t, `
fn Maybe() Number? { return 5 }
fn Main() Number {
    if Maybe().IsSome() { return 1 }
    return 0
}
`)
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

// TestCheckOptionalRejectsUnknownMethod confirms a typo on an Optional
// surfaces the available-methods hint.
func TestCheckOptionalRejectsUnknownMethod(t *testing.T) {
	expectError(t, `
fn Maybe() Number? { return 5 }
fn Main() Number { return Maybe().Unwrap() }
`, "no method")
}

// (Double-nesting `Number??` is rejected at the parser layer — see
// TestParseDoubleOptionalRejected in the parser_test package — so the
// checker never sees it.)
