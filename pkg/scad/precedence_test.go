package scad

import (
	"strings"
	"testing"
)

// Centering and diameter-halving shift or scale a primitive by an emitted radius/
// size expression. When that expression is compound (e.g. `w + g`), the `-` and
// `/ 2` must bind to the whole value, not just its first/last operand — otherwise
// `-w + g` / `w + g / 2` silently mis-positions or mis-sizes the geometry. These
// cases all feed a compound expression (two consts, unfoldable) through the
// negate/half emit helpers and assert the parentheses survive.
func TestEmitCompoundExprPrecedence(t *testing.T) {
	const defs = "w = 4;\ng = 2;\n"
	const bosl2 = "include <BOSL2/std.scad>\n"
	cases := []struct {
		name string
		src  string
		want string // a substring that only the correctly-parenthesized output contains
	}{
		// builtins
		{"circle_d", defs + "circle(d = w + g);", "Move(x: -(w + g) / 2, y: -(w + g) / 2)"},
		{"circle_r", defs + "circle(r = w + g);", "Move(x: -(w + g), y: -(w + g))"},
		{"square_vec_center", defs + "square([w + g, 3], center = true);", "Move(x: -(w + g) / 2, y: -3 mm / 2)"},
		{"square_scalar_center", defs + "square(w + g, center = true);", "Move(x: -(w + g) / 2, y: -(w + g) / 2)"},
		// BOSL2
		{"rect_center", bosl2 + defs + "rect([w + g, 3]);", "Move(x: -(w + g) / 2, y: -3 mm / 2)"},
		{"hexagon_d", bosl2 + defs + "hexagon(d = w + g);", "Ngon(n: 6, r: (w + g) / 2)"},
		{"tube_od", bosl2 + defs + "tube(h = 6, od = w + g, id = 3);", "Cylinder(r: (w + g) / 2, h: 6 mm)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res, err := Transpile(c.src, "part.scad")
			if err != nil {
				t.Fatalf("transpile failed: %v", err)
			}
			if !strings.Contains(res.Facet, c.want) {
				t.Fatalf("missing correctly-parenthesized output %q in:\n%s", c.want, res.Facet)
			}
			assertTypeChecks(t, res.Facet)
		})
	}
}
