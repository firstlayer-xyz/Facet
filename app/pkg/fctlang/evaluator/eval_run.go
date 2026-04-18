package evaluator

import (
	"facet/app/pkg/fctlang/parser"
	"fmt"
	"math"
	"sort"

	"facet/app/pkg/manifold"
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
		if e.libSources != nil {
			e.libSources[stdSrc.Path] = stdSrc.Text
		}
	}

	// Populate struct declarations (stdlib + user program)
	e.structDecls = buildStructDecls(e.prog, e.currentKey)

	// Build operator function dispatch table
	e.opFuncs = buildOpFuncs(e.prog, e.currentKey)

	e.globals = make(map[string]value)

	// Evaluate stdlib globals (PI, TAU, E, etc.)
	if stdSrc := e.prog.Std(); stdSrc != nil {
		for _, g := range stdSrc.Globals() {
			v, err := e.evalExpr(g.Value, e.globals)
			if err != nil {
				return nil, fmt.Errorf("stdlib: %v", err)
			}
			if g.IsConst {
				e.globals[g.Name] = &constVal{inner: v}
			} else {
				e.globals[g.Name] = v
			}
		}
	}

	currentSrc := e.prog.Sources[e.currentKey]
	for _, g := range currentSrc.Globals() {
		if ov, ok := e.overrides[g.Name]; ok && g.Constraint != nil {
			e.globals[g.Name] = ov
		} else {
			v, err := e.evalExpr(g.Value, e.globals)
			if err != nil {
				return nil, err
			}
			if g.IsConst {
				e.globals[g.Name] = &constVal{inner: v}
			} else {
				e.globals[g.Name] = v
			}
		}
		if g.Constraint != nil {
			if err := e.validateConstraint(g.Name, g.Constraint, unwrap(e.globals[g.Name]), e.globals); err != nil {
				return nil, err
			}
			// Wrap with constrainedVal so reassignment re-validates.
			inner := e.globals[g.Name]
			if g.IsConst {
				// constVal wrapping constrainedVal: &constVal{inner: &constrainedVal{...}}
				cv := unwrap(inner) // get bare value
				e.globals[g.Name] = &constVal{inner: &constrainedVal{inner: cv, constraint: g.Constraint, name: g.Name}}
			} else {
				bare := unwrap(inner)
				e.globals[g.Name] = &constrainedVal{inner: bare, constraint: g.Constraint, name: g.Name}
			}
		}
		// Register library struct declarations with qualified names
		// (e.g. "T.Config") so namespace collisions are avoided.
		if lv, ok := unwrap(e.globals[g.Name]).(*libRef); ok {
			if lvSrc := e.prog.Sources[e.prog.Resolve(lv.path)]; lvSrc != nil {
				for _, sd := range lvSrc.StructDecls() {
					e.structDecls[g.Name+"."+sd.Name] = sd
				}
			}
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
			args[p.Name] = ov
		} else if p.Default != nil {
			v, err := e.evalExpr(p.Default, e.globals)
			if err != nil {
				return nil, err
			}
			args[p.Name] = v
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


	// Build PosMap: resolve solidTracks to face IDs
	type posKey struct {
		file     string
		line, col int
	}
	posToIDs := make(map[posKey]map[uint32]bool)
	for _, track := range *e.solidTracks {
		if len(track.Solid.FaceMap) == 0 {
			continue
		}
		key := posKey{track.File, track.Line, track.Col}
		if posToIDs[key] == nil {
			posToIDs[key] = make(map[uint32]bool)
		}
		for id := range track.Solid.FaceMap {
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
	sort.Slice(posMap, func(i, j int) bool {
		if posMap[i].File != posMap[j].File {
			return posMap[i].File < posMap[j].File
		}
		if posMap[i].Line != posMap[j].Line {
			return posMap[i].Line < posMap[j].Line
		}
		return posMap[i].Col < posMap[j].Col
	})

	return &EvalResult{Solids: solids, Stats: stats, PosMap: posMap}, nil
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
func arrangeLayout(solids []*manifold.Solid, gap float64) []*manifold.Solid {
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

	if gap < 0 {
		gap = maxSide * 0.1
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
