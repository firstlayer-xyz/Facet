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

// SourceEntry describes a parsed source file with its text and origin.
type SourceEntry struct {
	Text string            `json:"text"`
	Kind parser.SourceKind `json:"kind"`
}

// RunResult captures the full outcome of a run: check data + optional eval data.
type RunResult struct {
	// Check results (always populated)
	Errors       []parser.SourceError    `json:"errors,omitempty"`
	Sources      map[string]SourceEntry  `json:"sources,omitempty"`
	VarTypes     checker.VarTypeMap      `json:"-"`
	Declarations *checker.DeclResult     `json:"-"`
	EntryPoints  []EntryPoint            `json:"entryPoints,omitempty"`
	DocIndex     []doc.DocEntry          `json:"-"`

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

// sourceErrorFromErr converts a generic error into a parser.SourceError.
func sourceErrorFromErr(err error) parser.SourceError {
	var se *parser.SourceError
	if errors.As(err, &se) {
		return *se
	}
	return parser.SourceError{Message: err.Error()}
}

// solidBBoxes computes per-solid bounding boxes and the overall global bbox.
func solidBBoxes(solids []*manifold.Solid) (boxes []SolidBBox, globalMin, globalMax [3]float64) {
	boxes = make([]SolidBBox, len(solids))
	if len(solids) > 0 {
		globalMin = [3]float64{math.MaxFloat64, math.MaxFloat64, math.MaxFloat64}
		globalMax = [3]float64{-math.MaxFloat64, -math.MaxFloat64, -math.MaxFloat64}
	}
	for i, s := range solids {
		mnX, mnY, mnZ, mxX, mxY, mxZ := s.BoundingBox()
		boxes[i] = SolidBBox{
			Min:    [3]float64{sanitizeBBox(mnX), sanitizeBBox(mnY), sanitizeBBox(mnZ)},
			Max:    [3]float64{sanitizeBBox(mxX), sanitizeBBox(mxY), sanitizeBBox(mxZ)},
			Pieces: s.NumComponents(),
		}
		globalMin[0] = math.Min(globalMin[0], mnX)
		globalMin[1] = math.Min(globalMin[1], mnY)
		globalMin[2] = math.Min(globalMin[2], mnZ)
		globalMax[0] = math.Max(globalMax[0], mxX)
		globalMax[1] = math.Max(globalMax[1], mxY)
		globalMax[2] = math.Max(globalMax[2], mxZ)
	}
	return
}

func sanitizeBBox(v float64) float64 {
	if math.IsInf(v, 0) || math.IsNaN(v) {
		return 0
	}
	return v
}
