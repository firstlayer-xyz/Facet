package scad

import (
	"strings"
	"testing"
)

// TestBOSL2DistributorSpacingPrecedence guards the string-splice precedence fix:
// a compound spacing/radius expression must be parenthesized where it is
// multiplied, subtracted, or negated, or it silently mis-groups. e.g.
// `xcopies(w+g)` copy i must sit at (i-(n-1)/2)*(w+g), not (i-(n-1)/2)*w + g.
func TestBOSL2DistributorSpacingPrecedence(t *testing.T) {
	cases := []struct {
		name   string
		src    string
		want   string // the correctly-grouped substring
		reject string // the mis-grouped form that must NOT appear
	}{
		{
			"xcopies additive spacing",
			"module m(w, g) { xcopies(w + g, n = 3) cube(1); } m(4, 2);",
			"* (w + g)", "* w + g",
		},
		{
			"grid_copies additive spacing",
			"module m(w, g) { grid_copies(spacing = [w + g, 5], n = [2, 2]) cube(1); } m(4, 2);",
			"* (w + g)", "* w + g",
		},
		{
			"line_copies additive spacing",
			"module m(w, g) { line_copies(spacing = [w + g, 0, 0], n = 3) cube(1); } m(4, 2);",
			"* (w + g)", "* w + g",
		},
		{
			// All-symbolic radii so the expression stays Number-domain and
			// type-checks; a literal r_maj mixed with a symbolic r_min is a
			// separate Length/Number issue, not a precedence one.
			"torus additive minor radius",
			"module m(c, a, b) { torus(r_maj = c, r_min = a + b); } m(20, 3, 2);",
			"c - (a + b), y: -(a + b)", "c - a + b",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := Transpile("include <BOSL2/std.scad>\n"+tc.src, "part.scad")
			if err != nil {
				t.Fatalf("Transpile errored: %v", err)
			}
			if !strings.Contains(res.Facet, tc.want) {
				t.Fatalf("want substring %q, got:\n%s", tc.want, res.Facet)
			}
			if strings.Contains(res.Facet, tc.reject) {
				t.Fatalf("found mis-grouped %q (precedence bug), got:\n%s", tc.reject, res.Facet)
			}
			assertTypeChecks(t, res.Facet)
		})
	}
}
