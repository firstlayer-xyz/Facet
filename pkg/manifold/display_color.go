package manifold

import (
	"encoding/binary"
	"math"
	"strconv"
	"strings"
)

// DefaultFaceColor is the fallback RGB for expanded vertices whose face has no
// assigned color, within a mesh that carries color on at least one face. It
// matches the web viewer's legacy flat color (0.55, 0.7, 0.88).
var DefaultFaceColor = [3]byte{0x8c, 0xb3, 0xe0} // 140, 179, 224

// ParseHexRGB parses "#RRGGBB" or "#RRGGBBAA" (alpha ignored) into RGB bytes.
func ParseHexRGB(s string) ([3]byte, bool) {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 && len(s) != 8 {
		return [3]byte{}, false
	}
	v, err := strconv.ParseUint(s[:6], 16, 32)
	if err != nil {
		return [3]byte{}, false
	}
	return [3]byte{byte(v >> 16), byte(v >> 8), byte(v)}, true
}

// ExpandedPositions decodes the little-endian float32 expanded position buffer
// into a []float32 (9 floats per triangle: three xyz verts).
func (dm *DisplayMesh) ExpandedPositions() []float32 {
	out := make([]float32, len(dm.ExpandedRaw)/4)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(dm.ExpandedRaw[i*4:]))
	}
	return out
}

// ExpandedColors returns a per-expanded-vertex RGB buffer (3 bytes per vertex)
// parallel to the expanded positions, or nil when no face carries a color.
// Each vertex takes its triangle's face color; faces with no assigned color
// fall back to DefaultFaceColor. Alpha is dropped (opaque rendering).
func (dm *DisplayMesh) ExpandedColors() []byte {
	if len(dm.FaceColorMap) == 0 {
		return nil
	}
	expVerts := dm.ExpandedCount
	if expVerts == 0 {
		return nil
	}
	rgb := make(map[uint32][3]byte, len(dm.FaceColorMap))
	for k, hex := range dm.FaceColorMap {
		id, err := strconv.ParseUint(k, 10, 32)
		if err != nil {
			continue
		}
		if c, ok := ParseHexRGB(hex); ok {
			rgb[uint32(id)] = c
		}
	}

	// FaceGroupRaw carries one uint32 face id per triangle (the common case)
	// or per expanded vertex; detect which so we index it correctly.
	fgN := len(dm.FaceGroupRaw) / 4
	perVertex := fgN == expVerts
	faceID := func(vert int) (uint32, bool) {
		idx := vert
		if !perVertex {
			idx = vert / 3
		}
		off := idx * 4
		if off+4 > len(dm.FaceGroupRaw) {
			return 0, false
		}
		return binary.LittleEndian.Uint32(dm.FaceGroupRaw[off : off+4]), true
	}

	out := make([]byte, expVerts*3)
	for v := 0; v < expVerts; v++ {
		c := DefaultFaceColor
		if fgN > 0 {
			if id, ok := faceID(v); ok {
				if cc, ok := rgb[id]; ok {
					c = cc
				}
			}
		}
		out[v*3], out[v*3+1], out[v*3+2] = c[0], c[1], c[2]
	}
	return out
}
