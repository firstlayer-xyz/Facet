//go:build !js

package manifold

import (
	"math"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// SetColor channel clamping (bug D)
// ---------------------------------------------------------------------------

// TestSetColorClampsAllChannels confirms that out-of-range RGB channels are
// clamped to [0, 1] before quantization. Pre-fix, only alpha was clamped via
// clamp01, so a negative R or a >1 G would wrap through int<<16/<<8 into a
// completely wrong color (or, with negative values, sign-extend into the
// upper bits of the uint32 and pollute neighbouring channels).
func TestSetColorClampsAllChannels(t *testing.T) {
	cube, err := CreateCube(10, 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	colored := cube.SetColor(2.0, -0.5, 1.5, 0.5)
	for id, fi := range colored.FaceMap {
		// All three channels clamped: R=1.0→255, G=0→0, B=1.0→255.
		// Color is 0xRRGGBB (low 24 bits).
		const wantColor = uint32(0xFF00FF)
		if fi.Color != wantColor {
			t.Errorf("face %d: color = 0x%X, want 0x%X", id, fi.Color, wantColor)
		}
		// Alpha 0.5 → 128 (within rounding).
		if fi.Alpha != 128 {
			t.Errorf("face %d: alpha = %d, want 128", id, fi.Alpha)
		}
	}
}

// TestSetColorNoStrayHighBits is the targeted regression for the bit-wrap.
// Pre-fix, `int(r*255+0.5)<<16` with r=2 produced 510<<16 = 0x1FE0000, which
// when reinterpreted as the 0xRRGGBB color leaked a 1-bit into bit 24 — that
// later confused downstream consumers that masked with 0xFFFFFF assuming the
// high byte was zero.
func TestSetColorNoStrayHighBits(t *testing.T) {
	cube, err := CreateCube(10, 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	colored := cube.SetColor(2.0, 2.0, 2.0, 1.0)
	for id, fi := range colored.FaceMap {
		if fi.Color>>24 != 0 {
			t.Errorf("face %d: high byte of color must be 0, got 0x%X", id, fi.Color)
		}
	}
}

// ---------------------------------------------------------------------------
// requireSolids panic guard (bug F + others)
// ---------------------------------------------------------------------------

func mustPanic(t *testing.T, op string, fn func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("%s: expected panic, got none", op)
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("%s: panic value is %T, want string", op, r)
		}
		if !strings.Contains(msg, "is nil") {
			t.Fatalf("%s: panic message %q should mention nil", op, msg)
		}
	}()
	fn()
}

func TestBooleanOpsRejectNilOperand(t *testing.T) {
	cube, err := CreateCube(10, 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	mustPanic(t, "Union(nil)", func() { cube.Union(nil) })
	mustPanic(t, "Difference(nil)", func() { cube.Difference(nil) })
	mustPanic(t, "Intersection(nil)", func() { cube.Intersection(nil) })
	mustPanic(t, "Insert(nil)", func() { cube.Insert(nil) })
}

func TestHullRejectsNilReceiver(t *testing.T) {
	var s *Solid
	mustPanic(t, "Hull on nil", func() { s.Hull() })
}

func TestBatchHullRejectsNilEntry(t *testing.T) {
	cube, err := CreateCube(10, 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	mustPanic(t, "BatchHull with nil entry", func() {
		// Discard the (error, *Solid) — the panic fires before either returns.
		_, _ = BatchHull([]*Solid{cube, nil})
	})
}

func TestSplitSolidRejectsNilOperands(t *testing.T) {
	cube, err := CreateCube(10, 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	mustPanic(t, "SplitSolid(nil, cutter)", func() { SplitSolid(nil, cube) })
	mustPanic(t, "SplitSolid(m, nil)", func() { SplitSolid(cube, nil) })
}

// ---------------------------------------------------------------------------
// SplitSolid face-map propagation (bug O)
// ---------------------------------------------------------------------------

// TestSplitSolidFaceMapFromBodyOnly confirms both halves of a split take their
// FaceMap from the body (`m`) alone, not a merge of m + cutter. Pre-fix, both
// the inside and outside pieces received `mergeFaceMaps(m, cutter)`, so the
// cutter's face IDs leaked onto the outside half — visually surprising if the
// cutter had been SetColor'd a distinct color.
func TestSplitSolidFaceMapFromBodyOnly(t *testing.T) {
	body, err := CreateCube(10, 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	cutter, err := CreateCube(4, 4, 20)
	if err != nil {
		t.Fatal(err)
	}
	cutter = cutter.Translate(3, 3, -5)

	bodyIDs := make(map[uint32]bool, len(body.FaceMap))
	for id := range body.FaceMap {
		bodyIDs[id] = true
	}
	cutterIDs := make(map[uint32]bool, len(cutter.FaceMap))
	for id := range cutter.FaceMap {
		cutterIDs[id] = true
	}
	if len(bodyIDs) == 0 || len(cutterIDs) == 0 {
		t.Fatal("test setup: both cubes should seed FaceMaps")
	}

	halves := SplitSolid(body, cutter)
	for which, half := range halves {
		if half == nil {
			continue
		}
		for id := range half.FaceMap {
			if cutterIDs[id] && !bodyIDs[id] {
				t.Errorf("half %d carries cutter face ID %d (must not propagate)", which, id)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// DecomposeSolid nil-entry guard (bug P)
// ---------------------------------------------------------------------------

// TestDecomposeSolidNoNilDeref confirms DecomposeSolid does not panic and
// produces well-formed components. The earlier code would dereference the
// result of newSolid (which is nil for a nil C ptr) before checking. We
// can't easily fabricate a nil-ptr return from the real kernel, so this
// test exercises the normal path and confirms every returned component has
// a non-nil ptr — i.e. no nil entries leaked through.
func TestDecomposeSolidNoNilDeref(t *testing.T) {
	a, err := CreateCube(5, 5, 5)
	if err != nil {
		t.Fatal(err)
	}
	b, err := CreateCube(5, 5, 5)
	if err != nil {
		t.Fatal(err)
	}
	b = b.Translate(20, 0, 0)
	combined, err := ComposeSolids([]*Solid{a, b})
	if err != nil {
		t.Fatal(err)
	}
	parts := DecomposeSolid(combined)
	if len(parts) == 0 {
		t.Fatal("DecomposeSolid returned no parts")
	}
	for i, p := range parts {
		if p == nil {
			t.Errorf("part %d is nil", i)
			continue
		}
		if p.ptr == nil {
			t.Errorf("part %d has nil ptr", i)
		}
	}
}

// ---------------------------------------------------------------------------
// Hull / BatchHull original_id guard (bug H)
// ---------------------------------------------------------------------------

// TestHullFaceMapKeyMatchesOriginalID confirms Hull's seeded FaceMap key is
// the kernel-reported original_id and never the wraparound of a negative
// sentinel through uint32. The kernel currently always returns a valid id
// for Hull, so this asserts the well-formed-id path stays consistent and
// that no spurious 0xFFFFFFFF key sneaks in.
func TestHullFaceMapKeyMatchesOriginalID(t *testing.T) {
	cube, err := CreateCube(10, 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	h := cube.Hull()
	if h == nil {
		t.Fatal("Hull returned nil")
	}
	// Hull always seeds at most one FaceMap entry (or none if original_id < 0).
	if len(h.FaceMap) > 1 {
		t.Fatalf("Hull FaceMap should have at most one entry, got %d", len(h.FaceMap))
	}
	for id := range h.FaceMap {
		// 0xFFFFFFFF would be the wraparound of -1, which we now guard against.
		if id == 0xFFFFFFFF {
			t.Errorf("Hull FaceMap key is 0xFFFFFFFF — sentinel-wrap regression")
		}
	}
}

// ---------------------------------------------------------------------------
// 2D primitive input validation (bug Q)
// ---------------------------------------------------------------------------

func TestCreateSquareRejectsNonPositive(t *testing.T) {
	cases := [][2]float64{{0, 1}, {1, 0}, {-1, 1}, {1, -1}}
	for _, c := range cases {
		_, err := CreateSquare(c[0], c[1])
		if err == nil {
			t.Errorf("CreateSquare(%v): expected error", c)
			continue
		}
		if !strings.Contains(err.Error(), "positive") {
			t.Errorf("CreateSquare(%v): error should mention positive: %v", c, err)
		}
	}
}

func TestCreateCircleRejectsNonPositive(t *testing.T) {
	for _, r := range []float64{0, -1, -1e-12} {
		_, err := CreateCircle(r, 32)
		if err == nil {
			t.Errorf("CreateCircle(%g): expected error", r)
			continue
		}
		if !strings.Contains(err.Error(), "positive") {
			t.Errorf("CreateCircle(%g): error should mention positive: %v", r, err)
		}
	}
}

// ---------------------------------------------------------------------------
// Mirror length check (bug M)
// ---------------------------------------------------------------------------

// TestMirrorRejectsNearZeroNormal confirms that a normal vector too short to
// normalize stably is rejected, not just the exactly-zero vector. Pre-fix
// the guard was `nx == 0 && ny == 0 && nz == 0`, so (1e-200, 0, 0) slipped
// through and the C-side normalization produced numerical noise.
func TestMirrorRejectsNearZeroNormal(t *testing.T) {
	cube, err := CreateCube(10, 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	_, err = cube.Mirror(1e-200, 0, 0, 0)
	if err == nil {
		t.Fatal("expected error for sub-epsilon normal")
	}
	if !strings.Contains(err.Error(), "degenerate") {
		t.Errorf("error should mention degenerate: %v", err)
	}

	// NaN normal must also error.
	_, err = cube.Mirror(math.NaN(), 0, 1, 0)
	if err == nil {
		t.Fatal("expected error for NaN normal")
	}
}

// ---------------------------------------------------------------------------
// Scale NaN/Inf rejection (bug R)
// ---------------------------------------------------------------------------

func TestSolidScaleRejectsNaNAndInf(t *testing.T) {
	cube, err := CreateCube(10, 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	bad := []float64{math.NaN(), math.Inf(1), math.Inf(-1)}
	for _, f := range bad {
		_, err := cube.Scale(f, 1, 1, 0, 0, 0)
		if err == nil {
			t.Errorf("Scale(%g): expected error", f)
		}
	}
}

func TestSketchScaleRejectsNaNAndInf(t *testing.T) {
	sq, err := CreateSquare(10, 10)
	if err != nil {
		t.Fatal(err)
	}
	bad := []float64{math.NaN(), math.Inf(1), math.Inf(-1)}
	for _, f := range bad {
		_, err := sq.Scale(1, f, 0, 0)
		if err == nil {
			t.Errorf("Sketch.Scale(_, %g): expected error", f)
		}
	}
}

// ---------------------------------------------------------------------------
// mergeFaceMaps semantics + fast paths (bug K + cleanup)
// ---------------------------------------------------------------------------

// TestMergeFaceMapsAFullyOverrides documents the actual merge semantic:
// when both maps share a key, a's full FaceInfo wins (struct overwrite — not
// a per-field merge). The pre-fix doc comment claimed "color from a wins if
// set" which would imply b's color survives when a's is NoColor; the code
// never did that, so this test pins the real behavior.
func TestMergeFaceMapsAFullyOverrides(t *testing.T) {
	a := map[uint32]FaceInfo{1: {Color: NoColor, Alpha: 0}}
	b := map[uint32]FaceInfo{1: {Color: 0xFF0000, Alpha: 200}}
	m := mergeFaceMaps(a, b)
	got := m[1]
	if got.Color != NoColor || got.Alpha != 0 {
		t.Fatalf("expected a's entry to fully overwrite b, got %+v", got)
	}
}

// TestMergeFaceMapsFastPaths covers the single-side-empty branches.
func TestMergeFaceMapsFastPaths(t *testing.T) {
	a := map[uint32]FaceInfo{1: {Color: 0x111111, Alpha: 10}}

	bothEmpty := mergeFaceMaps(nil, nil)
	if bothEmpty != nil {
		t.Errorf("both-empty should return nil, got %v", bothEmpty)
	}

	onlyA := mergeFaceMaps(a, nil)
	if len(onlyA) != 1 || onlyA[1] != a[1] {
		t.Errorf("only-a should copy a, got %v", onlyA)
	}
	// Result must be a copy, not the same map.
	onlyA[2] = FaceInfo{Color: 0x222222}
	if _, leaked := a[2]; leaked {
		t.Error("only-a path returned aliased map, must be a copy")
	}

	onlyB := mergeFaceMaps(nil, a)
	if len(onlyB) != 1 || onlyB[1] != a[1] {
		t.Errorf("only-b should copy b, got %v", onlyB)
	}
}
