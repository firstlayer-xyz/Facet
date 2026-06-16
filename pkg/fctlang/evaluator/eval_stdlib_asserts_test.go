package evaluator

import (
	"strings"
	"testing"
)

// The stdlib minimum-count asserts must surface an actionable message on
// degenerate counts instead of the opaque internal "fold: empty array" /
// "empty array" error from the comprehension they fold over.
func TestStdlibMinimumCountAsserts(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"Ngon", `fn Main() Sketch { return Ngon(n: 2, r: 10 mm); }`, "Ngon requires n >= 3"},
		{"Star", `fn Main() Sketch { return Star(n: 1, r: 10 mm, ir: 4 mm); }`, "Star requires n >= 2"},
		{"EvenOdd", `fn Main() Solid { return EvenOdd(solids: []Solid[]); }`, "EvenOdd requires at least one solid"},
		{"Solid.LinearPattern", `fn Main() Solid { return Cube(s: 5 mm).LinearPattern(count: 0); }`, "LinearPattern requires count >= 1"},
		{"Sketch.LinearPattern", `fn Main() Sketch { return Square(s: 5 mm).LinearPattern(count: 0); }`, "LinearPattern requires count >= 1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if msg := evalErr(t, c.src); !strings.Contains(msg, c.want) {
				t.Errorf("got: %s\nwant substring: %q", msg, c.want)
			}
		})
	}
}
