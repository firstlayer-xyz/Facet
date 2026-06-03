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
	s0, err := anim0.Frame(0)
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
	s10, err := anim1.Frame(10)
	if err != nil {
		t.Fatalf("Frame(10): %v", err)
	}
	if v := s10.Volume(); v < 7999 || v > 8001 {
		t.Fatalf("Frame(10) volume = %v, want ~8000", v)
	}
}
