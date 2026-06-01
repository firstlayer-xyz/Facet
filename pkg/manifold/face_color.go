package manifold

// NoColor is the sentinel value for FaceInfo.Color indicating no color is assigned.
const NoColor uint32 = 0xFFFFFFFF

// FaceInfo holds per-face metadata keyed by Manifold originalID.
// Color is 0xRRGGBB (low 24 bits) or NoColor when no color is assigned.
// Alpha is the 0-255 opacity; only meaningful when Color != NoColor.
// Alpha == 0 means "default opaque" so the zero value of FaceInfo means
// "no color, default opacity" without needing an extra HasColor flag.
type FaceInfo struct {
	Color uint32
	Alpha uint8
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
