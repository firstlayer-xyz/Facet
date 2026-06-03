package evaluator

import "testing"

// ── IndexOf — first index as Number?, nil if absent ─────────────────────────

func TestEvalIndexOfPresent(t *testing.T) {
	stdlibIfThenCubeWithSetup(t,
		`var i = IndexOf(arr: [10, 20, 30, 40], value: 30);`,
		`(i ?? -1) == 2`)
}

func TestEvalIndexOfFirstOccurrenceWins(t *testing.T) {
	stdlibIfThenCubeWithSetup(t,
		`var i = IndexOf(arr: [1, 2, 1, 3, 1], value: 1);`,
		`(i ?? -1) == 0`)
}

func TestEvalIndexOfAbsent(t *testing.T) {
	stdlibIfThenCubeWithSetup(t,
		`var i = IndexOf(arr: [10, 20, 30], value: 99);`,
		`i == nil`)
}

func TestEvalIndexOfAbsentBindSkips(t *testing.T) {
	// `if var` over an absent IndexOf does not bind — the else branch runs.
	stdlibIfThenCubeWithSetup(t,
		`var found = false; if var i = IndexOf(arr: [10, 20, 30], value: 99) { found = true }`,
		`found == false`)
}

func TestEvalIndexOfStringArray(t *testing.T) {
	stdlibIfThenCubeWithSetup(t,
		`var i = IndexOf(arr: ["a", "b", "c"], value: "c");`,
		`(i ?? -1) == 2`)
}

// ── IndicesOf — all matching indices ───────────────────────────────────────

func TestEvalIndicesOfMultiMatch(t *testing.T) {
	stdlibIfThenCubeWithSetup(t,
		`var ix = IndicesOf(arr: [1, 2, 1, 3, 1], value: 1);`,
		`Size(of: ix) == 3 && ix[0] == 0 && ix[1] == 2 && ix[2] == 4`)
}

func TestEvalIndicesOfNoMatch(t *testing.T) {
	stdlibIfThenCubeWithSetup(t,
		`var ix = IndicesOf(arr: [10, 20, 30], value: 99);`,
		`Size(of: ix) == 0`)
}

func TestEvalIndicesOfSingleMatch(t *testing.T) {
	stdlibIfThenCubeWithSetup(t,
		`var ix = IndicesOf(arr: [10, 20, 30], value: 20);`,
		`Size(of: ix) == 1 && ix[0] == 1`)
}

// ── FindIndex — first match by predicate ───────────────────────────────────

func TestEvalFindIndexPresent(t *testing.T) {
	stdlibIfThenCubeWithSetup(t,
		`var i = FindIndex(arr: [1, 5, 9, 2], pred: fn(n Any) Bool { return n > 3 });`,
		`(i ?? -1) == 1`)
}

func TestEvalFindIndexAbsent(t *testing.T) {
	stdlibIfThenCubeWithSetup(t,
		`var i = FindIndex(arr: [1, 2, 3], pred: fn(n Any) Bool { return n > 99 });`,
		`i == nil`)
}

// ── FindIndices — all matches by predicate ─────────────────────────────────

func TestEvalFindIndicesMultiMatch(t *testing.T) {
	stdlibIfThenCubeWithSetup(t,
		`var ix = FindIndices(arr: [1, 5, 9, 2, 7], pred: fn(n Any) Bool { return n > 3 });`,
		`Size(of: ix) == 3 && ix[0] == 1 && ix[1] == 2 && ix[2] == 4`)
}

func TestEvalFindIndicesNoMatch(t *testing.T) {
	stdlibIfThenCubeWithSetup(t,
		`var ix = FindIndices(arr: [1, 2, 3], pred: fn(n Any) Bool { return n > 99 });`,
		`Size(of: ix) == 0`)
}
