//go:build js

package manifold

import (
	"fmt"
	"math"
	"syscall/js"
)

func (s *Solid) Translate(x, y, z float64) *Solid {
	id := js.Global().Call("_mf_translate", s.id, x, y, z).Int()
	return transformSolid(s, id)
}

func scale(s *Solid, x, y, z float64) *Solid {
	id := js.Global().Call("_mf_scale", s.id, x, y, z).Int()
	return transformSolid(s, id)
}

func mirror(s *Solid, nx, ny, nz float64) *Solid {
	id := js.Global().Call("_mf_mirror", s.id, nx, ny, nz).Int()
	return transformSolid(s, id)
}

func (s *Solid) Rotate(x, y, z float64) *Solid {
	id := js.Global().Call("_mf_rotate_local", s.id, x, y, z).Int()
	return transformSolid(s, id)
}

func (s *Solid) Scale(x, y, z, ox, oy, oz float64) (*Solid, error) {
	if x == 0 || y == 0 || z == 0 {
		return nil, fmt.Errorf("Scale: factors must be non-zero (got x=%g, y=%g, z=%g)", x, y, z)
	}
	return scale(s.Translate(-ox, -oy, -oz), x, y, z).Translate(ox, oy, oz), nil
}

func (s *Solid) Mirror(nx, ny, nz, offset float64) (*Solid, error) {
	ln := math.Sqrt(nx*nx + ny*ny + nz*nz)
	if ln == 0 {
		return nil, fmt.Errorf("Mirror: normal vector has zero length")
	}
	dx, dy, dz := nx/ln*offset, ny/ln*offset, nz/ln*offset
	return mirror(s.Translate(-dx, -dy, -dz), nx, ny, nz).Translate(dx, dy, dz), nil
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

func sketchScale(p *Sketch, x, y float64) *Sketch {
	id := js.Global().Call("_mf_cs_scale", p.id, x, y).Int()
	return newSketch(id)
}

func sketchMirror(p *Sketch, ax, ay float64) *Sketch {
	id := js.Global().Call("_mf_cs_mirror", p.id, ax, ay).Int()
	return newSketch(id)
}

func (p *Sketch) Rotate(degrees float64) *Sketch {
	id := js.Global().Call("_mf_cs_rotate_local", p.id, degrees).Int()
	return newSketch(id)
}

func (p *Sketch) Scale(x, y, px, py float64) (*Sketch, error) {
	if x == 0 || y == 0 {
		return nil, fmt.Errorf("Scale: factors must be non-zero (got x=%g, y=%g)", x, y)
	}
	return sketchScale(p.Translate(-px, -py), x, y).Translate(px, py), nil
}

func (p *Sketch) Mirror(ax, ay, offset float64) (*Sketch, error) {
	ln := math.Sqrt(ax*ax + ay*ay)
	if ln == 0 {
		return nil, fmt.Errorf("Mirror: axis vector has zero length")
	}
	dx, dy := ax/ln*offset, ay/ln*offset
	return sketchMirror(p.Translate(-dx, -dy), ax, ay).Translate(dx, dy), nil
}
