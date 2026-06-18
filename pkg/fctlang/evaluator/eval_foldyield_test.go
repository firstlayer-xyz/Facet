package evaluator

import "testing"

// A for-yield over an Optional nested inside a fold must collect its body's yields
// into its own result, not the enclosing fold's accumulator. A yield inside a
// nested block (here `if n > 0`) is dispatched through blockYield, which writes to
// foldAcc when it is live. evalForYieldOptional must clear foldAcc for the loop's
// extent (as evalForYieldArray does); otherwise the inner yield leaks into the
// fold and the loop silently returns None.
//
// With the bug, `inner` is None, so `inner ?? -1` is -1 and `r == 5` fails.
func TestEvalForYieldOptionalInsideFoldDoesNotLeak(t *testing.T) {
	stdlibIfThenCubeWithSetup(t, `
        var maybe = fn() Number? { return 5 };
        var r = fold acc, x [1, 2] {
            var inner = for n maybe() { if n > 0 { yield n } };
            yield (inner ?? -1)
        };`,
		`r == 5`)
}
