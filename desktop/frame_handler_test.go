package main

import (
	"context"
	"testing"
)

const animSrc = `fn Main() Animation {
    return Animation{frame: fn(t Number) Solid { return Cube(s: (10 + t) * 1 mm) }}
}
`

// TestFrameSessionVariesAndReuses verifies that the session cache builds an
// Animation once and reuses it on identical inputs, and that Frame varies with
// the time argument.
func TestFrameSessionVariesAndReuses(t *testing.T) {
	c := newSessionCache()
	sources := map[string]string{"main.fct": animSrc}

	anim0, err := c.getOrBuild(context.Background(), sources, "main.fct", "Main", nil)
	if err != nil {
		t.Fatalf("getOrBuild: %v", err)
	}
	s0, err := anim0.Frame(context.Background(), 0)
	if err != nil {
		t.Fatalf("Frame(0): %v", err)
	}
	if v := s0.Volume(); v < 999 || v > 1001 {
		t.Fatalf("Frame(0) volume = %v, want ~1000", v)
	}

	anim1, err := c.getOrBuild(context.Background(), sources, "main.fct", "Main", nil)
	if err != nil {
		t.Fatalf("getOrBuild (reuse): %v", err)
	}
	if anim1 != anim0 {
		t.Fatal("expected the same cached Animation handle on identical inputs")
	}
	s10, err := anim1.Frame(context.Background(), 10)
	if err != nil {
		t.Fatalf("Frame(10): %v", err)
	}
	if v := s10.Volume(); v < 7999 || v > 8001 {
		t.Fatalf("Frame(10) volume = %v, want ~8000", v)
	}
}

// TestSessionKeyInjectiveOverSeparator guards against the hash collision a
// naive separator scheme allows. These two inputs serialize to the SAME byte
// stream under a "<path>\x00<content>\x00" scheme — config A's single source
// content embeds the \x00 separators that, in config B, fall between two
// distinct sources. Length-prefixing (and a leading source count) must keep
// their keys distinct.
func TestSessionKeyInjectiveOverSeparator(t *testing.T) {
	a := map[string]string{"a": "b\x00c\x00"}
	b := map[string]string{"a": "b", "c": ""}

	ka, err := sessionKey(a, "", "", nil)
	if err != nil {
		t.Fatalf("sessionKey(a): %v", err)
	}
	kb, err := sessionKey(b, "", "", nil)
	if err != nil {
		t.Fatalf("sessionKey(b): %v", err)
	}
	if ka == kb {
		t.Fatalf("distinct inputs collided to the same key %q — separator scheme is not injective", ka)
	}
}

// TestSessionPutPrimesCache verifies put() stores a handle so a subsequent
// getOrBuild with identical inputs reuses it instead of rebuilding — the
// optimization that lets /eval hand its freshly built Animation to the
// playback /frame requests that follow.
func TestSessionPutPrimesCache(t *testing.T) {
	sources := map[string]string{"main.fct": animSrc}

	// Build a handle once (in its own cache) to use as the thing /eval would
	// have produced.
	built, err := newSessionCache().getOrBuild(context.Background(), sources, "main.fct", "Main", nil)
	if err != nil {
		t.Fatalf("getOrBuild (build handle): %v", err)
	}

	c := newSessionCache()
	if err := c.put(sources, "main.fct", "Main", nil, built); err != nil {
		t.Fatalf("put: %v", err)
	}
	got, err := c.getOrBuild(context.Background(), sources, "main.fct", "Main", nil)
	if err != nil {
		t.Fatalf("getOrBuild (after put): %v", err)
	}
	if got != built {
		t.Fatal("getOrBuild after put rebuilt the Animation instead of reusing the primed handle")
	}
}
