package checker

import (
	"strings"
	"testing"
)

// TestValidateEntryPoint_AcceptsSolid covers the ordinary happy path.
func TestValidateEntryPoint_AcceptsSolid(t *testing.T) {
	const src = `
fn Main() Solid {
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm})
}
`
	prog := parseTestProg(t, src)
	result := Check(prog)
	if err := result.ValidateEntryPoint(testMainKey, "Main"); err != nil {
		t.Errorf("expected no error, got %s", err.Message)
	}
}

// TestValidateEntryPoint_AcceptsSolidArray covers the list-of-solids entry.
func TestValidateEntryPoint_AcceptsSolidArray(t *testing.T) {
	const src = `
fn Main() []Solid {
    return [Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm})]
}
`
	prog := parseTestProg(t, src)
	result := Check(prog)
	if err := result.ValidateEntryPoint(testMainKey, "Main"); err != nil {
		t.Errorf("expected no error, got %s", err.Message)
	}
}

// TestValidateEntryPoint_RejectsNumber is the target case from the review:
// a function declared to return Number cannot be an entry point.
func TestValidateEntryPoint_RejectsNumber(t *testing.T) {
	const src = `
fn Main() Number {
    return 42
}
`
	prog := parseTestProg(t, src)
	result := Check(prog)
	err := result.ValidateEntryPoint(testMainKey, "Main")
	if err == nil {
		t.Fatal("expected error for Number-returning entry point")
	}
	if !strings.Contains(err.Message, "Main()") || !strings.Contains(err.Message, "Number") {
		t.Errorf("message should mention Main and Number, got %q", err.Message)
	}
}

// TestValidateEntryPoint_NameIsNotSpecial verifies the check is applied by
// name, not by convention. Main() returning Number is only rejected when
// Main is the requested entry. Any other function can still return Number.
func TestValidateEntryPoint_NameIsNotSpecial(t *testing.T) {
	const src = `
fn Main() Number {
    return 42
}

fn Shape() Solid {
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm})
}
`
	prog := parseTestProg(t, src)
	result := Check(prog)
	if err := result.ValidateEntryPoint(testMainKey, "Shape"); err != nil {
		t.Errorf("Shape() is a valid entry, but got error: %s", err.Message)
	}
	if err := result.ValidateEntryPoint(testMainKey, "Main"); err == nil {
		t.Error("Main() returns Number, should have been rejected when used as entry")
	}
}

// TestValidateEntryPoint_InfersUnannotatedReturn exercises the fallback to
// the inferred-return-types map when the function has no declared return.
func TestValidateEntryPoint_InfersUnannotatedReturn(t *testing.T) {
	const src = `
fn Main() {
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm})
}
`
	prog := parseTestProg(t, src)
	result := Check(prog)
	if err := result.ValidateEntryPoint(testMainKey, "Main"); err != nil {
		t.Errorf("unannotated entry that returns Solid should be accepted, got %s", err.Message)
	}
}

// TestValidateEntryPoint_UnknownEntryReturnsNil guarantees we do not invent
// errors for missing functions — that case is the evaluator's "no such
// function" error and has its own message.
func TestValidateEntryPoint_UnknownEntryReturnsNil(t *testing.T) {
	const src = `
fn Main() Solid {
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm})
}
`
	prog := parseTestProg(t, src)
	result := Check(prog)
	if err := result.ValidateEntryPoint(testMainKey, "DoesNotExist"); err != nil {
		t.Errorf("missing entry should return nil, got %s", err.Message)
	}
}
