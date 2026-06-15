package manifold

import (
	"bytes"
	"fmt"

	"github.com/firstlayer-xyz/meshio"
)

// default3MFColor is applied to any triangle lacking an explicit color when a
// 3MF carries colors at all — slicers (OrcaSlicer/PrusaSlicer) ignore colors
// entirely if any triangle is left uncolored.
const default3MFColor = "#C0C0C0"

// EncodeSolidMesh serializes an indexed triangle mesh to the given format and
// returns the file bytes. vertices is flat xyz; indices are triangle corners
// (len % 3 == 0). faceHex holds one hex color per triangle ("" = uncolored);
// pass nil for no colors. STL is colorless by format and ignores faceHex.
// attachments are extra OPC parts embedded in the 3MF package; pass nil for
// none. Attachments are rejected for non-3mf formats.
//
// This is the single serialization path shared by the desktop file export and
// the browser download: the caller supplies an extracted mesh, this builds the
// meshio.Mesh and emits the format bytes.
func EncodeSolidMesh(vertices []float32, indices []uint32, faceHex []string, format string, attachments []meshio.Attachment) ([]byte, error) {
	if len(vertices) == 0 || len(indices) == 0 {
		return nil, fmt.Errorf("export failed: empty mesh")
	}
	if len(attachments) > 0 && format != "3mf" {
		return nil, fmt.Errorf("EncodeSolidMesh: attachments are only supported for 3mf, not %q", format)
	}
	m := &meshio.Mesh{Vertices: vertices, Indices: indices}
	var buf bytes.Buffer
	switch format {
	case "stl":
		if err := m.EncodeSTL(&buf); err != nil {
			return nil, err
		}
	case "3mf":
		m.FaceColors = faceColorsFromHex(faceHex, len(indices)/3, default3MFColor)
		m.Attachments = attachments
		if err := m.Encode3MF(&buf); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("EncodeSolidMesh: unsupported format %q", format)
	}
	return buf.Bytes(), nil
}

// faceColorsFromHex maps per-triangle hex colors to meshio.FaceColor, applying
// defaultHex to triangles whose hex is "". It returns nil when no triangle has
// a color, leaving the mesh uncolored.
func faceColorsFromHex(faceHex []string, numTris int, defaultHex string) []meshio.FaceColor {
	hasColor := false
	for _, h := range faceHex {
		if h != "" {
			hasColor = true
			break
		}
	}
	if !hasColor {
		return nil
	}
	colors := make([]meshio.FaceColor, numTris)
	for t := 0; t < numTris; t++ {
		hex := defaultHex
		if t < len(faceHex) && faceHex[t] != "" {
			hex = faceHex[t]
		}
		colors[t] = meshio.FaceColor{Hex: hex}
	}
	return colors
}
