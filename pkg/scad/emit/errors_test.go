package emit

import "testing"

func TestErrorListSortsAndFormats(t *testing.T) {
	errs := []TranspileError{
		{Feature: "module 'minkowski'", Line: 5, Col: 1},
		{Feature: "expr *ast.Undef", Line: 2, Col: 9},
	}
	err := ErrorList("part.scad", errs)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	want := "scad: cannot translate part.scad:\n  2:9: expr *ast.Undef\n  5:1: module 'minkowski'"
	if err.Error() != want {
		t.Fatalf("got:\n%s\nwant:\n%s", err.Error(), want)
	}
}

func TestErrorListEmptyIsNil(t *testing.T) {
	if err := ErrorList("x.scad", nil); err != nil {
		t.Fatalf("expected nil for empty errors, got %v", err)
	}
}

func TestErrorListSortsByColumnOnSameLine(t *testing.T) {
	errs := []TranspileError{
		{Feature: "b", Line: 3, Col: 10},
		{Feature: "a", Line: 3, Col: 2},
	}
	got := ErrorList("x.scad", errs).Error()
	want := "scad: cannot translate x.scad:\n  3:2: a\n  3:10: b"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}
