package evaluator

import (
	"context"
	"facet/pkg/fctlang/loader"
	"facet/pkg/fctlang/parser"
	"fmt"

	"facet/pkg/manifold"
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
	constraint parser.Expr // AST constraint expression for re-validation
	name       string      // binding name, for error messages
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
// Empty string means untyped/heterogeneous. "Any" means explicitly dynamic.
type array struct {
	elems    []value
	elemType string
}

// structVal represents a runtime user-defined struct instance.
type structVal struct {
	typeName string
	fields   map[string]value
	decl     *parser.StructDecl // declaration from the scope where this struct was defined
	lib      *libRef            // non-nil if struct was created in a library context
}

// optionalVal represents the runtime form of an Optional. When `present`
// is false, `inner` is nil — this is the None case. innerType records the
// expected inner type (used for typeName() reporting and downstream type
// checks at borrow sites).
type optionalVal struct {
	present   bool
	inner     value
	innerType string // "Number", "Length", ..., or "" for bare nil
}

// some wraps a definite value as an Optional with that value present.
func some(v value, innerType string) *optionalVal {
	return &optionalVal{present: true, inner: v, innerType: innerType}
}

// none returns the None variant for a given inner type. innerType may be ""
// for a bare nil literal; downstream code that needs to know the inner type
// must thread it through from the binding context.
func none(innerType string) *optionalVal {
	return &optionalVal{present: false, innerType: innerType}
}

// functionVal represents a first-class function (lambda) value.
// captured holds a snapshot of the local scope at the time the lambda was created.
// globals is the DEFINING context's global scope — calls resolve free names
// against it rather than against whoever happens to invoke the lambda (a
// library or stdlib body calling a user lambda must not substitute its own
// globals for the user's).
type functionVal struct {
	params   []*parser.Param
	retType  string
	body     []parser.Stmt
	captured map[string]value
	globals  map[string]value
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
	Meshes  []DebugMesh // populated lazily by ResolveMeshes
	Line    int
	Col     int
	File    string       // disk path of the source file
	entries []debugEntry // unexported — holds shape ptrs until ResolveMeshes
}

// DebugResult holds the evaluated solids plus the step-by-step debug trace.
type DebugResult struct {
	Solids []*manifold.Solid
	Steps  []DebugStep
	Files  map[string]string // path → source text (for editor display)
}

// ModelStats holds computed model statistics from evaluation.
type ModelStats struct {
	Triangles   int        `json:"triangles"`
	Vertices    int        `json:"vertices"`
	Volume      float64    `json:"volume"`      // mm³
	SurfaceArea float64    `json:"surfaceArea"` // mm²
	BBoxMin     [3]float64 `json:"bboxMin"`     // [x, y, z] min corner in mm
	BBoxMax     [3]float64 `json:"bboxMax"`     // [x, y, z] max corner in mm
}

// SolidFrameStats builds the ModelStats for a single rendered solid — one
// animation frame, or any one-solid render. Triangle/vertex counts come from
// the already-extracted display mesh; volume, surface area, and the bounding
// box from the solid itself. Shared by the desktop /eval and /frame handlers
// and the wasm preview so every per-solid frame reports stats identically
// (including the bounding box, which the wasm path previously omitted).
func SolidFrameStats(solid *manifold.Solid, mesh *manifold.DisplayMesh) ModelStats {
	s := ModelStats{
		Triangles:   mesh.IndexCount / 3,
		Vertices:    mesh.VertexCount,
		Volume:      solid.Volume(),
		SurfaceArea: solid.SurfaceArea(),
	}
	s.BBoxMin, s.BBoxMax = manifold.SolidsBounds([]*manifold.Solid{solid})
	return s
}

// PosEntry maps a source position (file+line+col) to the face IDs of solids
// that were created or operated on at that position.
type PosEntry struct {
	File    string   `json:"file"` // disk path of the source file
	Line    int      `json:"line"`
	Col     int      `json:"col"`
	FaceIDs []uint32 `json:"faceIDs"`
}

// SolidTrack records a source position and the face IDs of the solid produced
// there. It snapshots the IDs (all buildPosMap ever reads) rather than the
// *manifold.Solid, so an intermediate solid isn't pinned alive by its track —
// its finalizer-driven C++ release fires as soon as the evaluator drops it.
type SolidTrack struct {
	File    string
	Line    int
	Col     int
	FaceIDs []uint32
}

// EvalResult holds the evaluated solids and model statistics.
// Callers extract meshes as needed via ToMesh().
// When the entry returns an Animation, Animation is non-nil and Solids is empty.
type EvalResult struct {
	Solids    []*manifold.Solid
	Animation *Animation // non-nil when the entry returned an Animation
	Stats     ModelStats
	PosMap    []PosEntry
}

// newEvaluator creates an evaluator with all shared fields initialized. It
// errors if an override value can't be converted to its target type.
func newEvaluator(ctx context.Context, prog loader.Program, currentKey string, overrides map[string]interface{}, entryPoint string) (*evaluator, error) {
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
		conv, err := convertOverrides(prog, currentKey, overrides, entryPoint)
		if err != nil {
			return nil, err
		}
		e.overrides = conv
	}
	return e, nil
}

// Eval evaluates a parsed facet program. entryPoint must name a function that returns a Solid.
// Libraries must be resolved via ResolveLibraries before calling Eval.
// The context can be used to cancel evaluation mid-execution.
// overrides maps var names to raw values (JSON numbers/strings) for slider parameters.
func Eval(ctx context.Context, prog loader.Program, currentKey string, overrides map[string]interface{}, entryPoint string) (*EvalResult, error) {
	if entryPoint == "" {
		return nil, fmt.Errorf("entry point not set")
	}
	e, err := newEvaluator(ctx, prog, currentKey, overrides, entryPoint)
	if err != nil {
		return nil, err
	}
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
	e, err := newEvaluator(ctx, prog, currentKey, overrides, entryPoint)
	if err != nil {
		return nil, err
	}
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
	currentKey   string // which source file we're executing ("::main" or lib path)
	globals      map[string]value
	stdGlobals   map[string]value // the stdlib's own globals (PI, …), hermetic from user redefinition
	inStdlib     bool             // true while a stdlib function/method body executes
	entryPoint   string           // entry function name (default "Main")
	debug        bool
	steps        []DebugStep
	libEvalCache map[string]map[string]value    // import path → evaluated globals
	libLoadStack map[string]bool                // libraries currently being loaded (cycle detection)
	file         string                         // current file disk path
	yieldTarget  *[]value                       // non-nil when inside a for-yield body
	foldAcc      *value                         // non-nil when inside a fold body; yield writes here
	libSources   map[string]string              // source text per file (error snippets, debug tabs); always allocated
	stdFuncs     []*parser.Function             // stdlib free functions
	stdMethods   map[string][]*parser.Function  // receiverType → method functions
	structDecls  map[string]*parser.StructDecl  // user-defined struct declarations
	opFuncs      map[opFuncKey]*parser.Function // operator function dispatch table
	currentLib   *libRef                        // non-nil when evaluating inside a library
	overrides    map[string]value               // slider overrides: varName → value
	solidTracks  *[]SolidTrack                  // shared across parent/child evaluators
	callDepth    int                            // current function call nesting depth
}

// libRef identifies a loaded library by its import path.
// Globals are stored in evaluator.libEvalCache; the program is shared via evaluator.prog.
type libRef struct {
	path string // import path of the library
}

// trackSolid records a source position and the face IDs of the solid produced
// there. Empty-FaceMap solids are skipped (buildPosMap ignores them anyway), so
// a track always carries at least one ID.
func (e *evaluator) trackSolid(pos parser.Pos, s *manifold.Solid) {
	if len(s.FaceMap) == 0 {
		return
	}
	ids := make([]uint32, 0, len(s.FaceMap))
	for id := range s.FaceMap {
		ids = append(ids, id)
	}
	*e.solidTracks = append(*e.solidTracks, SolidTrack{
		File: e.file, Line: pos.Line, Col: pos.Col, FaceIDs: ids,
	})
}

// trackIfSolid tracks the value at pos if it is (or wraps) a Solid.
func (e *evaluator) trackIfSolid(pos parser.Pos, v value) {
	if s, ok := unwrap(v).(*manifold.Solid); ok {
		e.trackSolid(pos, s)
	}
}

// reidentifyBinding gives a solid its own identity when a binding REUSES an
// existing named solid — that is, its faces are a subset of a solid already in
// scope, as in `var a = proto; var b = proto.Rotate(...); var c = proto...`.
// Transforms preserve original IDs, so those copies otherwise share proto's
// identity and can't be selected or colored apart in a combined model.
//
// A fresh construction (a primitive, or a NewX() call producing new IDs) and an
// assembly of already-distinct parts (`var part = a + b + c`) are left as-is:
// they don't alias a scoped solid, so #264's most-specific-first posMap ordering
// already resolves a click to the right sub-part. Only uniformly colored solids
// are re-originaled — collapsing a multi-color solid would flatten its per-part
// colors, so those pass through untouched. It also descends into arrays,
// structs, and Optionals, so a solid reused inside a container binding
// (`[proto, proto.Move(...)]`) gets its own identity too.
func reidentifyBinding(v value, scope map[string]value) value {
	nv, _ := reidentifyValue(v, scope)
	return nv
}

// reidentifyValue re-originals every reused scoped solid inside v, descending
// into arrays, structs, and Optionals. It returns whether anything changed and,
// when nothing did, the original value — so binding a container that holds no
// reused solid allocates nothing, matching the pass-through for non-solid
// scalars.
func reidentifyValue(v value, scope map[string]value) (value, bool) {
	switch tv := v.(type) {
	case *manifold.Solid:
		// Only single-part solids are collapsed to a fresh identity. Reidentify
		// (manifold's AsOriginal) flattens ALL of a solid's faces into one group,
		// so applying it to a multi-part solid destroys its per-part identities —
		// a click can then no longer tell its features apart. A multi-part solid
		// reused verbatim keeps its groups; giving reused multi-part copies their
		// own per-part identities is the job of the per-part reidentify path.
		if len(tv.FaceMap) <= 1 && uniformlyColored(tv) && reusesScopedSolid(tv, scope) {
			return tv.Reidentify(), true
		}
		return v, false
	case array:
		var out []value // allocated lazily on the first changed element
		for i, el := range tv.elems {
			nel, ch := reidentifyValue(el, scope)
			if ch && out == nil {
				out = make([]value, len(tv.elems))
				copy(out, tv.elems)
			}
			if out != nil {
				out[i] = nel
			}
		}
		if out == nil {
			return v, false
		}
		return array{elems: out, elemType: tv.elemType}, true
	case *structVal:
		var fields map[string]value // allocated lazily on the first changed field
		for k, fv := range tv.fields {
			nfv, ch := reidentifyValue(fv, scope)
			if ch && fields == nil {
				fields = make(map[string]value, len(tv.fields))
				for kk, vv := range tv.fields {
					fields[kk] = vv
				}
			}
			if fields != nil {
				fields[k] = nfv
			}
		}
		if fields == nil {
			return v, false
		}
		return &structVal{typeName: tv.typeName, fields: fields, decl: tv.decl, lib: tv.lib}, true
	case *optionalVal:
		if tv.present {
			if ninner, ch := reidentifyValue(tv.inner, scope); ch {
				return &optionalVal{present: true, inner: ninner, innerType: tv.innerType}, true
			}
		}
		return v, false
	default:
		return v, false
	}
}

// forEachSolid calls fn for every *manifold.Solid reachable in v, descending
// into arrays, structs, and Optionals (peeling const/constraint wrappers) so a
// solid nested in a scoped container still counts as a reuse donor.
func forEachSolid(v value, fn func(*manifold.Solid)) {
	switch tv := unwrap(v).(type) {
	case *manifold.Solid:
		fn(tv)
	case array:
		for _, el := range tv.elems {
			forEachSolid(el, fn)
		}
	case *structVal:
		for _, fv := range tv.fields {
			forEachSolid(fv, fn)
		}
	case *optionalVal:
		if tv.present {
			forEachSolid(tv.inner, fn)
		}
	}
}

// uniformlyColored reports whether every face of the solid shares one color and
// alpha (including all-uncolored). Reidentify collapses to a single face, so it
// is only lossless — safe to apply here — when the faces are uniform.
func uniformlyColored(s *manifold.Solid) bool {
	first := true
	var shared manifold.FaceInfo
	for _, fi := range s.FaceMap {
		if first {
			shared, first = fi, false
			continue
		}
		if fi != shared {
			return false
		}
	}
	return true
}

// reusesScopedSolid reports whether s's face IDs are a subset of some solid
// reachable in scope — i.e. s is a copy or transform of one named solid (an
// alias to break), not a fresh construction or an assembly spanning several of
// them (which is a superset of any single one). Donors nested inside scoped
// arrays/structs count too, so a solid reused out of a scoped collection is
// still re-originaled.
func reusesScopedSolid(s *manifold.Solid, scope map[string]value) bool {
	if len(s.FaceMap) == 0 {
		return false
	}
	reused := false
	for _, bound := range scope {
		forEachSolid(bound, func(other *manifold.Solid) {
			if reused || len(s.FaceMap) > len(other.FaceMap) {
				return
			}
			for id := range s.FaceMap {
				if _, in := other.FaceMap[id]; !in {
					return
				}
			}
			reused = true
		})
	}
	return reused
}

func (e *evaluator) recordStep(op string, pos parser.Pos, entries ...debugEntry) {
	if !e.debug {
		return
	}
	step := DebugStep{Op: op, Line: pos.Line, Col: pos.Col, File: e.file}
	step.entries = append(step.entries, entries...)
	e.steps = append(e.steps, step)
}
