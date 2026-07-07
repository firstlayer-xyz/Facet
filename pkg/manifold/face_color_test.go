//go:build !js

package manifold

import "testing"

// TestFirstFaceInfoDeterministic pins the fix for map-iteration nondeterminism:
// after a boolean the FaceMap holds several colored originalIDs, and iterating
// it directly returned a different color per run (Go randomizes map order),
// so the same program rendered/exported in a different color each time. The
// colored face with the lowest ID must always win.
func TestFirstFaceInfoDeterministic(t *testing.T) {
	s := &Solid{FaceMap: map[uint32]FaceInfo{
		5: {Color: 0xFF0000, Alpha: 255},
		2: {Color: 0x0000FF, Alpha: 128},
		9: {Color: NoColor},
	}}
	for i := 0; i < 50; i++ {
		got := firstFaceInfo(s)
		if got.Color != 0x0000FF || got.Alpha != 128 {
			t.Fatalf("firstFaceInfo = %#06x/%d, want 0x0000ff/128 (lowest colored ID)", got.Color, got.Alpha)
		}
	}
}
