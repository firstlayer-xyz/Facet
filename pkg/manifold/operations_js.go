//go:build js

package manifold

import (
	"fmt"
	"syscall/js"
)

func (s *Solid) Hull() *Solid {
	requireSolids("Hull", s)
	id := js.Global().Call("_mf_hull", s.id).Int()
	r := newSolid(id)
	if r == nil {
		return nil
	}
	seedHullFaceMap(r, js.Global().Call("_mf_original_id", id).Int(), firstFaceInfo(s))
	return r
}

func (s *Solid) Reidentify() *Solid {
	requireSolids("Reidentify", s)
	id := js.Global().Call("_mf_as_original", s.id).Int()
	r := newSolid(id)
	if r == nil {
		return nil
	}
	seedHullFaceMap(r, js.Global().Call("_mf_original_id", id).Int(), firstFaceInfo(s))
	return r
}

func BatchHull(solids []*Solid) (*Solid, error) {
	if len(solids) == 0 {
		return nil, fmt.Errorf("BatchHull: solids is empty")
	}
	requireSolids("BatchHull", solids...)
	arr := js.Global().Get("Array").New()
	for _, s := range solids {
		arr.Call("push", s.id)
	}
	id := js.Global().Call("_mf_batch_hull", arr).Int()
	r := newSolid(id)
	if r == nil {
		return nil, nil
	}
	seedHullFaceMap(r, js.Global().Call("_mf_original_id", id).Int(), firstFaceInfo(solids...))
	return r, nil
}

func HullPoints(points []Point3D) (*Solid, error) {
	n := len(points)
	if n < 4 {
		return nil, fmt.Errorf("HullPoints: need at least 4 points for a 3D hull, got %d", n)
	}
	arr := js.Global().Get("Float64Array").New(n * 3)
	for i, p := range points {
		arr.SetIndex(i*3, p.X)
		arr.SetIndex(i*3+1, p.Y)
		arr.SetIndex(i*3+2, p.Z)
	}
	id := js.Global().Call("_mf_hull_points", arr, n).Int()
	return newSolidWithOrigin(id), nil
}

func (s *Solid) TrimByPlane(nx, ny, nz, offset float64) *Solid {
	id := js.Global().Call("_mf_trim_by_plane", s.id, nx, ny, nz, offset).Int()
	return transformSolid(s, id)
}

func (s *Solid) SmoothOut(minSharpAngle, minSmoothness float64) *Solid {
	id := js.Global().Call("_mf_smooth_out", s.id, minSharpAngle, minSmoothness).Int()
	return transformSolid(s, id)
}

func (s *Solid) Refine(n int) *Solid {
	id := js.Global().Call("_mf_refine", s.id, n).Int()
	return transformSolid(s, id)
}

func (s *Solid) Simplify(tolerance float64) *Solid {
	id := js.Global().Call("_mf_simplify", s.id, tolerance).Int()
	return transformSolid(s, id)
}

func (s *Solid) RefineToLength(length float64) *Solid {
	id := js.Global().Call("_mf_refine_to_length", s.id, length).Int()
	return transformSolid(s, id)
}

func (s *Solid) Offset(delta, edgeLen float64) *Solid {
	requireSolids("Offset", s)
	id := js.Global().Call("_mf_offset", s.id, delta, edgeLen).Int()
	r := newSolid(id)
	if r == nil {
		return nil
	}
	seedHullFaceMap(r, js.Global().Call("_mf_original_id", id).Int(), firstFaceInfo(s))
	return r
}

func SplitSolid(m, cutter *Solid) [2]*Solid {
	requireSolids("SplitSolid", m, cutter)
	arr := js.Global().Call("_mf_split", m.id, cutter.id)
	// Both halves originate from m's geometry; the cutter's FaceMap is
	// intentionally not propagated (its faces appear in neither result), matching
	// the native build and SplitSolidByPlane. Each half gets its OWN copy —
	// sharing one instance (as before) let a later mutation of one half's map
	// bleed into the other, unlike native (operations.go).
	first := newSolid(arr.Index(0).Int())
	if first != nil {
		first.FaceMap = m.withFaceMap()
	}
	second := newSolid(arr.Index(1).Int())
	if second != nil {
		second.FaceMap = m.withFaceMap()
	}
	return [2]*Solid{first, second}
}

func SplitSolidByPlane(s *Solid, nx, ny, nz, offset float64) [2]*Solid {
	requireSolids("SplitSolidByPlane", s)
	arr := js.Global().Call("_mf_split_by_plane", s.id, nx, ny, nz, offset)
	first := newSolid(arr.Index(0).Int())
	if first != nil {
		first.FaceMap = s.withFaceMap()
	}
	second := newSolid(arr.Index(1).Int())
	if second != nil {
		second.FaceMap = s.withFaceMap()
	}
	return [2]*Solid{first, second}
}

func (p *Sketch) Hull() *Sketch {
	id := js.Global().Call("_mf_cs_hull", p.id).Int()
	return newSketch(id)
}

func SketchBatchHull(sketches []*Sketch) (*Sketch, error) {
	if len(sketches) == 0 {
		return nil, fmt.Errorf("SketchBatchHull: sketches is empty")
	}
	arr := js.Global().Get("Array").New()
	for _, p := range sketches {
		arr.Call("push", p.id)
	}
	id := js.Global().Call("_mf_cs_batch_hull", arr).Int()
	return newSketch(id), nil
}

func (p *Sketch) Offset(delta float64, segments int) *Sketch {
	id := js.Global().Call("_mf_cs_offset", p.id, delta, segments).Int()
	return newSketch(id)
}
