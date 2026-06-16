// Package meshpreview loads mesh files (.stl/.obj/.3mf) into the renderer's
// inputs — expanded triangle positions plus an optional per-expanded-vertex RGB
// color buffer — for thumbnail/preview rendering. It reads raw triangles via
// meshio (no manifold validation), so open or non-watertight meshes still
// render.
package meshpreview

import (
	"fmt"

	"facet/pkg/manifold"

	"github.com/firstlayer-xyz/meshio"
)

// LoadColored reads a mesh file and returns expanded triangle positions (9
// floats per triangle) and a parallel per-expanded-vertex RGB color buffer (3
// bytes per vertex), or nil colors when the file carries no per-face color.
func LoadColored(path string) (positions []float32, colors []byte, err error) {
	m, err := meshio.Read(path)
	if err != nil {
		return nil, nil, err
	}
	positions, colors, err = meshToPreview(m)
	if err != nil {
		return nil, nil, fmt.Errorf("meshpreview: %s: %w", path, err)
	}
	if len(positions) == 0 {
		return nil, nil, fmt.Errorf("meshpreview: %s contains no triangles", path)
	}
	return positions, colors, nil
}

// meshToPreview expands an indexed meshio.Mesh into renderer inputs. colors is
// nil when the mesh has no per-face color; otherwise each triangle's color is
// repeated across its three expanded vertices, with DefaultFaceColor for any
// face whose hex is empty or unparseable. It errors on a triangle index that is
// out of range or a face-color count that doesn't match the triangle count — the
// decoders don't validate these and this runs over arbitrary browsed files.
func meshToPreview(m *meshio.Mesh) (positions []float32, colors []byte, err error) {
	nTri := len(m.Indices) / 3
	if nTri == 0 {
		return nil, nil, nil
	}
	nVert := uint32(len(m.Vertices) / 3)
	positions = make([]float32, 0, nTri*9)
	for _, idx := range m.Indices {
		if idx >= nVert {
			return nil, nil, fmt.Errorf("triangle index %d out of range (%d vertices)", idx, nVert)
		}
		b := idx * 3
		positions = append(positions, m.Vertices[b], m.Vertices[b+1], m.Vertices[b+2])
	}

	if len(m.FaceColors) == 0 {
		return positions, nil, nil
	}
	if len(m.FaceColors) != nTri {
		return nil, nil, fmt.Errorf("%d face colors for %d triangles", len(m.FaceColors), nTri)
	}
	colors = make([]byte, nTri*9)
	for i := 0; i < nTri; i++ {
		c := manifold.DefaultFaceColor
		if rgb, ok := manifold.ParseHexRGB(m.FaceColors[i].Hex); ok {
			c = rgb
		}
		for v := 0; v < 3; v++ {
			o := (i*3 + v) * 3
			colors[o], colors[o+1], colors[o+2] = c[0], c[1], c[2]
		}
	}
	return positions, colors, nil
}
