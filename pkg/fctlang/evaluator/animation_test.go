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

// TestAnimationFrameTruncatesPriorFrameTracks verifies each frame's per-frame
// solidTracks (indices >= baseTracks) are truncated at the start of the next
// frame rather than accumulated. A frame producing many solids followed by one
// producing few must leave the smaller frame's track count — otherwise the
// per-frame posMap would carry stale entries from earlier frames.
func TestAnimationFrameTruncatesPriorFrameTracks(t *testing.T) {
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
	manyPerFrame := len(*anim.e.solidTracks) - anim.baseTracks
	if _, err := anim.Frame(1); err != nil { // few solids
		t.Fatal(err)
	}
	fewPerFrame := len(*anim.e.solidTracks) - anim.baseTracks
	if fewPerFrame <= 0 {
		t.Fatalf("second frame recorded no per-frame tracks (%d)", fewPerFrame)
	}
	if fewPerFrame >= manyPerFrame {
		t.Fatalf("per-frame tracks not truncated: frame 0 had %d, frame 1 has %d (want fewer) — tracks accumulated across frames", manyPerFrame, fewPerFrame)
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

// FrameWithPosMap must return a populated source-position → face-ID map for
// every frame, so face-click → source navigation works during animation
// playback. Without it the frontend received an empty posMap and silently
// swallowed every click. Regression for "clicking a face does nothing on
// animated models". Two frames are checked because playback truncates the
// per-frame tracks between frames — the rebuild must still produce a posMap.
func TestAnimationFrameWithPosMap(t *testing.T) {
	src := `fn Main() Animation {
    return Animation{frame: fn(t Number) Solid { return Cube(s: (10 + t) * 1 mm) }}
}
`
	anim := evalAnim(t, src, "Main")

	for _, tm := range []float64{0, 10} {
		solid, posMap, err := anim.FrameWithPosMap(tm)
		if err != nil {
			t.Fatalf("FrameWithPosMap(%v): %v", tm, err)
		}
		if len(posMap) == 0 {
			t.Fatalf("FrameWithPosMap(%v): empty PosMap — face-click would be dead on animation frames", tm)
		}
		// At least one mapped face ID must be a real face of the rendered solid,
		// so a click that lands on a face group resolves to a source position.
		faceIDs := map[uint32]bool{}
		for id := range solid.FaceMap {
			faceIDs[id] = true
		}
		matched := false
		for _, e := range posMap {
			for _, id := range e.FaceIDs {
				if faceIDs[id] {
					matched = true
				}
			}
		}
		if !matched {
			t.Fatalf("FrameWithPosMap(%v): no PosMap face ID matches the frame solid's FaceMap — clicks could not resolve", tm)
		}
	}
}
