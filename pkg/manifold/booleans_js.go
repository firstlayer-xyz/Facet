//go:build js

package manifold

import (
	"fmt"
	"syscall/js"
)

func (a *Solid) Union(b *Solid) *Solid {
	id := js.Global().Call("_mf_union", a.id, b.id).Int()
	s := newSolid(id)
	s.FaceMap = mergeFaceMaps(a.FaceMap, b.FaceMap)
	return s
}

func (a *Solid) Difference(b *Solid) *Solid {
	id := js.Global().Call("_mf_difference", a.id, b.id).Int()
	s := newSolid(id)
	s.FaceMap = mergeFaceMaps(a.FaceMap, b.FaceMap)
	return s
}

func (a *Solid) Intersection(b *Solid) *Solid {
	id := js.Global().Call("_mf_intersection", a.id, b.id).Int()
	s := newSolid(id)
	s.FaceMap = mergeFaceMaps(a.FaceMap, b.FaceMap)
	return s
}

// Insert cuts a hole in a for b, removes floating inner plugs, and seats b.
// Calls the same C++ facet_insert as the native build (via the _mf_insert
// bridge) so web and desktop produce identical geometry. A null (id 0) result
// signals the no-shell condition — see errInsertNoShell.
func (a *Solid) Insert(b *Solid) (*Solid, error) {
	id := js.Global().Call("_mf_insert", a.id, b.id).Int()
	if id == 0 {
		return nil, errInsertNoShell
	}
	s := newSolid(id)
	s.FaceMap = mergeFaceMaps(a.FaceMap, b.FaceMap)
	return s, nil
}

func DecomposeSolid(s *Solid) []*Solid {
	arr := js.Global().Call("_mf_decompose", s.id)
	n := arr.Length()
	if n == 0 {
		return nil
	}
	result := make([]*Solid, n)
	for i := 0; i < n; i++ {
		id := arr.Index(i).Int()
		result[i] = newSolid(id)
		result[i].FaceMap = s.withFaceMap()
	}
	return result
}

func (a *Sketch) Union(b *Sketch) *Sketch {
	id := js.Global().Call("_mf_cs_union", a.id, b.id).Int()
	return newSketch(id)
}

func (a *Sketch) Difference(b *Sketch) *Sketch {
	id := js.Global().Call("_mf_cs_difference", a.id, b.id).Int()
	return newSketch(id)
}

func (a *Sketch) Intersection(b *Sketch) *Sketch {
	id := js.Global().Call("_mf_cs_intersection", a.id, b.id).Int()
	return newSketch(id)
}

func ComposeSolids(solids []*Solid) (*Solid, error) {
	if len(solids) == 0 {
		return nil, fmt.Errorf("ComposeSolids: solids is empty")
	}
	arr := js.Global().Get("Array").New()
	for _, s := range solids {
		arr.Call("push", s.id)
	}
	id := js.Global().Call("_mf_compose", arr).Int()
	r := newSolid(id)
	for _, s := range solids {
		r.FaceMap = mergeFaceMaps(r.FaceMap, s.FaceMap)
	}
	return r, nil
}
