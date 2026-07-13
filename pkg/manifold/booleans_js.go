//go:build js

package manifold

import (
	"fmt"
	"syscall/js"
)

// binaryBool is the shared body of the pairwise Solid booleans: validate, call
// the fn bridge, merge the face maps (a's entries win). On a 0 id the kernel
// failed — return nil, not a silently-empty solid (matches native).
func (a *Solid) binaryBool(name, fn string, b *Solid) *Solid {
	requireSolids(name, a, b)
	id := js.Global().Call(fn, a.id, b.id).Int()
	if id == 0 {
		return nil
	}
	s := newSolid(id)
	s.FaceMap = mergeFaceMaps(a.FaceMap, b.FaceMap)
	return s
}

func (a *Solid) Union(b *Solid) *Solid {
	return a.binaryBool("Union", "_mf_union", b)
}

func (a *Solid) Difference(b *Solid) *Solid {
	return a.binaryBool("Difference", "_mf_difference", b)
}

func (a *Solid) Intersection(b *Solid) *Solid {
	return a.binaryBool("Intersection", "_mf_intersection", b)
}

// Insert cuts a hole in a for b, removes floating inner plugs, and seats b.
// Calls the same C++ facet_insert as the native build (via the _mf_insert
// bridge) so web and desktop produce identical geometry. A null (id 0) result
// signals the no-shell condition — see errInsertNoShell.
func (a *Solid) Insert(b *Solid) (*Solid, error) {
	requireSolids("Insert", a, b)
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
		if result[i] != nil {
			result[i].FaceMap = s.withFaceMap()
		}
	}
	return result
}

// binaryBool is the shared body of the pairwise Sketch booleans (sketches
// carry no face map). Panics if either operand is nil.
func (a *Sketch) binaryBool(name, fn string, b *Sketch) *Sketch {
	requireSketches(name, a, b)
	return newSketch(js.Global().Call(fn, a.id, b.id).Int())
}

func (a *Sketch) Union(b *Sketch) *Sketch {
	return a.binaryBool("Sketch.Union", "_mf_cs_union", b)
}

func (a *Sketch) Difference(b *Sketch) *Sketch {
	return a.binaryBool("Sketch.Difference", "_mf_cs_difference", b)
}

func (a *Sketch) Intersection(b *Sketch) *Sketch {
	return a.binaryBool("Sketch.Intersection", "_mf_cs_intersection", b)
}

func ComposeSolids(solids []*Solid) (*Solid, error) {
	if len(solids) == 0 {
		return nil, fmt.Errorf("ComposeSolids: solids is empty")
	}
	requireSolids("ComposeSolids", solids...)
	id := js.Global().Call("_mf_compose", solidIDArray(solids)).Int()
	if id == 0 {
		return nil, fmt.Errorf("ComposeSolids: compose failed")
	}
	r := newSolid(id)
	r.FaceMap = mergedFaceMaps(solids)
	return r, nil
}

// BatchBoolean combines solids with op in one kernel tree-reduction. wasm twin of
// the native build; same semantics so both produce identical geometry + colors.
func BatchBoolean(solids []*Solid, op BoolOp) (*Solid, error) {
	if len(solids) == 0 {
		return nil, errBatchBooleanEmpty
	}
	requireSolids("BatchBoolean", solids...)
	id := js.Global().Call("_mf_batch_boolean", solidIDArray(solids), int(op)).Int()
	if id == 0 {
		return nil, errBatchBooleanFailed
	}
	s := newSolid(id)
	s.FaceMap = mergedFaceMaps(solids)
	return s, nil
}

// SketchBatchBoolean is the 2D counterpart (no face map).
func SketchBatchBoolean(sketches []*Sketch, op BoolOp) (*Sketch, error) {
	if len(sketches) == 0 {
		return nil, errBatchBooleanEmpty
	}
	requireSketches("SketchBatchBoolean", sketches...)
	id := js.Global().Call("_mf_cs_batch_boolean", sketchIDArray(sketches), int(op)).Int()
	if id == 0 {
		return nil, errBatchBooleanFailed
	}
	return newSketch(id), nil
}
