package manifold

import (
	"fmt"
	"sync"
)

// ---------------------------------------------------------------------------
// Generic future core — lazy evaluation with sync.Once
// ---------------------------------------------------------------------------

// future is the generic core of a lazy computation. The closure is not
// executed until resolve() is called.
type future[T any] struct {
	fn   func() (T, error)
	val  T
	err  error
	once sync.Once
}

// start returns a lazy future that defers fn until resolve() is called.
func start[T any](fn func() (T, error)) *future[T] {
	return &future[T]{fn: fn}
}

func (f *future[T]) resolve() (T, error) {
	f.once.Do(func() {
		f.val, f.err = f.fn()
		f.fn = nil // release closure for GC
	})
	return f.val, f.err
}

func immediate[T any](val T) *future[T] {
	f := &future[T]{val: val}
	f.once.Do(func() {}) // mark as already resolved
	return f
}

// ---------------------------------------------------------------------------
// Concrete future types
// ---------------------------------------------------------------------------

// SolidFuture represents an in-progress or completed Solid computation.
type SolidFuture struct{ inner *future[*Solid] }

// SketchFuture represents an in-progress or completed Sketch computation.
type SketchFuture struct{ inner *future[*Sketch] }

func startSolidFuture(fn func() (*Solid, error)) *SolidFuture {
	return &SolidFuture{start(fn)}
}

func startSketchFuture(fn func() (*Sketch, error)) *SketchFuture {
	return &SketchFuture{start(fn)}
}

// Resolve blocks until the solid is ready and returns it.
func (sf *SolidFuture) Resolve() (*Solid, error) { return sf.inner.resolve() }

// Resolve blocks until the sketch is ready and returns it.
func (sf *SketchFuture) Resolve() (*Sketch, error) { return sf.inner.resolve() }

// ImmediateSolid wraps an already-resolved Solid as a future.
func ImmediateSolid(s *Solid) *SolidFuture { return &SolidFuture{immediate(s)} }

// immediateSketch wraps an already-resolved Sketch as a future.
func immediateSketch(s *Sketch) *SketchFuture { return &SketchFuture{immediate(s)} }

// ToDisplayMesh resolves and extracts a DisplayMesh (implements debugShape).
func (sf *SolidFuture) ToDisplayMesh() *DisplayMesh {
	s, err := sf.Resolve()
	if err != nil || s == nil {
		return &DisplayMesh{}
	}
	return s.ToDisplayMesh()
}

// ToDisplayMesh resolves and extracts a DisplayMesh (implements debugShape).
func (sf *SketchFuture) ToDisplayMesh() *DisplayMesh {
	s, err := sf.Resolve()
	if err != nil || s == nil {
		return &DisplayMesh{}
	}
	return s.ToDisplayMesh()
}

// ---------------------------------------------------------------------------
// SolidFuture methods — chain goroutines for pipelined concurrency
// ---------------------------------------------------------------------------

// Translate moves a solid by (x, y, z).
func (sf *SolidFuture) Translate(x, y, z float64) *SolidFuture {
	return startSolidFuture(func() (*Solid, error) {
		s, err := sf.Resolve()
		if err != nil {
			return nil, err
		}
		return translate(s, x, y, z), nil
	})
}

// Rotate rotates a solid by (x, y, z) degrees around each axis, pivoting on
// the bounding box center so the solid spins in place.
func (sf *SolidFuture) Rotate(x, y, z float64) *SolidFuture {
	return startSolidFuture(func() (*Solid, error) {
		s, err := sf.Resolve()
		if err != nil {
			return nil, err
		}
		return rotateLocal(s, x, y, z), nil
	})
}

// Scale scales a solid by (x, y, z) around pivot point (ox, oy, oz).
func (sf *SolidFuture) Scale(x, y, z, ox, oy, oz float64) *SolidFuture {
	return startSolidFuture(func() (*Solid, error) {
		s, err := sf.Resolve()
		if err != nil {
			return nil, err
		}
		return scaleAt(s, x, y, z, ox, oy, oz), nil
	})
}

// Mirror mirrors a solid across the plane with normal (nx, ny, nz) at signed
// distance offset from the world origin.
func (sf *SolidFuture) Mirror(nx, ny, nz, offset float64) *SolidFuture {
	return startSolidFuture(func() (*Solid, error) {
		s, err := sf.Resolve()
		if err != nil {
			return nil, err
		}
		return mirrorAt(s, nx, ny, nz, offset), nil
	})
}

// RotateAt rotates a solid by (rx, ry, rz) degrees around pivot point (ox, oy, oz).
func (sf *SolidFuture) RotateAt(rx, ry, rz, ox, oy, oz float64) *SolidFuture {
	return startSolidFuture(func() (*Solid, error) {
		s, err := sf.Resolve()
		if err != nil {
			return nil, err
		}
		return rotateAt(s, rx, ry, rz, ox, oy, oz), nil
	})
}

// TrimByPlane trims a solid by the plane defined by normal and offset.
func (sf *SolidFuture) TrimByPlane(nx, ny, nz, offset float64) *SolidFuture {
	return startSolidFuture(func() (*Solid, error) {
		s, err := sf.Resolve()
		if err != nil {
			return nil, err
		}
		return trimByPlane(s, nx, ny, nz, offset), nil
	})
}

// SmoothOut smooths sharp edges of a solid.
func (sf *SolidFuture) SmoothOut(minSharpAngle, minSmoothness float64) *SolidFuture {
	return startSolidFuture(func() (*Solid, error) {
		s, err := sf.Resolve()
		if err != nil {
			return nil, err
		}
		return smoothOut(s, minSharpAngle, minSmoothness), nil
	})
}

// Refine subdivides the mesh of a solid n times.
func (sf *SolidFuture) Refine(n int) *SolidFuture {
	return startSolidFuture(func() (*Solid, error) {
		s, err := sf.Resolve()
		if err != nil {
			return nil, err
		}
		return refine(s, n), nil
	})
}

// Simplify reduces the triangle count of a solid by merging edges shorter than tolerance.
func (sf *SolidFuture) Simplify(tolerance float64) *SolidFuture {
	return startSolidFuture(func() (*Solid, error) {
		s, err := sf.Resolve()
		if err != nil {
			return nil, err
		}
		return simplify(s, tolerance), nil
	})
}

// RefineToLength subdivides edges longer than the given length.
func (sf *SolidFuture) RefineToLength(length float64) *SolidFuture {
	return startSolidFuture(func() (*Solid, error) {
		s, err := sf.Resolve()
		if err != nil {
			return nil, err
		}
		return refineToLength(s, length), nil
	})
}

// SetColor sets a uniform RGB color on all vertices.
func (sf *SolidFuture) SetColor(r, g, b float64) *SolidFuture {
	return startSolidFuture(func() (*Solid, error) {
		s, err := sf.Resolve()
		if err != nil {
			return nil, err
		}
		return setColor(s, r, g, b), nil
	})
}

// SplitSolid splits m by cutter, returning [inside, outside].
// Resolves eagerly because the count is fixed at 2.
func SplitSolid(m, cutter *SolidFuture) ([]*SolidFuture, error) {
	sm, err := m.Resolve()
	if err != nil {
		return nil, err
	}
	sc, err := cutter.Resolve()
	if err != nil {
		return nil, err
	}
	first, second := splitSolid(sm, sc)
	return []*SolidFuture{ImmediateSolid(first), ImmediateSolid(second)}, nil
}

// SplitSolidByPlane splits a solid by an infinite plane, returning [above, below].
// Resolves eagerly because the count is fixed at 2.
func SplitSolidByPlane(sf *SolidFuture, nx, ny, nz, offset float64) ([]*SolidFuture, error) {
	s, err := sf.Resolve()
	if err != nil {
		return nil, err
	}
	first, second := splitByPlane(s, nx, ny, nz, offset)
	return []*SolidFuture{ImmediateSolid(first), ImmediateSolid(second)}, nil
}

// Warp deforms each vertex of a solid using the given function.
// The function receives the current position and returns the new position.
func (sf *SolidFuture) Warp(fn func(x, y, z float64) (float64, float64, float64)) *SolidFuture {
	return startSolidFuture(func() (*Solid, error) {
		s, err := sf.Resolve()
		if err != nil {
			return nil, err
		}
		return warpSolid(s, fn), nil
	})
}

// LevelSet creates a solid from a signed-distance-field (SDF) function.
// The SDF returns negative values inside the solid and positive outside.
// bounds defines the region to sample; edgeLen controls mesh resolution.
func LevelSet(fn func(x, y, z float64) float64, minX, minY, minZ, maxX, maxY, maxZ, edgeLen float64) *SolidFuture {
	return startSolidFuture(func() (*Solid, error) {
		return levelSetSolid(fn, minX, minY, minZ, maxX, maxY, maxZ, edgeLen), nil
	})
}

// ComposeSolids assembles non-overlapping solids into one without boolean operations.
func ComposeSolids(futures []*SolidFuture) *SolidFuture {
	return startSolidFuture(func() (*Solid, error) {
		if len(futures) == 0 {
			return nil, fmt.Errorf("ComposeSolids requires at least 1 solid")
		}
		solids := make([]*Solid, len(futures))
		for i, f := range futures {
			s, err := f.Resolve()
			if err != nil {
				return nil, err
			}
			solids[i] = s
		}
		return composeSolids(solids), nil
	})
}

// Hull computes the convex hull of a solid.
func (sf *SolidFuture) Hull() *SolidFuture {
	return startSolidFuture(func() (*Solid, error) {
		s, err := sf.Resolve()
		if err != nil {
			return nil, err
		}
		return hull(s), nil
	})
}

// Union computes the boolean union of two solid futures.
func (sf *SolidFuture) Union(b *SolidFuture) *SolidFuture {
	return startSolidFuture(func() (*Solid, error) {
		sa, err := sf.Resolve()
		if err != nil {
			return nil, err
		}
		sb, err := b.Resolve()
		if err != nil {
			return nil, err
		}
		return union(sa, sb), nil
	})
}

// Difference computes the boolean difference of two solid futures.
func (sf *SolidFuture) Difference(b *SolidFuture) *SolidFuture {
	return startSolidFuture(func() (*Solid, error) {
		sa, err := sf.Resolve()
		if err != nil {
			return nil, err
		}
		sb, err := b.Resolve()
		if err != nil {
			return nil, err
		}
		return difference(sa, sb), nil
	})
}

// Intersection computes the boolean intersection of two solid futures.
func (sf *SolidFuture) Intersection(b *SolidFuture) *SolidFuture {
	return startSolidFuture(func() (*Solid, error) {
		sa, err := sf.Resolve()
		if err != nil {
			return nil, err
		}
		sb, err := b.Resolve()
		if err != nil {
			return nil, err
		}
		return intersection(sa, sb), nil
	})
}

// Insert cuts a hole in sf for b, removes floating inner plugs, and seats b.
func (sf *SolidFuture) Insert(b *SolidFuture) *SolidFuture {
	return startSolidFuture(func() (*Solid, error) {
		sa, err := sf.Resolve()
		if err != nil {
			return nil, err
		}
		sb, err := b.Resolve()
		if err != nil {
			return nil, err
		}
		return insert(sa, sb), nil
	})
}

// Slice takes a cross-section of a solid at the given Z height.
func (sf *SolidFuture) Slice(height float64) *SketchFuture {
	return startSketchFuture(func() (*Sketch, error) {
		s, err := sf.Resolve()
		if err != nil {
			return nil, err
		}
		return slice(s, height), nil
	})
}

// Project projects a solid onto the XY plane.
func (sf *SolidFuture) Project() *SketchFuture {
	return startSketchFuture(func() (*Sketch, error) {
		s, err := sf.Resolve()
		if err != nil {
			return nil, err
		}
		return project(s), nil
	})
}

// ---------------------------------------------------------------------------
// SketchFuture methods
// ---------------------------------------------------------------------------

// Translate moves a sketch by (x, y).
func (sf *SketchFuture) Translate(x, y float64) *SketchFuture {
	return startSketchFuture(func() (*Sketch, error) {
		s, err := sf.Resolve()
		if err != nil {
			return nil, err
		}
		return sketchTranslate(s, x, y), nil
	})
}

// Rotate rotates a sketch by degrees, pivoting on the bounding box center.
func (sf *SketchFuture) Rotate(degrees float64) *SketchFuture {
	return startSketchFuture(func() (*Sketch, error) {
		s, err := sf.Resolve()
		if err != nil {
			return nil, err
		}
		return sketchRotateLocal(s, degrees), nil
	})
}

// RotateOrigin rotates a sketch by degrees around the world origin (0, 0).
func (sf *SketchFuture) RotateOrigin(degrees float64) *SketchFuture {
	return startSketchFuture(func() (*Sketch, error) {
		s, err := sf.Resolve()
		if err != nil {
			return nil, err
		}
		return sketchRotate(s, degrees), nil
	})
}

// Scale scales a sketch by (x, y) around pivot point (px, py).
func (sf *SketchFuture) Scale(x, y, px, py float64) *SketchFuture {
	return startSketchFuture(func() (*Sketch, error) {
		s, err := sf.Resolve()
		if err != nil {
			return nil, err
		}
		return sketchScaleAt(s, x, y, px, py), nil
	})
}

// Mirror mirrors a sketch across the axis (ax, ay) at signed distance offset
// from the world origin.
func (sf *SketchFuture) Mirror(ax, ay, offset float64) *SketchFuture {
	return startSketchFuture(func() (*Sketch, error) {
		s, err := sf.Resolve()
		if err != nil {
			return nil, err
		}
		return sketchMirrorAt(s, ax, ay, offset), nil
	})
}

// Offset offsets a sketch's edges by delta with round join.
func (sf *SketchFuture) Offset(delta float64, segments int) *SketchFuture {
	return startSketchFuture(func() (*Sketch, error) {
		s, err := sf.Resolve()
		if err != nil {
			return nil, err
		}
		return sketchOffset(s, delta, segments), nil
	})
}

// Hull computes the convex hull of a sketch.
func (sf *SketchFuture) Hull() *SketchFuture {
	return startSketchFuture(func() (*Sketch, error) {
		s, err := sf.Resolve()
		if err != nil {
			return nil, err
		}
		return sketchHull(s), nil
	})
}

// Union computes the boolean union of two sketch futures.
func (sf *SketchFuture) Union(b *SketchFuture) *SketchFuture {
	return startSketchFuture(func() (*Sketch, error) {
		sa, err := sf.Resolve()
		if err != nil {
			return nil, err
		}
		sb, err := b.Resolve()
		if err != nil {
			return nil, err
		}
		return sketchUnion(sa, sb), nil
	})
}

// Difference computes the boolean difference of two sketch futures.
func (sf *SketchFuture) Difference(b *SketchFuture) *SketchFuture {
	return startSketchFuture(func() (*Sketch, error) {
		sa, err := sf.Resolve()
		if err != nil {
			return nil, err
		}
		sb, err := b.Resolve()
		if err != nil {
			return nil, err
		}
		return sketchDifference(sa, sb), nil
	})
}

// Intersection computes the boolean intersection of two sketch futures.
func (sf *SketchFuture) Intersection(b *SketchFuture) *SketchFuture {
	return startSketchFuture(func() (*Sketch, error) {
		sa, err := sf.Resolve()
		if err != nil {
			return nil, err
		}
		sb, err := b.Resolve()
		if err != nil {
			return nil, err
		}
		return sketchIntersection(sa, sb), nil
	})
}

// Extrude extrudes a sketch upward by height.
func (sf *SketchFuture) Extrude(height float64, slices int, twist, scaleX, scaleY float64) *SolidFuture {
	return startSolidFuture(func() (*Solid, error) {
		s, err := sf.Resolve()
		if err != nil {
			return nil, err
		}
		return extrude(s, height, slices, twist, scaleX, scaleY), nil
	})
}

// Revolve revolves a sketch around the Y axis.
func (sf *SketchFuture) Revolve(segments int, degrees float64) *SolidFuture {
	return startSolidFuture(func() (*Solid, error) {
		s, err := sf.Resolve()
		if err != nil {
			return nil, err
		}
		return revolve(s, segments, degrees), nil
	})
}

// Sweep extrudes a sketch along a 3D path.
func (sf *SketchFuture) Sweep(path []Point3D) *SolidFuture {
	return startSolidFuture(func() (*Solid, error) {
		if len(path) < 2 {
			return nil, fmt.Errorf("Sweep requires at least 2 path points, got %d", len(path))
		}
		s, err := sf.Resolve()
		if err != nil {
			return nil, err
		}
		return sweep(s, path), nil
	})
}
