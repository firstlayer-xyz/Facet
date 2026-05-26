//go:build js

package manifold

import (
	"fmt"
	"syscall/js"
)

func (p *Sketch) Extrude(height float64, slices int, twist, scaleX, scaleY float64) (*Solid, error) {
	if height == 0 {
		return nil, fmt.Errorf("Extrude: height must be non-zero")
	}
	id := js.Global().Call("_mf_extrude", p.id, height, slices, twist, scaleX, scaleY).Int()
	if id == 0 {
		return nil, fmt.Errorf("manifold: failed to extrude")
	}
	return newSolidWithOrigin(id), nil
}

func (p *Sketch) Revolve(segments int, degrees float64) (*Solid, error) {
	id := js.Global().Call("_mf_revolve", p.id, segments, degrees).Int()
	if id == 0 {
		return nil, fmt.Errorf("manifold: failed to revolve")
	}
	return newSolidWithOrigin(id), nil
}

func (p *Sketch) Sweep(path []Point3D) (*Solid, error) {
	n := len(path)
	if n < 2 {
		return nil, fmt.Errorf("Sweep: path must have at least 2 points, got %d", n)
	}
	pathArr := js.Global().Get("Float64Array").New(n * 3)
	for i, pt := range path {
		pathArr.SetIndex(i*3, pt.X)
		pathArr.SetIndex(i*3+1, pt.Y)
		pathArr.SetIndex(i*3+2, pt.Z)
	}
	id := js.Global().Call("_mf_sweep", p.id, pathArr, n).Int()
	if id == 0 {
		return nil, fmt.Errorf("manifold: failed to sweep")
	}
	return newSolidWithOrigin(id), nil
}

func Loft(sketches []*Sketch, heights []float64) (*Solid, error) {
	if len(sketches) < 2 {
		return nil, fmt.Errorf("Loft: need at least 2 sketches, got %d", len(sketches))
	}
	if len(sketches) != len(heights) {
		return nil, fmt.Errorf("Loft: sketches (%d) and heights (%d) must be same length", len(sketches), len(heights))
	}
	skArr := js.Global().Get("Array").New()
	for _, s := range sketches {
		skArr.Call("push", s.id)
	}
	htArr := js.Global().Get("Float64Array").New(len(heights))
	for i, h := range heights {
		htArr.SetIndex(i, h)
	}
	id := js.Global().Call("_mf_loft", skArr, len(sketches), htArr, len(heights)).Int()
	if id == 0 {
		return nil, fmt.Errorf("manifold: failed to loft")
	}
	return newSolidWithOrigin(id), nil
}

func (s *Solid) Slice(height float64) *Sketch {
	id := js.Global().Call("_mf_slice", s.id, height).Int()
	return newSketch(id)
}

func (s *Solid) Project() *Sketch {
	id := js.Global().Call("_mf_project", s.id).Int()
	return newSketch(id)
}
