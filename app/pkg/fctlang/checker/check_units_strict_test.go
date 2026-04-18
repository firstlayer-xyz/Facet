package checker

import "testing"

// Checker-level tests for the strict Length/Number rules. The checker must
// reject dimension-incompatible expressions at compile time, independent of
// whether the evaluator would also reject them at runtime.
//
// Rule set (see plans/2026-04-16-units-strict.md):
//   • Length + Length = Length          ✓
//   • Length - Length = Length          ✓
//   • Length * Number = Length          ✓
//   • Number * Length = Length          ✓
//   • Length / Number = Length          ✓
//   • Length / Length = Number          ✓ (dimensionless ratio)
//   • Length * Length = ERROR           (no Area type)
//   • Length % Length = ERROR
//
// Mixed Length/Number for +,-,% :
//   • Bare numeric literals are "untyped" and coerce to Length
//     (e.g. `5 mm + 3` → 8 mm).
//   • Committed Number variables/expressions do NOT coerce and error
//     (e.g. `var n = 3; 5 mm + n` is an error).

// --- Valid expressions ------------------------------------------------------

func TestCheckLengthPlusLength(t *testing.T) {
	expectNoErrors(t, `fn Main() { var x = 5 mm + 3 mm; return Cube(s: Vec3{x: x, y: x, z: x}) }`)
}

func TestCheckLengthMinusLength(t *testing.T) {
	expectNoErrors(t, `fn Main() { var x = 5 mm - 3 mm; return Cube(s: Vec3{x: x, y: x, z: x}) }`)
}

func TestCheckLengthTimesNumber(t *testing.T) {
	expectNoErrors(t, `fn Main() { var x = 5 mm * 2; return Cube(s: Vec3{x: x, y: x, z: x}) }`)
}

func TestCheckNumberTimesLength(t *testing.T) {
	expectNoErrors(t, `fn Main() { var x = 2 * 5 mm; return Cube(s: Vec3{x: x, y: x, z: x}) }`)
}

func TestCheckLengthDivNumber(t *testing.T) {
	expectNoErrors(t, `fn Main() { var x = 10 mm / 2; return Cube(s: Vec3{x: x, y: x, z: x}) }`)
}

func TestCheckLengthDivLengthIsNumber(t *testing.T) {
	// Ratio of two Lengths is a dimensionless Number; pass to Lerp.t (Number).
	expectNoErrors(t, `
fn Main() {
    var ratio = 10 mm / 2 mm
    var y = Lerp(from: 0, to: 10, t: ratio)
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm})
}
`)
}

func TestCheckExplicitNumberConversion(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    var n = Number(from: 5 mm)
    var y = Lerp(from: 0, to: 10, t: n)
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm})
}
`)
}

// --- Literal coercion -------------------------------------------------------

func TestCheckLengthPlusNumberLiteralIsLength(t *testing.T) {
	// Bare numeric literal on the Number side coerces to Length.
	expectNoErrors(t, `fn Main() { var x = 5 mm + 3; return Cube(s: Vec3{x: x, y: x, z: x}) }`)
}

func TestCheckLengthMinusNumberLiteralIsLength(t *testing.T) {
	expectNoErrors(t, `fn Main() { var x = 5 mm - 3; return Cube(s: Vec3{x: x, y: x, z: x}) }`)
}

func TestCheckLengthModNumberLiteralIsLength(t *testing.T) {
	expectNoErrors(t, `fn Main() { var x = 5 mm % 3; return Cube(s: Vec3{x: x, y: x, z: x}) }`)
}

func TestCheckNumberLiteralPlusLengthIsLength(t *testing.T) {
	expectNoErrors(t, `fn Main() { var x = 3 + 5 mm; return Cube(s: Vec3{x: x, y: x, z: x}) }`)
}

func TestCheckNegativeLiteralPlusLengthIsLength(t *testing.T) {
	// Unary-minus applied to a NumberLit still counts as a literal.
	expectNoErrors(t, `fn Main() { var x = 5 mm + -3; return Cube(s: Vec3{x: x, y: x, z: x}) }`)
}

// --- Invalid expressions ---------------------------------------------------

func TestCheckLengthPlusNumberVariableIsError(t *testing.T) {
	// A committed Number variable does NOT coerce — this is the distinction
	// from the literal case above.
	expectError(t, `
fn Main() {
    var n = 3
    var x = 5 mm + n
    return Cube(s: Vec3{x: x, y: x, z: x})
}
`, "incompatible types Length and Number")
}

func TestCheckLengthMinusNumberVariableIsError(t *testing.T) {
	expectError(t, `
fn Main() {
    var n = 3
    var x = 5 mm - n
    return Cube(s: Vec3{x: x, y: x, z: x})
}
`, "incompatible types Length and Number")
}

func TestCheckLengthModNumberVariableIsError(t *testing.T) {
	expectError(t, `
fn Main() {
    var n = 3
    var x = 5 mm % n
    return Cube(s: Vec3{x: x, y: x, z: x})
}
`, "incompatible types Length and Number")
}

func TestCheckNumberVariablePlusLengthIsError(t *testing.T) {
	expectError(t, `
fn Main() {
    var n = 3
    var x = n + 5 mm
    return Cube(s: Vec3{x: x, y: x, z: x})
}
`, "incompatible types Number and Length")
}

func TestCheckLengthTimesLengthIsError(t *testing.T) {
	expectError(t,
		`fn Main() { var x = 5 mm * 3 mm; return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}) }`,
		"Length * Length")
}

func TestCheckLengthModLengthIsError(t *testing.T) {
	expectError(t,
		`fn Main() { var x = 5 mm % 3 mm; return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm}) }`,
		"Length % Length")
}
