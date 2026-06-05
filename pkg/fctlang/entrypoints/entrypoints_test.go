package entrypoints

import (
	"context"
	"sort"
	"testing"

	"facet/pkg/fctlang/checker"
	"facet/pkg/fctlang/loader"
	"facet/pkg/fctlang/parser"
)

// TestEntryPointLessStrictWeakOrdering guards the bug flagged in the
// 2026-04-16 main-branch review (Important #15).  The naive predicate
// returned true for less("Main", "Main"), which violates strict weak
// ordering and can corrupt sort.Slice output when two sources both
// declare Main.
func TestEntryPointLessStrictWeakOrdering(t *testing.T) {
	// Irreflexivity: less(x, x) must be false for every x, including "Main".
	for _, name := range []string{"Main", "Foo", ""} {
		if entryPointLess(name, name) {
			t.Errorf("less(%q, %q) = true, want false (irreflexivity)", name, name)
		}
	}

	// Asymmetry: at most one of less(a,b), less(b,a) may be true.
	pairs := [][2]string{
		{"Main", "Foo"},
		{"Foo", "Bar"},
		{"Main", "Main"},
	}
	for _, p := range pairs {
		a, b := p[0], p[1]
		if entryPointLess(a, b) && entryPointLess(b, a) {
			t.Errorf("both less(%q,%q) and less(%q,%q) are true", a, b, b, a)
		}
	}

	// "Main" sorts first; everything else alphabetical.
	if !entryPointLess("Main", "Alpha") {
		t.Error("Main should sort before Alpha")
	}
	if entryPointLess("Alpha", "Main") {
		t.Error("Alpha should not sort before Main")
	}
	if !entryPointLess("Alpha", "Beta") {
		t.Error("Alpha should sort before Beta")
	}
}

// TestEntryPointSortWithDuplicateMain ensures sort.Slice produces a stable,
// non-panicking result when the input contains two "Main" entries.
func TestEntryPointSortWithDuplicateMain(t *testing.T) {
	names := []string{"Zeta", "Main", "Alpha", "Main", "Beta"}
	sort.Slice(names, func(i, j int) bool { return entryPointLess(names[i], names[j]) })
	want := []string{"Main", "Main", "Alpha", "Beta", "Zeta"}
	for i, n := range names {
		if n != want[i] {
			t.Errorf("sorted[%d] = %q, want %q (got %v)", i, n, want[i], names)
			break
		}
	}
}

// TestBuildMarksAnimation verifies that an Animation-returning entry appears in
// the entry-point list with Animated=true.
func TestBuildMarksAnimation(t *testing.T) {
	src := `fn Main() Animation {
    return Animation{frame: fn(t Number) Solid { return Cube(s: 10 mm) }}
}
`
	prog, err := loader.Load(context.Background(), src, "main.fct", parser.SourceUser, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	checked := checker.Check(prog)
	if len(checked.Errors) > 0 {
		t.Fatalf("check: %v", checked.Errors)
	}
	eps := Build(checked.Prog, checked.InferredReturnTypes)
	var main *EntryPoint
	for i := range eps {
		if eps[i].Name == "Main" {
			main = &eps[i]
		}
	}
	if main == nil {
		t.Fatal("Main not found as an entry point")
	}
	if !main.Animated {
		t.Fatal("expected Main.Animated == true for an Animation entry")
	}
}

// TestBuildConvertsDisplayUnits verifies a unit'd constrained parameter is
// reported in display units (the evaluator's convertOneOverride multiplies a
// bare override by the unit factor, so the UI must round-trip display units,
// not canonical mm).
func TestBuildConvertsDisplayUnits(t *testing.T) {
	src := `fn Main(width Length = 20 cm where [10:30] cm) Solid {
    return Cube(s: width)
}
`
	prog, err := loader.Load(context.Background(), src, "main.fct", parser.SourceUser, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	checked := checker.Check(prog)
	if len(checked.Errors) > 0 {
		t.Fatalf("check: %v", checked.Errors)
	}
	eps := Build(checked.Prog, checked.InferredReturnTypes)
	if len(eps) == 0 || len(eps[0].Params) == 0 {
		t.Fatal("expected one entry point with one parameter")
	}
	p := eps[0].Params[0]
	if p.Unit != "cm" {
		t.Fatalf("unit = %q, want cm", p.Unit)
	}
	if d, ok := p.Default.(float64); !ok || d < 19.9 || d > 20.1 {
		t.Fatalf("default = %v, want ~20 (cm, display units)", p.Default)
	}
	if p.Constraint == nil {
		t.Fatal("expected a range constraint")
	}
	if mx, ok := p.Constraint.Max.(float64); !ok || mx < 29.9 || mx > 30.1 {
		t.Fatalf("constraint max = %v, want ~30 (cm, display units)", p.Constraint.Max)
	}
}

// Mirrors the clock example: a Number parameter with a negative-range constraint
// on an Animation-returning entry. Without the constraint the param renders no
// slider in the function preview; with it, Build must extract the full [-12, 14]
// range — including the negative lower bound, which no other example exercises.
func TestBuildNegativeRangeConstraintOnAnimation(t *testing.T) {
	src := `fn Main(tzOffsetHours Number = 0 where [-12:14:1]) Animation {
    return Animation{frame: fn(t Number) Solid { return Cube(s: (10 + tzOffsetHours) * 1 mm) }}
}
`
	prog, err := loader.Load(context.Background(), src, "main.fct", parser.SourceUser, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	checked := checker.Check(prog)
	if len(checked.Errors) > 0 {
		t.Fatalf("check: %v", checked.Errors)
	}
	eps := Build(checked.Prog, checked.InferredReturnTypes)
	if len(eps) == 0 || len(eps[0].Params) == 0 {
		t.Fatal("expected one entry point with one parameter")
	}
	p := eps[0].Params[0]
	if p.Constraint == nil {
		t.Fatal("expected a range constraint (no constraint means no slider renders)")
	}
	if mn, ok := p.Constraint.Min.(float64); !ok || mn != -12 {
		t.Fatalf("constraint min = %v, want -12", p.Constraint.Min)
	}
	if mx, ok := p.Constraint.Max.(float64); !ok || mx != 14 {
		t.Fatalf("constraint max = %v, want 14", p.Constraint.Max)
	}
	if st, ok := p.Constraint.Step.(float64); !ok || st != 1 {
		t.Fatalf("constraint step = %v, want 1 (slider increments by 1 hour)", p.Constraint.Step)
	}
}
