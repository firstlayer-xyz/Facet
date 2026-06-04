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

// TestAnimationFrameReleasesPriorFrameTracks verifies that each frame's
// solidTracks are released, not merely hidden by reslicing. A frame that
// produces many solids followed by one that produces few must leave NO solid
// pointers retained in the backing array's trailing slots — otherwise those
// per-frame solids stay reachable and their C++ geometry never frees (a leak a
// len-only check cannot detect, since reslicing preserves the count).
func TestAnimationFrameReleasesPriorFrameTracks(t *testing.T) {
	anim := evalAnim(t, `fn Main() Animation {
    return Animation{frame: fn(t Number) Solid {
        const n = t == 0 ? 8 : 2
        return Union(arr: for i [0:<n] { yield Cube(s: 5 mm).Move(x: i * 6 mm) })
    }}
}
`, "Main")
	if _, err := anim.Frame(0); err != nil { // many solids
		t.Fatal(err)
	}
	if _, err := anim.Frame(1); err != nil { // few solids
		t.Fatal(err)
	}
	tracks := *anim.e.solidTracks
	full := tracks[:cap(tracks)]
	for i := len(tracks); i < len(full); i++ {
		if full[i].Solid != nil {
			t.Fatalf("solidTracks slot %d (beyond len %d) still retains a solid after a smaller frame — leak", i, len(tracks))
		}
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
