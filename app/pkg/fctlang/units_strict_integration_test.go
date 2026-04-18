package fctlang

import (
	"context"
	fctchecker "facet/app/pkg/fctlang/checker"
	"facet/app/pkg/fctlang/evaluator"
	"facet/app/pkg/fctlang/parser"
	"testing"
	"time"
)

// checkErrList is the concrete return type of checker.Result.Errors.
type checkErrList = []parser.SourceError


// End-to-end integration tests for the strict Length/Number type rules.
// These exercise the full parser → checker → evaluator pipeline on .fct source.
//
// The rule set (see plans/2026-04-16-units-strict.md):
//   • Length + Length = Length          (compatible dimensions)
//   • Length - Length = Length
//   • Length * Number = Length          (scale)
//   • Number * Length = Length          (commutative)
//   • Length / Number = Length          (scale)
//   • Length / Length = Number          (dimensionless ratio)
//   • Length * Length = ERROR           (no Area type)
//   • Length + Number = ERROR when the Number side is a committed value
//                       (variable, call result, etc.)
//   • Length + Number = Length when the Number side is a bare numeric literal
//                       — literals are "untyped" and coerce to Length.
//   • Same rules for -, %.
//   • Length → Number requires explicit Number(x) call
//   • Number → Length auto-coerces at boundaries (arg passing, var decl, return)
//
// Until Tasks 2–6 land, the error tests should *fail* here (they document
// desired behavior). The positive tests may or may not pass depending on the
// current state — they lock in the passing semantics after the fix.

// runCheckAndEval runs the checker and then the evaluator on a .fct source.
// Returns any errors from either stage.
func runCheckAndEval(t *testing.T, src string) (checkErrs checkErrList, evalErr error) {
	t.Helper()
	prog := parseTestProg(t, src)
	checkErrs = fctchecker.Check(prog).Errors
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, evalErr = evaluator.Eval(ctx, prog, testMainKey, nil, "Main")
	return checkErrs, evalErr
}

// --- Positive tests: these must pass once the refactor is complete ----------

func TestIntegrationLengthTimesNumber(t *testing.T) {
	src := `
fn Main() Solid {
    var w = 5 mm * 2
    return Cube(s: Vec3{x: w, y: w, z: w})
}
`
	checkErrs, err := runCheckAndEval(t, src)
	for _, ce := range checkErrs {
		t.Errorf("[check] %d:%d: %s", ce.Line, ce.Col, ce.Message)
	}
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
}

func TestIntegrationNumberTimesLength(t *testing.T) {
	src := `
fn Main() Solid {
    var w = 2 * 5 mm
    return Cube(s: Vec3{x: w, y: w, z: w})
}
`
	checkErrs, err := runCheckAndEval(t, src)
	for _, ce := range checkErrs {
		t.Errorf("[check] %d:%d: %s", ce.Line, ce.Col, ce.Message)
	}
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
}

func TestIntegrationLengthDivNumber(t *testing.T) {
	src := `
fn Main() Solid {
    var w = 10 mm / 2
    return Cube(s: Vec3{x: w, y: w, z: w})
}
`
	checkErrs, err := runCheckAndEval(t, src)
	for _, ce := range checkErrs {
		t.Errorf("[check] %d:%d: %s", ce.Line, ce.Col, ce.Message)
	}
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
}

// TestIntegrationLengthDivLengthIsNumber: a ratio of two Lengths is a
// dimensionless Number and can be fed to a Number-typed parameter.
func TestIntegrationLengthDivLengthIsNumber(t *testing.T) {
	src := `
fn Main() Solid {
    // 10mm / 2mm = 5 (Number). Pass to Lerp.t (Number parameter).
    var ratio = 10 mm / 2 mm
    var y = Lerp(from: 0, to: 10, t: ratio)
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm})
}
`
	checkErrs, err := runCheckAndEval(t, src)
	for _, ce := range checkErrs {
		t.Errorf("[check] %d:%d: %s", ce.Line, ce.Col, ce.Message)
	}
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
}

// TestIntegrationExplicitNumberConversion: Number(x) is the sanctioned way to
// strip units from a Length and obtain a bare Number.
func TestIntegrationExplicitNumberConversion(t *testing.T) {
	src := `
fn Main() Solid {
    var n = Number(from: 5 mm)
    var y = Lerp(from: 0, to: 10, t: n)
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm})
}
`
	checkErrs, err := runCheckAndEval(t, src)
	for _, ce := range checkErrs {
		t.Errorf("[check] %d:%d: %s", ce.Line, ce.Col, ce.Message)
	}
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
}

// --- Negative tests: these must produce a checker OR evaluator error --------

// TestIntegrationRatioLiteralIsLength: `20/2 mm` (no spaces around `/`) is a
// single numeric literal — the tokenizer reads `20/2` as one Number token
// (value 10) and `mm` attaches as the unit suffix → 10 mm. This must keep
// working even under strict units.
func TestIntegrationRatioLiteralIsLength(t *testing.T) {
	src := `
fn Main() Solid {
    var x = 20/2 mm
    return Cube(s: Vec3{x: x, y: x, z: x})
}
`
	checkErrs, err := runCheckAndEval(t, src)
	for _, ce := range checkErrs {
		t.Errorf("[check] %d:%d: %s", ce.Line, ce.Col, ce.Message)
	}
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
}

// TestIntegrationDivideExprIsError: with whitespace around `/`, `20 / 2 mm`
// parses as `20 / (2 mm)` = Number / Length, which is a dimension error under
// strict units. Whitespace distinguishes literal-ratio from division.
func TestIntegrationDivideExprIsError(t *testing.T) {
	src := `
fn Main() Solid {
    var x = 20 / 2 mm
    return Cube(s: Vec3{x: x, y: x, z: x})
}
`
	checkErrs, evalErr := runCheckAndEval(t, src)
	if len(checkErrs) == 0 && evalErr == nil {
		t.Fatal("expected an error for `20 / 2 mm` (Number / Length)")
	}
}

// TestIntegrationLengthPlusNumberLiteralIsLength: a bare numeric literal
// coerces to Length, so `5 mm + 3` is 8 mm and must not error.
func TestIntegrationLengthPlusNumberLiteralIsLength(t *testing.T) {
	src := `
fn Main() Solid {
    var w = 5 mm + 3
    return Cube(s: Vec3{x: w, y: w, z: w})
}
`
	checkErrs, evalErr := runCheckAndEval(t, src)
	if len(checkErrs) != 0 {
		t.Fatalf("unexpected checker errors: %v", checkErrs)
	}
	if evalErr != nil {
		t.Fatalf("unexpected eval error: %v", evalErr)
	}
}

// TestIntegrationErrorLengthPlusNumberVariable: a committed Number variable
// does NOT coerce — mixing it with a Length is a dimension error.
func TestIntegrationErrorLengthPlusNumberVariable(t *testing.T) {
	src := `
fn Main() Solid {
    var n = 3
    var w = 5 mm + n
    return Cube(s: Vec3{x: w, y: w, z: w})
}
`
	checkErrs, evalErr := runCheckAndEval(t, src)
	if len(checkErrs) == 0 && evalErr == nil {
		t.Fatal("expected an error from checker or evaluator for Length + Number variable")
	}
}

// TestIntegrationErrorLengthMinusNumberVariable: same rule for subtraction.
func TestIntegrationErrorLengthMinusNumberVariable(t *testing.T) {
	src := `
fn Main() Solid {
    var n = 3
    var w = 5 mm - n
    return Cube(s: Vec3{x: w, y: w, z: w})
}
`
	checkErrs, evalErr := runCheckAndEval(t, src)
	if len(checkErrs) == 0 && evalErr == nil {
		t.Fatal("expected an error from checker or evaluator for Length - Number variable")
	}
}

// TestIntegrationErrorLengthTimesLength: there is no Area type, so multiplying
// two Lengths must be a type error rather than silently producing a Number.
func TestIntegrationErrorLengthTimesLength(t *testing.T) {
	src := `
fn Main() Solid {
    var a = 5 mm * 3 mm
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm})
}
`
	checkErrs, evalErr := runCheckAndEval(t, src)
	if len(checkErrs) == 0 && evalErr == nil {
		t.Fatal("expected an error for Length * Length (no Area type)")
	}
}

// TestIntegrationErrorLengthModLength: modulo of two Lengths is a dimension
// error under strict units.
func TestIntegrationErrorLengthModLength(t *testing.T) {
	src := `
fn Main() Solid {
    var a = 5 mm % 3 mm
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm})
}
`
	checkErrs, evalErr := runCheckAndEval(t, src)
	if len(checkErrs) == 0 && evalErr == nil {
		t.Fatal("expected an error for Length %% Length")
	}
}

// TestIntegrationErrorNumberParamRejectsLength: a builtin whose parameter is
// typed Number must reject a Length argument. Lerp's `t` parameter is Number.
func TestIntegrationErrorNumberParamRejectsLength(t *testing.T) {
	src := `
fn Main() Solid {
    var x = Lerp(from: 0, to: 10, t: 5 mm)
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm})
}
`
	checkErrs, evalErr := runCheckAndEval(t, src)
	if len(checkErrs) == 0 && evalErr == nil {
		t.Fatal("expected an error: Lerp(t:) requires Number, got Length")
	}
}

// TestIntegrationErrorLengthReturnedAsNumber: a function declared to return
// Number must not silently accept a Length return value.
func TestIntegrationErrorLengthReturnedAsNumber(t *testing.T) {
	src := `
fn scale() Number {
    return 5 mm
}

fn Main() Solid {
    var n = scale()
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm})
}
`
	checkErrs, evalErr := runCheckAndEval(t, src)
	if len(checkErrs) == 0 && evalErr == nil {
		t.Fatal("expected an error: function returns Number but expression is Length")
	}
}

// TestIntegrationSplitByPlaneWithVec3: regression test. Solid.SplitByPlane
// takes a Vec3 normal whose components are Length-typed. The underlying
// _split_plane builtin wants Number components. Under strict units, the
// stdlib wrapper must strip units via Number(from: ...) explicitly —
// otherwise requireNumber rejects the Length.
func TestIntegrationSplitByPlaneWithVec3(t *testing.T) {
	src := `
fn Main() Solid {
    var parts = Cube(s: 20 mm)
        .SplitByPlane(normal: Vec3{x: 1, y: 0, z: 0}, offset: 10 mm)
    return parts[0]
}
`
	checkErrs, err := runCheckAndEval(t, src)
	for _, ce := range checkErrs {
		t.Errorf("[check] %d:%d: %s", ce.Line, ce.Col, ce.Message)
	}
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
}
