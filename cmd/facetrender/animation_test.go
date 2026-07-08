package main

import "testing"

// animSrc spins a centered cube once every 4 s (t is Unix epoch ms), so frames
// at different times have different vertex positions. staticSrc returns a plain
// Solid — Main is not an Animation.
const animSrc = `fn Main() Animation {
	var base = Cube(s: 20 mm).Move(x: -10 mm, y: -10 mm, z: -10 mm)
	return Animation{
		frame: fn(t Number) Solid {
			const deg = (t % 4000) / 4000 * 360
			return base.Rotate(z: deg * 1 deg)
		}
	}
}`

const staticSrc = `fn Main() Solid {
	return Cube(s: 20 mm)
}`

func TestOpenAnimation_DetectsAnimation(t *testing.T) {
	h, ok := openAnimation(animSrc)
	if !ok || h == 0 {
		t.Fatalf("openAnimation(animSrc) = (%d, %v), want non-zero handle and true", h, ok)
	}
	defer closeAnimation(h)

	h2, ok2 := openAnimation(staticSrc)
	if ok2 || h2 != 0 {
		t.Fatalf("openAnimation(staticSrc) = (%d, %v), want (0, false)", h2, ok2)
	}
}

func TestAnimationFrame_DiffersOverTime(t *testing.T) {
	h, ok := openAnimation(animSrc)
	if !ok {
		t.Fatal("openAnimation(animSrc) reported not-an-animation")
	}
	defer closeAnimation(h)

	p0, _ := animationFrame(h, 0)   // deg 0
	p1, _ := animationFrame(h, 500) // deg 45 (period 4000 ms)
	if len(p0) == 0 || len(p1) == 0 {
		t.Fatalf("empty frame: len(p0)=%d len(p1)=%d", len(p0), len(p1))
	}
	if len(p0) != len(p1) {
		t.Fatalf("frame vertex counts differ across time: %d vs %d", len(p0), len(p1))
	}
	same := true
	for i := range p0 {
		if p0[i] != p1[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("rotated frame identical to t=0; animation is not advancing")
	}
}

func TestAnimationFrame_AfterCloseReturnsNil(t *testing.T) {
	h, ok := openAnimation(animSrc)
	if !ok {
		t.Fatal("openAnimation(animSrc) reported not-an-animation")
	}
	closeAnimation(h)
	if p, c := animationFrame(h, 0); p != nil || c != nil {
		t.Errorf("animationFrame after close = (len %d, len %d), want (nil, nil)", len(p), len(c))
	}
}
