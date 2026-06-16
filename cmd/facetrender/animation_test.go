package main

import "testing"

// A box rotating about Z over time — vertex positions differ between times.
const animSrc = `fn Main() Animation {
    return Animation{
        frame: fn(t Number) Solid {
            return Cube(x: 10 mm, y: 4 mm, z: 4 mm).Rotate(z: (t / 10) * 1 deg)
        }
    }
}
`

const staticSrc = `fn Main() Solid { return Cube(x: 10 mm, y: 10 mm, z: 10 mm) }`

func TestOpenAnimation_DetectsAnimation(t *testing.T) {
	h, ok := openAnimation(animSrc)
	if !ok || h == 0 {
		t.Fatalf("openAnimation(anim) = (%d,%v), want non-zero handle, true", h, ok)
	}
	defer closeAnimation(h)

	if h0, ok0 := openAnimation(staticSrc); ok0 || h0 != 0 {
		t.Errorf("openAnimation(static) = (%d,%v), want (0,false)", h0, ok0)
	}
	if h2, ok2 := openAnimation("this is not facet"); ok2 || h2 != 0 {
		t.Errorf("openAnimation(garbage) = (%d,%v), want (0,false)", h2, ok2)
	}
}

func TestAnimationFrame_DiffersOverTime(t *testing.T) {
	h, ok := openAnimation(animSrc)
	if !ok {
		t.Fatal("openAnimation failed")
	}
	defer closeAnimation(h)

	p0, _ := animationFrame(h, 0)
	p1, _ := animationFrame(h, 900) // 90 degrees
	if len(p0) == 0 || len(p1) == 0 {
		t.Fatalf("empty frame(s): len(p0)=%d len(p1)=%d", len(p0), len(p1))
	}
	if len(p0) != len(p1) {
		t.Fatalf("frame vertex counts differ: %d vs %d", len(p0), len(p1))
	}
	differs := false
	for i := range p0 {
		if p0[i] != p1[i] {
			differs = true
			break
		}
	}
	if !differs {
		t.Error("frames at t=0 and t=900 are identical; animation did not advance")
	}
}

func TestAnimationFrame_AfterCloseReturnsNil(t *testing.T) {
	h, ok := openAnimation(animSrc)
	if !ok {
		t.Fatal("openAnimation failed")
	}
	closeAnimation(h)
	if p, _ := animationFrame(h, 0); p != nil {
		t.Errorf("animationFrame after close = %v, want nil", p)
	}
}
