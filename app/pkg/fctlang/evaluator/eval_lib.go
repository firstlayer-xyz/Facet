package evaluator

import (
	"facet/app/pkg/fctlang/parser"
)

// ---------------------------------------------------------------------------
// Library evaluation
// ---------------------------------------------------------------------------

func (e *evaluator) evalLibExpr(ex *parser.LibExpr) (value, error) {
	// Use the canonical key (ex.Resolved || ex.Path) for all lookups. The
	// raw ex.Path is kept for user-facing error messages. Two distinct
	// sources each doing `lib "./knurling"` get different keys (different
	// absolute disk paths) so they don't collide in the caches below.
	key := ex.Key()

	// Fast path: return cached libRef if globals already evaluated
	if _, ok := e.libEvalCache[key]; ok {
		return &libRef{path: key}, nil
	}

	// Detect circular dependencies
	if e.libLoadStack[key] {
		return nil, e.errAt(ex.Pos, "circular library dependency: %q", ex.Path)
	}
	e.libLoadStack[key] = true
	defer delete(e.libLoadStack, key)

	// Library must have been resolved by ResolveLibraries before evaluation.
	diskPath := e.prog.Resolve(key)
	libSrc := e.prog.Sources[diskPath]
	if libSrc == nil {
		return nil, e.errAt(ex.Pos, "library %q not resolved", ex.Path)
	}

	// Populate source for debug file tracking
	if e.libSources != nil && libSrc.Text != "" {
		e.libSources[key] = libSrc.Text
	}

	// Create a sub-evaluator to evaluate the library's globals.
	libEval := &evaluator{
		ctx:          e.ctx,
		prog:         e.prog,
		currentKey:   diskPath,
		file:         diskPath,
		debug:        e.debug,
		libEvalCache: e.libEvalCache,
		libLoadStack: e.libLoadStack,
		libSources:   e.libSources,
		stdFuncs:     e.stdFuncs,
		stdMethods:   e.stdMethods,
		structDecls:  buildStructDecls(e.prog, diskPath),
		solidTracks:  e.solidTracks,
	}
	libEval.globals = make(map[string]value)
	// Pre-create libRef so struct values created during globals eval get lib context
	lv := &libRef{path: key}
	libEval.currentLib = lv
	// Seed stdlib globals (PI, TAU, E, etc.) so library code can reference them
	if stdSrc := e.prog.Std(); stdSrc != nil {
		for _, g := range stdSrc.Globals() {
			v, err := libEval.evalExpr(g.Value, libEval.globals)
			if err != nil {
				return nil, e.errAt(ex.Pos, "stdlib: %v", err)
			}
			libEval.globals[g.Name] = v
		}
	}
	for _, g := range libSrc.Globals() {
		v, err := libEval.evalExpr(g.Value, libEval.globals)
		if err != nil {
			return nil, e.errAt(ex.Pos, "library %q: %v", ex.Path, err)
		}
		libEval.globals[g.Name] = v
		// Register imported library struct declarations with qualified names
		if ilv, ok := v.(*libRef); ok {
			if ilvSrc := e.prog.Sources[e.prog.Resolve(ilv.path)]; ilvSrc != nil {
				for _, sd := range ilvSrc.StructDecls() {
					libEval.structDecls[g.Name+"."+sd.Name] = sd
				}
			}
		}
	}

	e.libEvalCache[key] = libEval.globals

	return lv, nil
}
