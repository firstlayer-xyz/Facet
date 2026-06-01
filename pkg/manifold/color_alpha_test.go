//go:build !js

package manifold

import "testing"

// TestSetColorPersistsAlpha verifies SetColor stores alpha in FaceInfo
// alongside the packed RGB color.
func TestSetColorPersistsAlpha(t *testing.T) {
	c, err := CreateCube(10, 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	c.SetColor(1.0, 0.5, 0.25, 0.5)

	if len(c.FaceMap) == 0 {
		t.Fatal("cube has no FaceMap entries")
	}
	for id, fi := range c.FaceMap {
		// 0.5 * 255 + 0.5 rounds to 128.
		if fi.Alpha != 128 {
			t.Errorf("face %d: expected Alpha=128 (0.5), got %d", id, fi.Alpha)
		}
		// RGB packed as 0xRRGGBB. r=255 g=128 b=64.
		if fi.Color != 0xFF8040 {
			t.Errorf("face %d: expected Color=0xFF8040, got 0x%06X", id, fi.Color)
		}
	}
}

// TestColorFromFaceInfoSelectsHexLength locks the hex-string contract:
// opaque alpha collapses to "#RRGGBB", anything else expands to
// "#RRGGBBAA" so downstream consumers can tell the cases apart.
func TestColorFromFaceInfoSelectsHexLength(t *testing.T) {
	cases := []struct {
		name  string
		fi    FaceInfo
		wantS string
	}{
		{"opaque-default", FaceInfo{Color: 0xFF0000, Alpha: 0}, "#FF0000"},
		{"opaque-explicit", FaceInfo{Color: 0xFF0000, Alpha: 0xFF}, "#FF0000"},
		{"half-alpha", FaceInfo{Color: 0xFF0000, Alpha: 0x80}, "#FF000080"},
		{"transparent", FaceInfo{Color: 0x00FF00, Alpha: 0x00}, "#00FF00"}, // Alpha=0 means "default opaque" per FaceInfo doc
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := colorFromFaceInfo(tc.fi)
			if got != tc.wantS {
				t.Errorf("colorFromFaceInfo: got %q, want %q", got, tc.wantS)
			}
		})
	}
}
