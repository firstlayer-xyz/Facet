package runner

import (
	"encoding/json"
	"errors"
	"math"

	"facet/app/pkg/fctlang/checker"
	"facet/app/pkg/fctlang/doc"
	"facet/app/pkg/fctlang/evaluator"
	"facet/app/pkg/fctlang/parser"
	"facet/app/pkg/manifold"
)

// RunResult captures the full outcome of a run: check data + optional eval data.
type RunResult struct {
	// Check results (always populated)
	Errors       []parser.SourceError `json:"errors,omitempty"`
	VarTypes     checker.VarTypeMap   `json:"-"`
	Declarations *checker.DeclResult  `json:"-"`
	EntryPoints  []EntryPoint         `json:"entryPoints,omitempty"`
	DocIndex     []doc.DocEntry       `json:"-"`

	// Eval results (populated when entryPoint != "")
	Success bool                  `json:"success"`
	Stats   *evaluator.ModelStats `json:"stats,omitempty"`
	Boxes   []SolidBBox           `json:"boundingBoxes,omitempty"`
	Time    float64               `json:"buildTime,omitempty"`

	// Not serialized — used by frontend events, export, debug stepping
	Mesh        *manifold.DisplayMesh  `json:"-"`
	PosMap      []evaluator.PosEntry   `json:"-"`
	Solids      []*manifold.Solid      `json:"-"`
	DebugResult *evaluator.DebugResult `json:"-"`
}

// JSON returns the RunResult as indented JSON.
func (r *RunResult) JSON() string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data)
}

// SolidBBox describes the axis-aligned bounding box of a single solid.
type SolidBBox struct {
	Min    [3]float64 `json:"min"`    // [x, y, z]
	Max    [3]float64 `json:"max"`    // [x, y, z]
	Pieces int        `json:"pieces"` // number of disconnected components
}

// Callbacks are called by the runner when build state changes.
type Callbacks struct {
	OnStart  func()
	OnIdle   func()
	OnResult func(result *RunResult)
}

// SourceErrorFromErr converts a generic error into a parser.SourceError.
func SourceErrorFromErr(err error) parser.SourceError {
	var se *parser.SourceError
	if errors.As(err, &se) {
		return *se
	}
	return parser.SourceError{Message: err.Error()}
}

// SolidBBoxes computes per-solid bounding boxes.
func SolidBBoxes(solids []*manifold.Solid) []SolidBBox {
	result := make([]SolidBBox, len(solids))
	for i, s := range solids {
		mnX, mnY, mnZ, mxX, mxY, mxZ := s.BoundingBox()
		result[i] = SolidBBox{
			Min:    [3]float64{sanitizeBBox(mnX), sanitizeBBox(mnY), sanitizeBBox(mnZ)},
			Max:    [3]float64{sanitizeBBox(mxX), sanitizeBBox(mxY), sanitizeBBox(mxZ)},
			Pieces: s.NumComponents(),
		}
	}
	return result
}

func sanitizeBBox(v float64) float64 {
	if math.IsInf(v, 0) || math.IsNaN(v) {
		return 0
	}
	return v
}

// RenderMeshes converts each solid to a display mesh.
func RenderMeshes(solids []*manifold.Solid) []*manifold.DisplayMesh {
	meshes := make([]*manifold.DisplayMesh, len(solids))
	for i, s := range solids {
		meshes[i] = s.ToDisplayMesh()
	}
	return meshes
}
