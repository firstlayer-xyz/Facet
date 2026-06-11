package manifold

import (
	"math"
	"testing"
)

func sphereVol(r float64) float64 { return 4.0 / 3.0 * math.Pi * r * r * r }

// Offset(+delta) on a sphere of radius R yields ~ a sphere of radius R+delta.
func TestSolidOffsetGrowsSphere(t *testing.T) {
	s, err := CreateSphere(10, 64)
	if err != nil {
		t.Fatal(err)
	}
	got := s.Offset(2.0, 0.5).Volume()
	want := sphereVol(12.0)
	if math.Abs(got-want)/want > 0.05 {
		t.Errorf("Offset(+2) volume=%.1f want~%.1f (>5%% off)", got, want)
	}
}

// Offset(-delta) shrinks it.
func TestSolidOffsetShrinksSphere(t *testing.T) {
	s, err := CreateSphere(10, 64)
	if err != nil {
		t.Fatal(err)
	}
	got := s.Offset(-2.0, 0.5).Volume()
	want := sphereVol(8.0)
	if math.Abs(got-want)/want > 0.05 {
		t.Errorf("Offset(-2) volume=%.1f want~%.1f (>5%% off)", got, want)
	}
}
