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

// firstFaceColor returns the first explicitly-assigned color across the given
// solids' face maps, or NoColor if none is colored. Hull/offset operations
// produce new geometry and carry an input color onto the result.
func firstFaceColor(solids ...*Solid) uint32 {
	for _, s := range solids {
		for _, v := range s.FaceMap {
			if v.Color != NoColor {
				return v.Color
			}
		}
	}
	return NoColor
}

// seedHullFaceMap assigns color to the result's single originalID face. A
// negative originalID means the kernel did not mark an original, so the map is
// left empty — uint32(negative) would wrap into a garbage key.
func seedHullFaceMap(r *Solid, originalID int, color uint32) {
	if originalID >= 0 {
		r.FaceMap = map[uint32]FaceInfo{uint32(originalID): {Color: color}}
	}
}
