//go:build !js

package manifold

import (
	"math"
	"testing"
)

// TestPolygonHolesCutsArea verifies the holes argument actually removes
// area from the sketch. A 10×10 outer square with a 2×2 hole has area 96.
func TestPolygonHolesCutsArea(t *testing.T) {
	outer := []Point2D{{0, 0}, {10, 0}, {10, 10}, {0, 10}}
	hole := []Point2D{{4, 4}, {6, 4}, {6, 6}, {4, 6}}

	sk, err := CreatePolygon(outer, [][]Point2D{hole})
	if err != nil {
		t.Fatalf("CreatePolygon: %v", err)
	}

	got := sk.Area()
	const want = 96.0
	if math.Abs(got-want) > 1e-6 {
		t.Fatalf("area with hole: got %.6f, want %.6f", got, want)
	}
}

// TestPolygonHolesEmptyMatchesPlain verifies passing nil/empty holes is
// equivalent to a plain polygon — the holes parameter doesn't change
// the result when there are no holes.
func TestPolygonHolesEmptyMatchesPlain(t *testing.T) {
	outer := []Point2D{{0, 0}, {5, 0}, {5, 5}, {0, 5}}

	plain, err := CreatePolygon(outer, nil)
	if err != nil {
		t.Fatalf("CreatePolygon(nil holes): %v", err)
	}
	empty, err := CreatePolygon(outer, [][]Point2D{})
	if err != nil {
		t.Fatalf("CreatePolygon(empty holes): %v", err)
	}

	if math.Abs(plain.Area()-25.0) > 1e-6 {
		t.Fatalf("plain area: got %.6f, want 25", plain.Area())
	}
	if math.Abs(empty.Area()-25.0) > 1e-6 {
		t.Fatalf("empty-holes area: got %.6f, want 25", empty.Area())
	}
}

// TestPolygonHolesIgnoresWinding verifies the EvenOdd fill rule makes
// winding direction irrelevant: a hole works whether its points are
// listed CW or CCW.
func TestPolygonHolesIgnoresWinding(t *testing.T) {
	outer := []Point2D{{0, 0}, {10, 0}, {10, 10}, {0, 10}} // CCW
	holeCCW := []Point2D{{4, 4}, {6, 4}, {6, 6}, {4, 6}}
	holeCW := []Point2D{{4, 4}, {4, 6}, {6, 6}, {6, 4}} // reversed

	skCCW, err := CreatePolygon(outer, [][]Point2D{holeCCW})
	if err != nil {
		t.Fatal(err)
	}
	skCW, err := CreatePolygon(outer, [][]Point2D{holeCW})
	if err != nil {
		t.Fatal(err)
	}

	if math.Abs(skCCW.Area()-skCW.Area()) > 1e-6 {
		t.Fatalf("winding affects area: CCW=%.6f CW=%.6f", skCCW.Area(), skCW.Area())
	}
}
