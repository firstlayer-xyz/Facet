package scad

import (
	"strings"
	"testing"
)

// A function parameter that is a nested array (a list of points) reveals its
// nesting by being double-indexed, p[i][j]. The nested-parameter analysis must
// reach that double-index even when it appears only inside a list
// comprehension, or the parameter is mistyped as a flat []Number/Number instead
// of the dynamic Any that a nested array requires. This regression guards the
// list-comprehension descent in walkExprNodes.
func TestTranspileNestedParamDoubleIndexedInListComp(t *testing.T) {
	// m is only ever double-indexed (m[j][i]) inside nested list comprehensions.
	src := "function transpose(m) = [for (i = [0:len(m[0])-1]) [for (j = [0:len(m)-1]) m[j][i]]];\n" +
		"cols = transpose([[0, 0], [10, 0], [10, 10]]);\n" +
		"for (c = cols) translate([c[0], c[1], 0]) cube(1);\n"

	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("transpose should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "fn transpose(m Any)") {
		t.Fatalf("m is a nested array double-indexed inside a comprehension; expected it typed Any, got:\n%s", res.Facet)
	}
}
