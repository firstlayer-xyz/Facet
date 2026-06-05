//go:build !js

package manifold

import (
	"math"
	"testing"
)

// TestTextHalignShiftsBoundingBox pins the horizontal anchor contract:
// the same text at "left", "center", "right" produces the same shape
// (same width) but different x positions. Specifically the bbox should
// translate by exactly -width/2 (center) and -width (right) relative to
// the left-aligned baseline.
func TestTextHalignShiftsBoundingBox(t *testing.T) {
	font := DefaultFontData()
	const sample = "ABC"
	const size = 10.0

	left, err := CreateText(font, sample, size, "left", "baseline")
	if err != nil {
		t.Fatalf("CreateText left: %v", err)
	}
	center, err := CreateText(font, sample, size, "center", "baseline")
	if err != nil {
		t.Fatalf("CreateText center: %v", err)
	}
	right, err := CreateText(font, sample, size, "right", "baseline")
	if err != nil {
		t.Fatalf("CreateText right: %v", err)
	}

	lMinX, _, lMaxX, _ := left.BoundingBox()
	cMinX, _, cMaxX, _ := center.BoundingBox()
	rMinX, _, rMaxX, _ := right.BoundingBox()

	leftWidth := lMaxX - lMinX
	if math.Abs((cMaxX-cMinX)-leftWidth) > 1e-6 {
		t.Errorf("center width %.6f != left width %.6f", cMaxX-cMinX, leftWidth)
	}
	if math.Abs((rMaxX-rMinX)-leftWidth) > 1e-6 {
		t.Errorf("right width %.6f != left width %.6f", rMaxX-rMinX, leftWidth)
	}

	// Advance width: distance from the original origin (0) to the end of
	// the last glyph's advance. The bbox minX is close to but not exactly
	// 0 (depends on side-bearing), so anchor offsets reference cMinX/rMinX
	// relative to lMinX.
	centerShift := cMinX - lMinX
	rightShift := rMinX - lMinX

	// Right shift should be roughly twice the center shift (within glyph
	// bearing slop) — both negative.
	if centerShift >= 0 || rightShift >= 0 {
		t.Errorf("expected negative shifts: center=%.4f right=%.4f", centerShift, rightShift)
	}
	ratio := rightShift / centerShift
	if ratio < 1.8 || ratio > 2.2 {
		t.Errorf("right shift should be ~2× center shift, got ratio=%.3f (center=%.4f right=%.4f)",
			ratio, centerShift, rightShift)
	}
}

// TestTextValignShiftsBoundingBox pins the vertical anchor contract:
// "top" puts the top of caps at or below y=0; "bottom" puts descender
// bottom at or above y=0; "center" lands somewhere between the two.
func TestTextValignShiftsBoundingBox(t *testing.T) {
	font := DefaultFontData()
	const sample = "Apg" // ascender, cap, descender
	const size = 10.0

	baseline, err := CreateText(font, sample, size, "left", "baseline")
	if err != nil {
		t.Fatal(err)
	}
	top, err := CreateText(font, sample, size, "left", "top")
	if err != nil {
		t.Fatal(err)
	}
	bottom, err := CreateText(font, sample, size, "left", "bottom")
	if err != nil {
		t.Fatal(err)
	}
	center, err := CreateText(font, sample, size, "left", "center")
	if err != nil {
		t.Fatal(err)
	}

	_, bMinY, _, bMaxY := baseline.BoundingBox()
	_, _, _, tMaxY := top.BoundingBox()
	_, botMinY, _, _ := bottom.BoundingBox()
	_, cMinY, _, cMaxY := center.BoundingBox()

	// Baseline-anchored: glyph top above 0 (ascenders), descender below 0.
	if bMinY >= 0 || bMaxY <= 0 {
		t.Errorf("baseline anchor: expected bbox to straddle y=0, got [%.3f, %.3f]", bMinY, bMaxY)
	}
	// Top-anchored: maxY should be ≤ 0 (we shifted down by ascender), and
	// the bbox should sit entirely at or below y=0 modulo a small slop.
	if tMaxY > 1e-6 {
		t.Errorf("top anchor: expected maxY ≤ 0, got %.3f", tMaxY)
	}
	// Bottom-anchored: minY should be ≥ 0 (descender bottom now at y=0).
	if botMinY < -1e-6 {
		t.Errorf("bottom anchor: expected minY ≥ 0, got %.3f", botMinY)
	}
	// Center-anchored: y midpoint of the bbox should be near 0.
	mid := (cMinY + cMaxY) * 0.5
	// Slack: font asymmetry between ascender/descender vs actual glyph extent.
	if math.Abs(mid) > 0.5*size {
		t.Errorf("center anchor: bbox midpoint y=%.3f not near 0", mid)
	}
}

// TestTextRejectsUnknownAlign confirms that a bogus halign/valign value
// produces an error rather than silently picking a default.
func TestTextRejectsUnknownAlign(t *testing.T) {
	font := DefaultFontData()
	if _, err := CreateText(font, "X", 10.0, "middle", "baseline"); err == nil {
		t.Error(`expected error for halign="middle", got nil`)
	}
	if _, err := CreateText(font, "X", 10.0, "left", "middle"); err == nil {
		t.Error(`expected error for valign="middle", got nil`)
	}
}

// TestTextEmptyAlignMatchesDefault confirms that "" for halign/valign
// behaves the same as "left"/"baseline" — so callers that pass an empty
// string get the documented default.
func TestTextEmptyAlignMatchesDefault(t *testing.T) {
	font := DefaultFontData()
	a, err := CreateText(font, "X", 10.0, "", "")
	if err != nil {
		t.Fatal(err)
	}
	b, err := CreateText(font, "X", 10.0, "left", "baseline")
	if err != nil {
		t.Fatal(err)
	}
	aMin0, aMin1, aMax0, aMax1 := a.BoundingBox()
	bMin0, bMin1, bMax0, bMax1 := b.BoundingBox()
	if math.Abs(aMin0-bMin0) > 1e-9 || math.Abs(aMin1-bMin1) > 1e-9 ||
		math.Abs(aMax0-bMax0) > 1e-9 || math.Abs(aMax1-bMax1) > 1e-9 {
		t.Errorf("empty-string anchors should equal default, got\n  empty: [%.6f,%.6f]-[%.6f,%.6f]\n  default: [%.6f,%.6f]-[%.6f,%.6f]",
			aMin0, aMin1, aMax0, aMax1, bMin0, bMin1, bMax0, bMax1)
	}
}
