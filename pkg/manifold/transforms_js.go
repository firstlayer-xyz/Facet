//go:build js

package manifold

import (
	"syscall/js"
)

func (s *Solid) Translate(x, y, z float64) *Solid {
	id := js.Global().Call("_mf_translate", s.id, x, y, z).Int()
	return transformSolid(s, id)
}

func (s *Solid) Rotate(x, y, z float64) *Solid {
	id := js.Global().Call("_mf_rotate_local", s.id, x, y, z).Int()
	return transformSolid(s, id)
}

func (s *Solid) Scale(x, y, z, ox, oy, oz float64) (*Solid, error) {
	if err := validateScaleFactor("x", x); err != nil {
		return nil, err
	}
	if err := validateScaleFactor("y", y); err != nil {
		return nil, err
	}
	if err := validateScaleFactor("z", z); err != nil {
		return nil, err
	}
	id := js.Global().Call("_mf_scale_at", s.id, x, y, z, ox, oy, oz).Int()
	return transformSolid(s, id), nil
}

func (s *Solid) Mirror(nx, ny, nz, offset float64) (*Solid, error) {
	if err := validateMirrorNormal3(nx, ny, nz); err != nil {
		return nil, err
	}
	id := js.Global().Call("_mf_mirror_at", s.id, nx, ny, nz, offset).Int()
	return transformSolid(s, id), nil
}

func (s *Solid) RotateAt(rx, ry, rz, ox, oy, oz float64) *Solid {
	id := js.Global().Call("_mf_rotate_at", s.id, rx, ry, rz, ox, oy, oz).Int()
	return transformSolid(s, id)
}

func (p *Sketch) Translate(x, y float64) *Sketch {
	id := js.Global().Call("_mf_cs_translate", p.id, x, y).Int()
	return newSketch(id)
}

func (p *Sketch) RotateOrigin(degrees float64) *Sketch {
	id := js.Global().Call("_mf_cs_rotate", p.id, degrees).Int()
	return newSketch(id)
}

func (p *Sketch) Rotate(degrees float64) *Sketch {
	id := js.Global().Call("_mf_cs_rotate_local", p.id, degrees).Int()
	return newSketch(id)
}

func (p *Sketch) Scale(x, y, px, py float64) (*Sketch, error) {
	if err := validateScaleFactor("x", x); err != nil {
		return nil, err
	}
	if err := validateScaleFactor("y", y); err != nil {
		return nil, err
	}
	id := js.Global().Call("_mf_cs_scale_at", p.id, x, y, px, py).Int()
	return newSketch(id), nil
}

func (p *Sketch) Mirror(ax, ay, offset float64) (*Sketch, error) {
	if err := validateMirrorNormal2(ax, ay); err != nil {
		return nil, err
	}
	id := js.Global().Call("_mf_cs_mirror_at", p.id, ax, ay, offset).Int()
	return newSketch(id), nil
}
