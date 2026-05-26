//go:build js

package manifold

import "syscall/js"

func (s *Solid) BoundingBox() (minX, minY, minZ, maxX, maxY, maxZ float64) {
	r := js.Global().Call("_mf_bounding_box", s.id)
	return r.Index(0).Float(), r.Index(1).Float(), r.Index(2).Float(),
		r.Index(3).Float(), r.Index(4).Float(), r.Index(5).Float()
}

func (s *Solid) Volume() float64 {
	return js.Global().Call("_mf_volume", s.id).Float()
}

func (s *Solid) SurfaceArea() float64 {
	return js.Global().Call("_mf_surface_area", s.id).Float()
}

func (s *Solid) NumComponents() int {
	return js.Global().Call("_mf_num_components", s.id).Int()
}

func (s *Solid) Genus() int {
	return js.Global().Call("_mf_genus", s.id).Int()
}

func (s *Solid) MinGap(other *Solid, searchLength float64) float64 {
	return js.Global().Call("_mf_min_gap", s.id, other.id, searchLength).Float()
}

func (p *Sketch) BoundingBox() (minX, minY, maxX, maxY float64) {
	r := js.Global().Call("_mf_cs_bounds", p.id)
	return r.Index(0).Float(), r.Index(1).Float(), r.Index(2).Float(), r.Index(3).Float()
}

func (p *Sketch) Area() float64 {
	return js.Global().Call("_mf_cs_area", p.id).Float()
}
