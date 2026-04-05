package evaluator

import (
	"context"
	"facet/app/pkg/fctlang/loader"
	"facet/app/pkg/fctlang/parser"
	"fmt"

	"facet/app/pkg/manifold"
)

// value represents a runtime value in the evaluator.
// It is either a float64, a length, a *manifold.Solid, or a *manifold.Sketch.
type value any

// constVal wraps a value to mark it as const (immutable binding).
// Assignment or field mutation on a constVal produces a runtime error.
type constVal struct {
	inner value
}

// constrainedVal wraps a value to attach a constraint to its binding.
// Reassignment to this binding re-validates the constraint.
type constrainedVal struct {
	inner      value
	constraint parser.Expr   // AST constraint expression for re-validation
	name       string // binding name, for error messages
}

// unwrap returns the underlying value, stripping any constVal and constrainedVal wrappers.
func unwrap(v value) value {
	for {
		switch w := v.(type) {
		case *constVal:
			v = w.inner
		case *constrainedVal:
			v = w.inner
		default:
			return v
		}
	}
}

// isConst reports whether a value is a const-wrapped binding.
func isConst(v value) bool {
	_, ok := v.(*constVal)
	return ok
}

// getConstraint returns the constrainedVal wrapper if present (looking through constVal).
func getConstraint(v value) (*constrainedVal, bool) {
	if cv, ok := v.(*constVal); ok {
		v = cv.inner
	}
	cv, ok := v.(*constrainedVal)
	return cv, ok
}

// length represents a dimensional value stored in millimeters.
type length struct {
	mm float64
}

// angle represents an angular value stored in degrees.
type angle struct {
	deg float64
}

// array represents a runtime array of values.
// elemType tracks the element type (e.g. "Solid", "Length", "Number").
// Empty string means untyped/heterogeneous. "var" means explicitly generic.
type array struct {
	elems    []value
	elemType string
}

// structVal represents a runtime user-defined struct instance.
type structVal struct {
	typeName string
	fields   map[string]value
	decl     *parser.StructDecl // declaration from the scope where this struct was defined
	lib      *libRef // non-nil if struct was created in a library context
}

// functionVal represents a first-class function (lambda) value.
// captured holds a snapshot of the local scope at the time the lambda was created.
type functionVal struct {
	params   []*parser.Param
	retType  string
	body     []parser.Stmt
	captured map[string]value
}

// DebugMesh captures a single mesh tagged with a role for debug visualization.
type DebugMesh struct {
	Role string
	Mesh *manifold.DisplayMesh
}

// debugShape is anything that can produce a renderable display mesh (Solid or Sketch).
type debugShape interface {
	ToDisplayMesh() *manifold.DisplayMesh
}

// debugEntry holds a role-tagged shape pointer for lazy mesh extraction.
type debugEntry struct {
	Role  string
	Shape debugShape
}

// DebugStep captures one geometry operation and its associated meshes.
type DebugStep struct {
	Op      string
	Meshes  []DebugMesh  // populated lazily by ResolveMeshes
	Line    int
	Col     int
	File    string       // disk path of the source file
	entries []debugEntry // unexported — holds shape ptrs until ResolveMeshes
}

// DebugResult holds the evaluated solids plus the step-by-step debug trace.
// Final is populated by callers (not the evaluator) with render meshes for display.
type DebugResult struct {
	Solids []*manifold.Solid
	Final  []*manifold.DisplayMesh // render meshes — populated by caller
	Steps  []DebugStep
	Files  map[string]string // path → source text (for editor display)
}

// ModelStats holds computed model statistics from evaluation.
type ModelStats struct {
	Triangles   int     `json:"triangles"`
	Vertices    int     `json:"vertices"`
	Volume      float64 `json:"volume"`      // mm³
	SurfaceArea float64 `json:"surfaceArea"` // mm²
	BBoxMin     [3]float64 `json:"bboxMin"`  // [x, y, z] min corner in mm
	BBoxMax     [3]float64 `json:"bboxMax"`  // [x, y, z] max corner in mm
}

// PosEntry maps a source position (file+line+col) to the face IDs of solids
// that were created or operated on at that position.
type PosEntry struct {
	File    string   `json:"file"`    // disk path of the source file
	Line    int      `json:"line"`
	Col     int      `json:"col"`
	FaceIDs []uint32 `json:"faceIDs"`
}

// SolidTrack records a source position and the Solid produced there.
type SolidTrack struct {
	File  string
	Line  int
	Col   int
	Solid *manifold.Solid
}

// EvalResult holds the evaluated solids and model statistics.
// Callers extract meshes as needed: ToMesh() for rendering, ExtractMeshShared() for export.
type EvalResult struct {
	Solids []*manifold.Solid
	Stats  ModelStats
	PosMap []PosEntry
}

// newEvaluator creates an evaluator with all shared fields initialized.
func newEvaluator(ctx context.Context, prog loader.Program, currentKey string, overrides map[string]interface{}, entryPoint string) *evaluator {
	tracks := make([]SolidTrack, 0)
	e := &evaluator{
		ctx:          ctx,
		prog:         prog,
		currentKey:   currentKey,
		entryPoint:   entryPoint,
		libEvalCache: make(map[string]map[string]value),
		libLoadStack: make(map[string]bool),
		libSources:   make(map[string]string),
		solidTracks:  &tracks,
	}
	if currentKey != "" {
		e.file = currentKey
	}
	if overrides != nil {
		e.overrides = convertOverrides(prog, currentKey, overrides, entryPoint)
	}
	return e
}

// Eval evaluates a parsed facet program. entryPoint must name a function that returns a Solid.
// Libraries must be resolved via ResolveLibraries before calling Eval.
// The context can be used to cancel evaluation mid-execution.
// overrides maps var names to raw values (JSON numbers/strings) for slider parameters.
func Eval(ctx context.Context, prog loader.Program, currentKey string, overrides map[string]interface{}, entryPoint string) (*EvalResult, error) {
	if entryPoint == "" {
		return nil, fmt.Errorf("entry point not set")
	}
	e := newEvaluator(ctx, prog, currentKey, overrides, entryPoint)
	return e.run()
}

// EvalDebug evaluates a parsed facet program in debug mode, capturing
// each geometry operation as a step with role-tagged meshes.
// Libraries must be resolved via ResolveLibraries before calling EvalDebug.
// overrides maps var names to raw values (JSON numbers/strings) for slider parameters.
func EvalDebug(ctx context.Context, prog loader.Program, currentKey string, overrides map[string]interface{}, entryPoint string) (*DebugResult, error) {
	if entryPoint == "" {
		return nil, fmt.Errorf("entry point not set")
	}
	e := newEvaluator(ctx, prog, currentKey, overrides, entryPoint)
	e.debug = true
	result, err := e.run()
	if err != nil {
		return nil, err
	}
	return &DebugResult{Solids: result.Solids, Steps: e.steps, Files: e.libSources}, nil
}

// ResolveMeshes lazily extracts meshes for the given step index, caching
// them on the DebugStep so subsequent calls are free.
func (r *DebugResult) ResolveMeshes(i int) []DebugMesh {
	if i < 0 || i >= len(r.Steps) {
		return nil
	}
	step := &r.Steps[i]
	if len(step.Meshes) == 0 && len(step.entries) > 0 {
		for _, e := range step.entries {
			step.Meshes = append(step.Meshes, DebugMesh{Role: e.Role, Mesh: e.Shape.ToDisplayMesh()})
		}
	}
	return step.Meshes
}

// opFuncKey identifies an operator function by operator symbol and operand types.
type opFuncKey struct {
	op        string
	leftType  string // type name of left operand (or sole operand for unary)
	rightType string // "" for unary operators
}

type evaluator struct {
	ctx          context.Context
	prog         loader.Program
	currentKey   string                  // which source file we're executing ("::main" or lib path)
	globals      map[string]value
	entryPoint   string                 // entry function name (default "Main")
	debug        bool
	steps        []DebugStep
	libEvalCache map[string]map[string]value  // import path → evaluated globals
	libLoadStack map[string]bool         // libraries currently being loaded (cycle detection)
	file         string                  // current file disk path
	yieldTarget  *[]value               // non-nil when inside a for-yield body
	foldAcc      *value                 // non-nil when inside a fold body; yield writes here
	libSources   map[string]string      // collected library sources (debug only)
	stdFuncs     []*parser.Function            // stdlib free functions
	stdMethods   map[string][]*parser.Function // receiverType → method functions
	structDecls  map[string]*parser.StructDecl // user-defined struct declarations
	opFuncs      map[opFuncKey]*parser.Function // operator function dispatch table
	currentLib   *libRef                // non-nil when evaluating inside a library
	overrides    map[string]value       // slider overrides: varName → value
	solidTracks  *[]SolidTrack         // shared across parent/child evaluators
	callDepth    int                   // current function call nesting depth
}

// libRef identifies a loaded library by its import path.
// Globals are stored in evaluator.libEvalCache; the program is shared via evaluator.prog.
type libRef struct {
	path string // import path of the library
}

// trackSolid records a source position and the Solid produced there.
func (e *evaluator) trackSolid(pos parser.Pos, s *manifold.Solid) {
	*e.solidTracks = append(*e.solidTracks, SolidTrack{
		File: e.file, Line: pos.Line, Col: pos.Col, Solid: s,
	})
}

// trackIfSolid tracks the value at pos if it is (or wraps) a Solid.
func (e *evaluator) trackIfSolid(pos parser.Pos, v value) {
	if s, ok := unwrap(v).(*manifold.Solid); ok {
		e.trackSolid(pos, s)
	}
}

func (e *evaluator) recordStep(op string, pos parser.Pos, entries ...debugEntry) {
	if !e.debug {
		return
	}
	step := DebugStep{Op: op, Line: pos.Line, Col: pos.Col, File: e.file}
	step.entries = append(step.entries, entries...)
	e.steps = append(e.steps, step)
}


