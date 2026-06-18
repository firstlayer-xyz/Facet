//go:build !js

package manifold

import (
	"math"
	"testing"
)

// cubeAt builds an axis-aligned cube of edge `edge`, corner-anchored then
// translated to (x,y,z).
func cubeAt(t *testing.T, edge, x, y, z float64) *Solid {
	t.Helper()
	c, err := CreateCube(edge, edge, edge)
	if err != nil {
		t.Fatalf("CreateCube(%v): %v", edge, err)
	}
	return c.Translate(x, y, z)
}

// pairwiseFoldSolids folds the operation over the accumulator exactly the way
// the evaluator did before BatchBoolean — the behaviour BatchBoolean must match.
func pairwiseFoldSolids(solids []*Solid, op BoolOp) *Solid {
	r := solids[0]
	for _, s := range solids[1:] {
		switch op {
		case OpUnion:
			r = r.Union(s)
		case OpDifference:
			r = r.Difference(s)
		default:
			r = r.Intersection(s)
		}
	}
	return r
}

func relClose(a, b float64) bool {
	const tol = 1e-6
	scale := math.Max(1, math.Max(math.Abs(a), math.Abs(b)))
	return math.Abs(a-b) <= tol*scale
}

func TestBatchBooleanEmpty(t *testing.T) {
	if _, err := BatchBoolean(nil, OpUnion); err == nil {
		t.Error("BatchBoolean(nil): expected error, got nil")
	}
	if _, err := SketchBatchBoolean(nil, OpUnion); err == nil {
		t.Error("SketchBatchBoolean(nil): expected error, got nil")
	}
}

// A single operand has no boolean to perform — it is returned unchanged. This
// is the count==1 path the relaxed guard and the pattern helpers rely on.
func TestBatchBooleanSingleOperand(t *testing.T) {
	for _, op := range []BoolOp{OpUnion, OpDifference, OpIntersection} {
		got, err := BatchBoolean([]*Solid{cubeAt(t, 2, 0, 0, 0)}, op)
		if err != nil {
			t.Fatalf("op %d: %v", op, err)
		}
		if !relClose(got.Volume(), 8) {
			t.Errorf("op %d: single-cube volume = %v, want 8", op, got.Volume())
		}
	}
}

// BatchBoolean must produce the same geometry (here measured by volume) as the
// pairwise fold it replaces, for every operation and operand count.
func TestBatchBooleanMatchesPairwiseFold(t *testing.T) {
	cases := []struct {
		name    string
		op      BoolOp
		build   func() []*Solid
		wantVol float64 // -1 = only compare against the fold, no closed-form anchor
	}{
		{"union-disjoint", OpUnion, func() []*Solid {
			return []*Solid{
				cubeAt(t, 1, 0, 0, 0), cubeAt(t, 1, 2, 0, 0),
				cubeAt(t, 1, 4, 0, 0), cubeAt(t, 1, 6, 0, 0),
			}
		}, 4},
		{"union-overlapping", OpUnion, func() []*Solid {
			return []*Solid{
				cubeAt(t, 2, 0, 0, 0), cubeAt(t, 2, 1, 0, 0), cubeAt(t, 2, 2, 0, 0),
			}
		}, -1},
		{"intersection-overlapping", OpIntersection, func() []*Solid {
			return []*Solid{
				cubeAt(t, 2, 0, 0, 0), cubeAt(t, 2, 1, 0, 0), cubeAt(t, 2, 0.5, 0.5, 0),
			}
		}, -1},
		{"difference-internal-cavities", OpDifference, func() []*Solid {
			return []*Solid{
				cubeAt(t, 10, 0, 0, 0),
				cubeAt(t, 2, 1, 1, 1), cubeAt(t, 2, 5, 1, 1), cubeAt(t, 2, 1, 5, 1),
			}
		}, 1000 - 24},
		// The 2-operand case the evaluator routes here when not on the fast path.
		{"union-pair", OpUnion, func() []*Solid {
			return []*Solid{cubeAt(t, 1, 0, 0, 0), cubeAt(t, 1, 2, 0, 0)}
		}, 2},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			solids := tc.build()
			want := pairwiseFoldSolids(solids, tc.op)
			got, err := BatchBoolean(solids, tc.op)
			if err != nil {
				t.Fatalf("BatchBoolean: %v", err)
			}
			if !relClose(got.Volume(), want.Volume()) {
				t.Errorf("batch volume %v != fold volume %v", got.Volume(), want.Volume())
			}
			if tc.wantVol >= 0 && !relClose(got.Volume(), tc.wantVol) {
				t.Errorf("batch volume %v != expected %v", got.Volume(), tc.wantVol)
			}
		})
	}
}

// A colored operand's color must survive the batch onto the result's face map,
// exactly as it does through the pairwise ops (mergeFaceMaps).
func TestBatchBooleanPreservesColor(t *testing.T) {
	red := cubeAt(t, 2, 0, 0, 0).SetColor(1, 0, 0, 1) // 0xFF0000
	got, err := BatchBoolean([]*Solid{red, cubeAt(t, 2, 3, 0, 0), cubeAt(t, 2, 6, 0, 0)}, OpUnion)
	if err != nil {
		t.Fatal(err)
	}
	if c := firstFaceColor(got); c != 0xFF0000 {
		t.Errorf("result lost color: firstFaceColor = 0x%06X, want 0xFF0000", c)
	}
}

// ── 2D (Sketch) batch booleans ────────────────────────────────────────────────

func squareAt(t *testing.T, edge, x, y float64) *Sketch {
	t.Helper()
	s, err := CreateSquare(edge, edge)
	if err != nil {
		t.Fatalf("CreateSquare(%v): %v", edge, err)
	}
	return s.Translate(x, y)
}

func pairwiseFoldSketches(sketches []*Sketch, op BoolOp) *Sketch {
	r := sketches[0]
	for _, s := range sketches[1:] {
		switch op {
		case OpUnion:
			r = r.Union(s)
		case OpDifference:
			r = r.Difference(s)
		default:
			r = r.Intersection(s)
		}
	}
	return r
}

func TestSketchBatchBooleanSingleOperand(t *testing.T) {
	got, err := SketchBatchBoolean([]*Sketch{squareAt(t, 2, 0, 0)}, OpUnion)
	if err != nil {
		t.Fatal(err)
	}
	if !relClose(got.Area(), 4) {
		t.Errorf("single-square area = %v, want 4", got.Area())
	}
}

// disjointCubes builds n unit cubes spaced along x so a union has no internal
// boolean work — isolating the reduction-tree cost (fold vs batch).
func disjointCubes(b *testing.B, n int) []*Solid {
	b.Helper()
	solids := make([]*Solid, n)
	for i := range solids {
		c, err := CreateCube(1, 1, 1)
		if err != nil {
			b.Fatalf("CreateCube: %v", err)
		}
		solids[i] = c.Translate(float64(2*i), 0, 0)
	}
	return solids
}

// BenchmarkUnionFold reduces pairwise over a growing accumulator: O(N^2) work as
// every step re-meshes the whole union-so-far.
func BenchmarkUnionFold(b *testing.B) {
	solids := disjointCubes(b, 64)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pairwiseFoldSolidsBench(solids, OpUnion)
	}
}

// BenchmarkUnionBatch hands the whole set to the kernel's one tree-reduction.
func BenchmarkUnionBatch(b *testing.B) {
	solids := disjointCubes(b, 64)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := BatchBoolean(solids, OpUnion); err != nil {
			b.Fatal(err)
		}
	}
}

func pairwiseFoldSolidsBench(solids []*Solid, op BoolOp) *Solid {
	r := solids[0]
	for _, s := range solids[1:] {
		r = r.Union(s)
	}
	return r
}

func TestSketchBatchBooleanMatchesPairwiseFold(t *testing.T) {
	cases := []struct {
		name    string
		op      BoolOp
		build   func() []*Sketch
		wantA   float64
	}{
		{"union-disjoint", OpUnion, func() []*Sketch {
			return []*Sketch{squareAt(t, 1, 0, 0), squareAt(t, 1, 2, 0), squareAt(t, 1, 4, 0)}
		}, 3},
		{"intersection-overlapping", OpIntersection, func() []*Sketch {
			return []*Sketch{squareAt(t, 2, 0, 0), squareAt(t, 2, 1, 0), squareAt(t, 2, 0.5, 0.5)}
		}, -1},
		{"difference-internal-holes", OpDifference, func() []*Sketch {
			return []*Sketch{squareAt(t, 10, 0, 0), squareAt(t, 2, 1, 1), squareAt(t, 2, 5, 1)}
		}, 100 - 8},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sketches := tc.build()
			want := pairwiseFoldSketches(sketches, tc.op)
			got, err := SketchBatchBoolean(sketches, tc.op)
			if err != nil {
				t.Fatalf("SketchBatchBoolean: %v", err)
			}
			if !relClose(got.Area(), want.Area()) {
				t.Errorf("batch area %v != fold area %v", got.Area(), want.Area())
			}
			if tc.wantA >= 0 && !relClose(got.Area(), tc.wantA) {
				t.Errorf("batch area %v != expected %v", got.Area(), tc.wantA)
			}
		})
	}
}
