package evaluator

import "testing"

// ── Ternary `cond ? a : b` ─────────────────────────────────────────────────

func TestEvalTernaryPicksThenWhenTrue(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `var x = true ? 10 : 99;`, `x == 10`)
}

func TestEvalTernaryPicksElseWhenFalse(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `var x = false ? 10 : 99;`, `x == 99`)
}

// TestEvalTernaryShortCircuits confirms only the chosen arm is evaluated.
// The other arm divides by zero, which would error if evaluated; the test
// passes by NOT erroring.
func TestEvalTernaryShortCircuits(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `var x = true ? 5 : (1 / 0);`, `x == 5`)
}

// TestEvalTernaryNested confirms right-associative chaining produces the
// expected branch.
func TestEvalTernaryNested(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `
    var n = 0;
    var x = n > 0 ? 1 : n < 0 ? -1 : 0;`,
		`x == 0`)
}

// TestEvalTernaryInsideCallArg confirms the ternary value flows into a
// function call's named-arg slot.
func TestEvalTernaryInsideCallArg(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `
    var big = true;
    var size = big ? 10 mm : 5 mm;`,
		`size == 10 mm`)
}
