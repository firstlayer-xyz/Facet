package scad

import (
	"strings"
	"testing"
)

// TestCylinderPartialAndMixedRadii covers the ends-defaulting fix: cylinder()
// given only one of r1/r2 (or d1/d2), or a mixed r1+d2, used to pass a nil
// operand into expr() and crash the transpiler with a SIGSEGV. Each form must
// now transpile to the OpenSCAD-equivalent Frustum (a missing radius end
// defaults to r/1, a missing diameter end to d/2, i.e. r=1), and type-check.
//
// The expected radii were checked against real OpenSCAD by STL equivalence,
// e.g. `cylinder(h=10, r1=5)` == `cylinder(h=10, r1=5, r2=1)`.
func TestCylinderPartialAndMixedRadii(t *testing.T) {
	cases := []struct {
		src  string
		want string
	}{
		{"cylinder(h=10, r1=5);", "Frustum(r1: 5 mm, r2: 1 mm, h: 10 mm"},
		{"cylinder(h=10, r2=5);", "Frustum(r1: 1 mm, r2: 5 mm, h: 10 mm"},
		{"cylinder(h=10, d1=8);", "Frustum(r1: 8 mm / 2, r2: 1 mm, h: 10 mm"},
		{"cylinder(h=10, d2=8);", "Frustum(r1: 1 mm, r2: 8 mm / 2, h: 10 mm"},
		{"cylinder(h=10, r1=5, d2=8);", "Frustum(r1: 5 mm, r2: 8 mm / 2, h: 10 mm"},
		{"cylinder(h=10, r=3, r1=5);", "Frustum(r1: 5 mm, r2: 3 mm, h: 10 mm"},
		{"cylinder(h=10, d=6, d1=8);", "Frustum(r1: 8 mm / 2, r2: 6 mm / 2, h: 10 mm"},
		// Both ends explicit stays exactly as before (no halving of a bare radius).
		{"cylinder(h=10, r1=8, r2=3);", "Frustum(r1: 8 mm, r2: 3 mm, h: 10 mm"},
		// A compound diameter must keep its parens so halving binds to the whole
		// sum, not just the last term — the precedence class half() exists for.
		{"cylinder(h=10, d1=4+4);", "Frustum(r1: (4 + 4) / 2, r2: 1 mm, h: 10 mm"},
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			res, err := Transpile(tc.src, "part.scad")
			if err != nil {
				t.Fatalf("Transpile(%q) errored: %v", tc.src, err)
			}
			if !strings.Contains(res.Facet, tc.want) {
				t.Fatalf("Transpile(%q):\nwant substring %q\ngot:\n%s", tc.src, tc.want, res.Facet)
			}
			assertTypeChecks(t, res.Facet)
		})
	}
}
