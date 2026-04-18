package main

import (
	"sort"
	"testing"
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
