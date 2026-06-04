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

// TestAnimationFrameDoesNotLeakTracks verifies that solidTracks is bounded
// to the post-setup count after the first frame and does not grow across
// subsequent frames (i.e. no per-frame native-memory leak).
func TestAnimationFrameDoesNotLeakTracks(t *testing.T) {
	anim := evalAnim(t, `fn Main() Animation {
    return Animation{frame: fn(t Number) Solid { return Cube(s: (10 + t) * 1 mm) }}
}
`, "Main")
	if _, err := anim.Frame(0); err != nil {
		t.Fatal(err)
	}
	after1 := len(*anim.e.solidTracks)
	for i := 1; i < 50; i++ {
		if _, err := anim.Frame(float64(i)); err != nil {
			t.Fatal(err)
		}
	}
	after50 := len(*anim.e.solidTracks)
	if after50 != after1 {
		t.Fatalf("solidTracks grew across frames: after 1 = %d, after 50 = %d (leak)", after1, after50)
	}
}

// A retained Animation must keep working after the context that built it is
// canceled. Playback issues Frame() calls long after the building HTTP request
// has ended (and its context canceled), so the handle must not stay bound to
// that transient context — otherwise only the first frame renders.
func TestAnimationFrameSurvivesBuildContextCancel(t *testing.T) {
	prog := parseTestProg(t, `fn Main() Animation {
    var base = Cube(s: 20 mm)
    return Animation{frame: fn(t Number) Solid { return base.Rotate(z: t * 1 deg) }}
}
`)
	ctx, cancel := context.WithCancel(context.Background())
	res, err := Eval(ctx, prog, testMainKey, nil, "Main")
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if res.Animation == nil {
		t.Fatal("expected an Animation")
	}
	cancel() // the build request ends — its context is now canceled

	s, err := res.Animation.Frame(90)
	if err != nil {
		t.Fatalf("Frame after build-context cancel: %v", err)
	}
	if v := s.Volume(); math.Abs(v-8000) > 1e-6 { // 20³, rotation-invariant
		t.Fatalf("volume = %v, want 8000", v)
	}
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
