package evaluator

import (
	"context"
	"math"
	"testing"
)

// evalAnim parses, checks, and evaluates source, returning the Animation handle.
func evalAnim(t *testing.T, src, entry string) *Animation {
	t.Helper()
	prog := parseTestProg(t, src)
	res, err := Eval(context.Background(), prog, testMainKey, nil, entry)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if res.Animation == nil {
		t.Fatal("expected EvalResult.Animation to be non-nil")
	}
	return res.Animation
}

// The frame closure is callable with a time value and returns the Solid for
// that instant — time genuinely varies the geometry, and the handle is reusable
// across calls (the retained evaluator backs every call).
func TestAnimationFrameVariesWithTime(t *testing.T) {
	src := `fn Main() Animation {
    return Animation{frame: fn(t Number) Solid { return Cube(s: (10 + t) * 1 mm) }}
}
`
	anim := evalAnim(t, src, "Main")

	s0, err := anim.Frame(0)
	if err != nil {
		t.Fatalf("Frame(0): %v", err)
	}
	s10, err := anim.Frame(10)
	if err != nil {
		t.Fatalf("Frame(10): %v", err)
	}
	if v := s0.Volume(); math.Abs(v-1000) > 1e-6 { // 10³ = 1000
		t.Fatalf("Frame(0) volume = %v, want 1000", v)
	}
	if v := s10.Volume(); math.Abs(v-8000) > 1e-6 { // 20³ = 8000
		t.Fatalf("Frame(10) volume = %v, want 8000", v)
	}
}
