package scad

import (
	"strings"
	"testing"
)

// OpenSCAD concat() wraps-and-flattens: concat(1, 2) is [1, 2], not 3. Facet maps
// it to list `+`, which is correct for lists but would do numeric/broadcast `+`
// for scalars. A provably-scalar literal arg is untranslatable and must error
// rather than emit a silently-wrong value.
func TestTranspileConcatScalarLiteralErrors(t *testing.T) {
	// Lists concatenate fine.
	res, err := Transpile("x = concat([1, 2], [3, 4]);\ncube([x[0], 1, 1]);\n", "part.scad")
	if err != nil {
		t.Fatalf("concat of lists should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "[1, 2] + [3, 4]") {
		t.Fatalf("expected list concat in output:\n%s", res.Facet)
	}

	// A scalar literal arg is untranslatable.
	for _, src := range []string{
		"x = concat(1, 2);\ncube([1, 1, 1]);\n",
		"x = concat([1, 2], 3);\ncube([1, 1, 1]);\n",
	} {
		if _, err := Transpile(src, "part.scad"); err == nil {
			t.Fatalf("expected an error for concat with a scalar literal: %q", src)
		}
	}
}
