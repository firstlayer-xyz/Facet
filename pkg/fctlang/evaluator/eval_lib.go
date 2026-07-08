package evaluator

import (
	"facet/pkg/fctlang/parser"
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
		opFuncs:      buildOpFuncs(e.prog, diskPath),
		solidTracks:  e.solidTracks,
	}
	libEval.globals = make(map[string]value)
	// Pre-create libRef so struct values created during globals eval get lib context
	lv := &libRef{path: key}
	libEval.currentLib = lv
	// Seed stdlib globals (PI, TAU, E, etc.) so library code can reference them.
	// Const stdlib globals stay const-wrapped inside the library body too, so a
	// library `var PI = 3` errors as a const reassignment just like in main.
	if stdSrc := e.prog.Std(); stdSrc != nil {
		for _, g := range stdSrc.Globals() {
			v, err := libEval.evalExpr(g.Value, libEval.globals)
			if err != nil {
				return nil, e.errAt(ex.Pos, "stdlib: %v", err)
			}
			if g.IsConst {
				v = &constVal{inner: v}
			}
			libEval.globals[g.Name] = v
		}
	}
	// Bind library globals through the same path as main-file globals, so a
	// computed constrained global (`var x = f() where [...]`) is validated at
	// load instead of silently accepted, and const/struct-decl handling matches.
	for _, g := range libSrc.Globals() {
		if err := libEval.bindGlobal(g, libEval.globals); err != nil {
			return nil, e.errAt(ex.Pos, "library %q: %v", ex.Path, err)
		}
	}

	e.libEvalCache[key] = libEval.globals

	return lv, nil
}
