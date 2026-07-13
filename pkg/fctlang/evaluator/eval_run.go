package evaluator

import (
	"facet/pkg/fctlang/parser"
	"fmt"
	"math"
	"sort"

	"facet/pkg/manifold"
)

func (e *evaluator) run() (*EvalResult, error) {
	// Load stdlib functions and methods
	e.stdFuncs = nil
	e.stdMethods = make(map[string][]*parser.Function)
	if stdSrc := e.prog.Std(); stdSrc != nil {
		for _, fn := range stdSrc.Functions() {
			if fn.ReceiverType != "" {
				e.stdMethods[fn.ReceiverType] = append(e.stdMethods[fn.ReceiverType], fn)
			} else {
				e.stdFuncs = append(e.stdFuncs, fn)
			}
		}
		// Make stdlib source available for error navigation and debug tabs
		e.libSources[stdSrc.Path] = stdSrc.Text
	}

	// Populate struct declarations (stdlib + user program)
	e.structDecls = buildStructDecls(e.prog, e.currentKey)

	// Build operator function dispatch table
	e.opFuncs = buildOpFuncs(e.prog, e.currentKey)

	e.globals = make(map[string]value)
	e.stdGlobals = make(map[string]value)

	// Evaluate stdlib globals (PI, TAU, E, etc.). They are kept in a separate
	// hermetic map as well: a user `var PI = 3` shadows PI for USER code, but
	// stdlib bodies must keep seeing the stdlib's own value.
	if stdSrc := e.prog.Std(); stdSrc != nil {
		for _, g := range stdSrc.Globals() {
			v, err := e.evalExpr(g.Value, e.stdGlobals)
			if err != nil {
				return nil, fmt.Errorf("stdlib: %v", err)
			}
			var wrapped value = v
			if g.IsConst {
				wrapped = &constVal{inner: v}
			}
			e.stdGlobals[g.Name] = wrapped
			e.globals[g.Name] = wrapped
		}
	}

	currentSrc := e.prog.Sources[e.currentKey]
	if currentSrc == nil {
		// The entry key names no loaded source — e.g. a view-only library tab was
		// made the eval target but its source is resolved from its backing, not
		// passed as a root. Fail loudly instead of dereferencing a nil *Source.
		return nil, fmt.Errorf("entry point source %q is not loaded", e.currentKey)
	}
	for _, g := range currentSrc.Globals() {
		// A slider override replaces the initializer with the user's value; it is
		// still constraint-checked and committed, just not re-evaluated.
		if ov, ok := e.overrides[g.Name]; ok && g.Constraint != nil {
			if err := e.commitGlobal(g, ov, e.globals); err != nil {
				return nil, err
			}
		} else if err := e.bindGlobal(g, e.globals); err != nil {
			return nil, err
		}
	}

	// Find entry point function
	var entryFn *parser.Function
	for _, fn := range currentSrc.Functions() {
		if fn.Name == e.entryPoint {
			entryFn = fn
			break
		}
	}
	if entryFn == nil {
		return nil, &parser.SourceError{Line: 1, Col: 1, Message: fmt.Sprintf("no %s() function found", e.entryPoint)}
	}

	// Return-type validation is a static constraint enforced in the checker
	// (see checker.Result.ValidateEntryPoint). The runtime switch on the
	// actual returned value below is the remaining defense: it rejects
	// values that the checker could not pin down (e.g. unannotated entry
	// points whose inferred type was typeUnknown).

	// Build argument map for entry point: use overrides for params with constraints, else defaults.
	args := make(map[string]value)
	for _, p := range entryFn.Params {
		if ov, ok := e.overrides[p.Name]; ok {
			// Slider overrides arrive as JSON-typed primitives (float64 for
			// numbers, string, etc.). Coerce them to the param's declared
			// type so a Number → Length param doesn't smuggle a bare float
			// into the body where the checker expects a length.
			args[p.Name] = e.coerceToType(p.Type, ov, e.globals)
		} else if p.Default != nil {
			v, err := e.evalExpr(p.Default, e.globals)
			if err != nil {
				return nil, err
			}
			args[p.Name] = v
		} else if parser.IsOptionalType(p.Type) {
			args[p.Name] = none("")
		} else {
			return nil, e.errAt(entryFn.Pos, "%s() parameter %q has no default and no override", e.entryPoint, p.Name)
		}
	}

	result, err := e.evalFunction(entryFn, args)
	if err != nil {
		return nil, err
	}

	var solids []*manifold.Solid
	switch r := result.(type) {
	case *manifold.Solid:
		solids = []*manifold.Solid{r}
	case *structVal:
		if r.typeName == "Animation" {
			frameVal := r.fields["frame"]
			frameFn, ok := frameVal.(*functionVal)
			if !ok {
				return nil, e.errAt(entryFn.Pos, "Animation.frame must be a function, got %s", typeName(frameVal))
			}
			if len(frameFn.params) != 1 {
				return nil, e.errAt(entryFn.Pos, "Animation.frame must take exactly one parameter (time in ms), got %d", len(frameFn.params))
			}
			// The handle is retained and its frame closure runs per displayed
			// frame, long after this build's context is done. Each Frame call
			// installs its own context (see frameLocked), so the build context is
			// never consulted at frame time — no detach is needed here.
			return &EvalResult{
				Animation: &Animation{e: e, frame: frameFn, argName: frameFn.params[0].Name, baseTracks: len(*e.solidTracks)},
			}, nil
		}
		if r.typeName != "PolyMesh" {
			return nil, e.errAt(entryFn.Pos, "%s() must return a Solid, PolyMesh, or Array of Solids, got %s", e.entryPoint, r.typeName)
		}
		pm, err := structValToPolyMesh(r)
		if err != nil {
			return nil, err
		}
		s, err := pm.ToSolid()
		if err != nil {
			return nil, err
		}
		solids = []*manifold.Solid{s}
	case array:
		var err error
		solids, err = extractSolids(e.entryPoint, r)
		if err != nil {
			return nil, err
		}
	default:
		return nil, e.errAt(entryFn.Pos, "%s() must return a Solid, PolyMesh, or Array of Solids, got %s", e.entryPoint, typeName(result))
	}

	// Compute stats from solids
	var stats ModelStats
	for _, s := range solids {
		stats.Volume += s.Volume()
		stats.SurfaceArea += s.SurfaceArea()
	}

	return &EvalResult{Solids: solids, Stats: stats, PosMap: buildPosMap(*e.solidTracks)}, nil
}

// bindGlobal evaluates global g's initializer in scope, applies value semantics
// (copy + reidentify of a reused scoped solid, like bindVar for locals), and
// commits it via commitGlobal. Shared by run()'s top-level loop and evalLibExpr,
// so a library global is bound — and constraint-checked — exactly like a
// main-file one.
func (e *evaluator) bindGlobal(g *parser.VarStmt, scope map[string]value) error {
	v, err := e.evalExpr(g.Value, scope)
	if err != nil {
		return err
	}
	return e.commitGlobal(g, reidentifyBinding(copyValue(v), scope), scope)
}

// commitGlobal stores value v as global g in scope: const/constraint wrapping,
// runtime constraint validation, solid tracking for face-click navigation, and
// library struct-decl registration. Split from bindGlobal so the slider-override
// path can commit a user-supplied value without re-evaluating the initializer.
func (e *evaluator) commitGlobal(g *parser.VarStmt, v value, scope map[string]value) error {
	if g.IsConst {
		scope[g.Name] = &constVal{inner: v}
	} else {
		scope[g.Name] = v
	}
	if g.Constraint != nil {
		if err := e.validateConstraint(g.Name, g.Constraint, unwrap(scope[g.Name]), scope); err != nil {
			return err
		}
		// Wrap with constrainedVal so reassignment re-validates (constVal on the
		// outside when the global is also const).
		bare := unwrap(scope[g.Name])
		if g.IsConst {
			scope[g.Name] = &constVal{inner: &constrainedVal{inner: bare, constraint: g.Constraint, name: g.Name}}
		} else {
			scope[g.Name] = &constrainedVal{inner: bare, constraint: g.Constraint, name: g.Name}
		}
	}
	// Give a solid binding its posMap entry (unwraps const/constraint first).
	e.trackIfSolid(g.Pos, scope[g.Name])
	// Register imported library struct declarations under qualified names
	// (e.g. "T.Config") so namespace collisions are avoided.
	if lv, ok := unwrap(scope[g.Name]).(*libRef); ok {
		if lvSrc := e.prog.Sources[e.prog.Resolve(lv.path)]; lvSrc != nil {
			for _, sd := range lvSrc.StructDecls() {
				e.structDecls[g.Name+"."+sd.Name] = sd
			}
		}
	}
	return nil
}

// buildPosMap resolves solidTracks into a source-position → face-ID index for
// face-click navigation: every solid produced at a source position contributes
// its FaceMap's IDs to that position's entry. Shared by the one-shot Eval path
// and the per-frame Animation path so face-click works in both.
func buildPosMap(tracks []SolidTrack) []PosEntry {
	type posKey struct {
		file      string
		line, col int
	}
	posToIDs := make(map[posKey]map[uint32]bool)
	for _, track := range tracks {
		if len(track.FaceIDs) == 0 {
			continue
		}
		key := posKey{track.File, track.Line, track.Col}
		if posToIDs[key] == nil {
			posToIDs[key] = make(map[uint32]bool)
		}
		for _, id := range track.FaceIDs {
			posToIDs[key][id] = true
		}
	}
	var posMap []PosEntry
	for key, ids := range posToIDs {
		entry := PosEntry{File: key.file, Line: key.line, Col: key.col}
		for id := range ids {
			entry.FaceIDs = append(entry.FaceIDs, id)
		}
		posMap = append(posMap, entry)
	}
	// Order most-specific-first: entries covering fewer face IDs come before
	// broader ones. A face-click resolves to the first source entry that owns
	// the clicked face, so the narrowest binding wins over whole-model entries
	// (the entry point's return, an outer `a + b` union), which cover every
	// face. Without this, clicking any face lands on the whole-model entry and
	// highlights everything — defeating per-binding identity. Line/col break
	// ties so the ordering stays deterministic.
	sort.Slice(posMap, func(i, j int) bool {
		if len(posMap[i].FaceIDs) != len(posMap[j].FaceIDs) {
			return len(posMap[i].FaceIDs) < len(posMap[j].FaceIDs)
		}
		if posMap[i].File != posMap[j].File {
			return posMap[i].File < posMap[j].File
		}
		if posMap[i].Line != posMap[j].Line {
			return posMap[i].Line < posMap[j].Line
		}
		return posMap[i].Col < posMap[j].Col
	})
	return posMap
}

// extractSolids validates that all elements of an array are Solids and returns them.
func extractSolids(entryPoint string, arr array) ([]*manifold.Solid, error) {
	if len(arr.elems) == 0 {
		return nil, fmt.Errorf("%s() returned an empty array; expected at least one Solid", entryPoint)
	}
	solids := make([]*manifold.Solid, len(arr.elems))
	for i, elem := range arr.elems {
		s, ok := elem.(*manifold.Solid)
		if !ok {
			return nil, fmt.Errorf("%s() array element %d is %s, expected Solid", entryPoint, i, typeName(elem))
		}
		solids[i] = s
	}
	return solids, nil
}

// arrangeLayout packs solids on the X/Y plane using a shelf bin-packing
// algorithm. Items are sorted by footprint area (descending) and greedily
// placed left-to-right onto shelves; a new shelf starts when the next item
// would exceed the target row width. The target width is chosen so the
// overall layout is roughly square. All solids are translated so Z=0 rests
// on the build plate. The returned slice preserves the input order.
// gap < 0 means auto (10% of the largest footprint dimension).
func arrangeLayout(solids []*manifold.Solid, gapOpt *float64) []*manifold.Solid {
	n := len(solids)
	if n == 0 {
		return solids
	}

	type item struct {
		idx                                int
		s                                  *manifold.Solid
		minX, minY, minZ, maxX, maxY, maxZ float64
		w, d                               float64
	}

	items := make([]item, n)
	var totalArea, maxSide float64
	for i, s := range solids {
		minX, minY, minZ, maxX, maxY, maxZ := s.BoundingBox()
		w := maxX - minX
		d := maxY - minY
		items[i] = item{i, s, minX, minY, minZ, maxX, maxY, maxZ, w, d}
		totalArea += w * d
		if w > maxSide {
			maxSide = w
		}
		if d > maxSide {
			maxSide = d
		}
	}

	// nil gap = automatic spacing (10% of the largest footprint dimension).
	gap := maxSide * 0.1
	if gapOpt != nil {
		gap = *gapOpt
	}

	// Sort by footprint area descending (largest first).
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].w*items[i].d > items[j].w*items[j].d
	})

	// Target shelf width: roughly square overall layout, but never narrower
	// than the widest item.
	targetWidth := math.Sqrt(totalArea)
	for _, it := range items {
		if it.w > targetWidth {
			targetWidth = it.w
		}
	}

	result := make([]*manifold.Solid, n)
	var shelfX, shelfY, shelfH float64
	for _, it := range items {
		if shelfX > 0 && shelfX+it.w > targetWidth {
			// Start a new shelf.
			shelfY += shelfH + gap
			shelfX = 0
			shelfH = 0
		}
		dx := shelfX - it.minX
		dy := shelfY - it.minY
		dz := -it.minZ
		result[it.idx] = it.s.Translate(dx, dy, dz)
		shelfX += it.w + gap
		if it.d > shelfH {
			shelfH = it.d
		}
	}
	return result
}
