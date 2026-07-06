package manifold

import "fmt"

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

// firstFaceInfo returns the color AND alpha of the first explicitly-colored face
// across the given solids, or the uncolored FaceInfo if none is colored. Ops
// that produce a single new original (Hull, Offset, Reidentify) carry an input
// face onto the result; carrying only the color would silently turn a
// translucent input opaque.
func firstFaceInfo(solids ...*Solid) FaceInfo {
	for _, s := range solids {
		for _, v := range s.FaceMap {
			if v.Color != NoColor {
				return v
			}
		}
	}
	return FaceInfo{Color: NoColor}
}

// seedHullFaceMap assigns a single face (color and alpha) to the result's one
// originalID. A negative originalID means the kernel did not mark an original,
// so the map is left empty — uint32(negative) would wrap into a garbage key.
func seedHullFaceMap(r *Solid, originalID int, face FaceInfo) {
	if originalID >= 0 {
		r.FaceMap = map[uint32]FaceInfo{uint32(originalID): face}
	}
}

// seedOriginFaceMap seeds a fresh single-entry FaceMap (uncolored) keyed by the
// kernel's originalID. A negative id means the kernel marked no original, so the
// map is left nil — uint32(negative) would wrap into a garbage key.
func seedOriginFaceMap(s *Solid, originalID int) {
	if originalID >= 0 {
		s.FaceMap = map[uint32]FaceInfo{uint32(originalID): {Color: NoColor}}
	}
}

// encodeColor quantizes float RGBA in [0,1] to a packed 0xRRGGBB color plus a
// 0-255 alpha. Every channel is clamped first so an out-of-range value cannot
// overflow its byte field and bleed into the next channel.
func encodeColor(r, g, b, a float64) (color uint32, alpha uint8) {
	ri := uint32(clamp01(r)*255 + 0.5)
	gi := uint32(clamp01(g)*255 + 0.5)
	bi := uint32(clamp01(b)*255 + 0.5)
	return ri<<16 | gi<<8 | bi, uint8(clamp01(a)*255 + 0.5)
}

// colorFromFaceInfo converts a FaceInfo color+alpha into a hex string.
// Emits "#RRGGBB" when alpha is 0 (default) or 255 (explicitly opaque),
// and "#RRGGBBAA" when alpha is anything else. Downstream consumers
// (renderer + meshio) both accept either form.
func colorFromFaceInfo(fi FaceInfo) string {
	c := fi.Color
	if fi.Alpha == 0 || fi.Alpha == 0xFF {
		return fmt.Sprintf("#%02X%02X%02X", (c>>16)&0xFF, (c>>8)&0xFF, c&0xFF)
	}
	return fmt.Sprintf("#%02X%02X%02X%02X", (c>>16)&0xFF, (c>>8)&0xFF, c&0xFF, fi.Alpha)
}

// buildFaceColorMap constructs a faceID→hex color map from a slice of face IDs
// and a FaceInfo map. Only face IDs with a non-NoColor entry are included.
func buildFaceColorMap(faceIDs []uint32, faceMap map[uint32]FaceInfo) map[string]string {
	var fcMap map[string]string
	seen := make(map[uint32]bool)
	for _, fid := range faceIDs {
		if seen[fid] {
			continue
		}
		seen[fid] = true
		if fi, ok := faceMap[fid]; ok && fi.Color != NoColor {
			if fcMap == nil {
				fcMap = make(map[string]string)
			}
			fcMap[fmt.Sprintf("%d", fid)] = colorFromFaceInfo(fi)
		}
	}
	return fcMap
}
