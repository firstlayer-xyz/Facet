package evaluator

import (
	"context"
	"strings"
	"testing"
)

// Segment / subdivision counts flow straight into the C++ kernel, which does
// not clamp; an unbounded literal (e.g. 1e8) would OOM/hang the host. They are
// now capped and error cleanly instead. (The comprehension-product cap shares
// maxRangeSize and isn't unit-tested here — triggering it needs >10M iterations.)
func TestEvalCountCaps(t *testing.T) {
	cases := []struct{ name, src string }{
		{"sphere segments", `fn Main() Solid { return Sphere(r: 5 mm, segments: 100000000); }`},
		{"cylinder segments", `fn Main() Solid { return Cylinder(r: 5 mm, h: 10 mm, segments: 100000000); }`},
		{"refine factor", `fn Main() Solid { return Cube(s: 10 mm).Refine(n: 100000); }`},
	}
	for _, c := range cases {
		prog := parseTestProg(t, c.src)
		_, err := Eval(context.Background(), prog, testMainKey, nil, "Main")
		if err == nil {
			t.Errorf("%s: expected a count-cap error, got none", c.name)
			continue
		}
		if !strings.Contains(err.Error(), "maximum") {
			t.Errorf("%s: expected a 'maximum count' error, got %v", c.name, err)
		}
	}
}
