package evaluator

import (
	"facet/app/pkg/fctlang/parser"
)

// ---------------------------------------------------------------------------
// Library evaluation
// ---------------------------------------------------------------------------

func (e *evaluator) evalLibExpr(ex *parser.LibExpr) (value, error) {
	// Fast path: return cached libRef if globals already evaluated
	if _, ok := e.libEvalCache[ex.Path]; ok {
		return &libRef{path: ex.Path}, nil
	}

	// Detect circular dependencies
	if e.libLoadStack[ex.Path] {
		return nil, e.errAt(ex.Pos, "circular library dependency: %q", ex.Path)
	}
	e.libLoadStack[ex.Path] = true
	defer delete(e.libLoadStack, ex.Path)

	// Library must have been resolved by ResolveLibraries before evaluation.
	libSrc := e.prog.Sources[e.prog.Resolve(ex.Path)]
	if libSrc == nil {
		return nil, e.errAt(ex.Pos, "library %q not resolved", ex.Path)
	}

	// Populate source for debug file tracking
	if e.libSources != nil && libSrc.Text != "" {
		e.libSources[ex.Path] = libSrc.Text
	}

	// Create a sub-evaluator to evaluate the library's globals.
	diskPath := e.prog.Resolve(ex.Path)
	libEval := &evaluator{
		ctx:          e.ctx,
		prog:         e.prog,
		currentKey:   diskPath,
		debug:        e.debug,
		libEvalCache: e.libEvalCache,
		libLoadStack: e.libLoadStack,
		libSources:   e.libSources,
		stdFuncs:     e.stdFuncs,
		stdMethods:   e.stdMethods,
		structDecls:  buildStructDecls(e.prog, diskPath),
	}
	libEval.globals = make(map[string]value)
	// Pre-create libRef so struct values created during globals eval get lib context
	lv := &libRef{path: ex.Path}
	libEval.currentLib = lv
	// Seed stdlib globals (PI, TAU, E, etc.) so library code can reference them
	if stdSrc := e.prog.Std(); stdSrc != nil {
		for _, g := range stdSrc.Globals {
			v, err := libEval.evalExpr(g.Value, libEval.globals)
			if err != nil {
				return nil, e.errAt(ex.Pos, "stdlib: %v", err)
			}
			libEval.globals[g.Name] = v
		}
	}
	for _, g := range libSrc.Globals {
		v, err := libEval.evalExpr(g.Value, libEval.globals)
		if err != nil {
			return nil, e.errAt(ex.Pos, "library %q: %v", ex.Path, err)
		}
		libEval.globals[g.Name] = v
		// Register imported library struct declarations with qualified names
		if ilv, ok := v.(*libRef); ok {
			if ilvSrc := e.prog.Sources[e.prog.Resolve(ilv.path)]; ilvSrc != nil {
				for _, sd := range ilvSrc.StructDecls {
					libEval.structDecls[g.Name+"."+sd.Name] = sd
				}
			}
		}
	}

	e.libEvalCache[ex.Path] = libEval.globals

	return lv, nil
}
