package evaluator

import (
	"facet/app/pkg/fctlang/loader"
	"facet/app/pkg/fctlang/parser"
	"strings"

	"facet/app/pkg/manifold"
)

// stripNamedArgs extracts bare values from a []value that may contain namedArgVal wrappers.
// Used before passing args to internal _-prefixed methods that expect positional values.
func stripNamedArgs(args []value) []value {
	for i, a := range args {
		if na, ok := a.(*namedArgVal); ok {
			args[i] = na.val
		}
	}
	return args
}

// ---------------------------------------------------------------------------
// Method call dispatch helpers
// ---------------------------------------------------------------------------

// findMethods returns functions from funcs matching name and arg count.
// receiverType filters by ReceiverType; "*" matches any.
func findMethods(funcs []*parser.Function, name, receiverType string, nArgs int) []*parser.Function {
	var candidates []*parser.Function
	for _, fn := range funcs {
		if fn.Name == name && fn.ArgsInRange(nArgs) {
			if receiverType == "*" || fn.ReceiverType == receiverType {
				candidates = append(candidates, fn)
			}
		}
	}
	return candidates
}

// resolveOverload performs type-based overload resolution on candidates.
// resolver is the evaluator used for fillDefaults/coerceArgs.
// call is invoked with the matched function and resolved args as a map.
// Returns (nil, nil, false) if candidates is empty.
func (e *evaluator) resolveOverload(
	pos parser.Pos,
	method string,
	candidates []*parser.Function,
	args []value,
	resolver *evaluator,
	call func(fn *parser.Function, args map[string]value) (value, error),
) (value, error, bool) {
	if len(candidates) == 0 {
		return nil, nil, false
	}

	for _, fn := range candidates {
		// Build named arg map from namedArgVal wrappers.
		paramNames := make(map[string]bool, len(fn.Params))
		for _, p := range fn.Params {
			paramNames[p.Name] = true
		}
		argMap := make(map[string]value, len(fn.Params))
		buildErr := false
		for _, a := range args {
			na, ok := a.(*namedArgVal)
			if !ok {
				// Non-named arg (only from _-prefixed internal calls) — positional fallback.
				buildErr = true
				break
			}
			if !paramNames[na.name] {
				buildErr = true
				break
			}
			if _, dup := argMap[na.name]; dup {
				buildErr = true
				break
			}
			argMap[na.name] = na.val
		}

		if buildErr {
			if len(candidates) > 1 {
				continue
			}
			// Single candidate — produce a specific error for bad names.
			for _, a := range args {
				na, ok := a.(*namedArgVal)
				if !ok {
					break
				}
				if !paramNames[na.name] {
					return nil, e.errAt(pos, "%s() has no parameter named %q", method, na.name), true
				}
			}
			// Non-named args to an internal function — positional fallback: build map by index.
			argMap = make(map[string]value, len(fn.Params))
			for i, a := range args {
				if i < len(fn.Params) {
					if na, ok := a.(*namedArgVal); ok {
						argMap[fn.Params[i].Name] = na.val
					} else {
						argMap[fn.Params[i].Name] = a
					}
				}
			}
		}

		if fillErr := resolver.fillDefaults(fn, argMap, resolver.globals); fillErr != nil {
			if len(candidates) > 1 {
				continue
			}
			return nil, e.wrapErr(pos, fillErr), true
		}
		if coerceErr := resolver.coerceArgs(method, fn.Params, argMap, resolver.globals); coerceErr != nil {
			if len(candidates) > 1 {
				continue
			}
			return nil, e.wrapErr(pos, coerceErr), true
		}
		result, err := call(fn, argMap)
		if err != nil {
			return nil, e.wrapErr(pos, err), true
		}
		return result, nil, true
	}
	argTypeNames := make([]string, len(args))
	for i, a := range args {
		if na, ok := a.(*namedArgVal); ok {
			argTypeNames[i] = na.name + ": " + typeName(na.val)
		} else {
			argTypeNames[i] = typeName(a)
		}
	}
	return nil, e.errAt(pos, "no matching overload for %s(%s)",
		method, strings.Join(argTypeNames, ", ")), true
}

// newLibEval creates a sub-evaluator scoped to a library's program and globals.
func (e *evaluator) newLibEval(lib *libRef) *evaluator {
	diskPath := e.prog.Resolve(lib.path)
	return &evaluator{
		ctx:          e.ctx,
		prog:         e.prog,
		currentKey:   diskPath,
		globals:      e.libEvalCache[lib.path],
		debug:        e.debug,
		libEvalCache: e.libEvalCache,
		file:         lib.path,
		libSources:   e.libSources,
		stdFuncs:     e.stdFuncs,
		stdMethods:   e.stdMethods,
		structDecls:  buildStructDecls(e.prog, diskPath),
		currentLib:   lib,
	}
}

// ---------------------------------------------------------------------------
// Method call dispatch
// ---------------------------------------------------------------------------

func (e *evaluator) evalMethodCall(mc *parser.MethodCallExpr, locals map[string]value) (value, error) {
	if err := e.ctx.Err(); err != nil {
		return nil, err
	}

	receiver, err := e.evalExpr(mc.Receiver, locals)
	if err != nil {
		return nil, err
	}

	args := make([]value, len(mc.Args))
	for i, argExpr := range mc.Args {
		v, err := e.evalExpr(argExpr, locals)
		if err != nil {
			return nil, err
		}
		args[i] = v
	}

	// stdlibCall returns a callback that evaluates fn as a stdlib method on receiver.
	stdlibCall := func(receiver value) func(fn *parser.Function, args map[string]value) (value, error) {
		return func(fn *parser.Function, args map[string]value) (value, error) {
			savedFile := e.file
			e.file = loader.StdlibPath
			defer func() { e.file = savedFile }()
			return e.evalMethodFunction(fn, receiver, args)
		}
	}

	switch r := receiver.(type) {
	case *manifold.SolidFuture:
		if !strings.HasPrefix(mc.Method, "_") {
			candidates := findMethods(e.stdMethods["Solid"], mc.Method, "*", len(args))
			if result, err, ok := e.resolveOverload(mc.Pos, mc.Method, candidates, args, e, stdlibCall(r)); ok {
				if err != nil {
					return nil, err
				}
				if rs, ok := result.(*manifold.SolidFuture); ok {
					e.trackSolid(mc.Pos, rs)
					e.recordStep(mc.Method, mc.Pos, debugRole{"input", r}, debugRole{"result", rs})
				}
				return result, nil
			}
		}
		result, err := solidMethod(e, r, mc.Method, stripNamedArgs(args))
		if err != nil {
			return nil, e.wrapErr(mc.Pos, err)
		}
		if rs, ok := result.(*manifold.SolidFuture); ok {
			e.trackSolid(mc.Pos, rs)
		}
		return result, nil

	case *manifold.SketchFuture:
		if !strings.HasPrefix(mc.Method, "_") {
			candidates := findMethods(e.stdMethods["Sketch"], mc.Method, "*", len(args))
			if result, err, ok := e.resolveOverload(mc.Pos, mc.Method, candidates, args, e, stdlibCall(r)); ok {
				if err != nil {
					return nil, err
				}
				if rs, ok := result.(*manifold.SolidFuture); ok {
					e.trackSolid(mc.Pos, rs)
					e.recordStep(mc.Method, mc.Pos, debugRole{"input", r}, debugRole{"result", rs})
				} else if rsk, ok := result.(*manifold.SketchFuture); ok {
					e.recordStep(mc.Method, mc.Pos, debugRole{"input", r}, debugRole{"result", rsk})
				}
				return result, nil
			}
		}
		result, err := sketchMethod(r, mc.Method, stripNamedArgs(args))
		if err != nil {
			return nil, e.wrapErr(mc.Pos, err)
		}
		return result, nil

	case *libRef:
		candidates := findMethods(e.prog.Sources[e.prog.Resolve(r.path)].Functions, mc.Method, "", len(args))
		if len(candidates) == 0 {
			return nil, e.errAt(mc.Pos, "library has no function %q", mc.Method)
		}
		libEval := e.newLibEval(r)
		result, err, _ := e.resolveOverload(mc.Pos, mc.Method, candidates, args, libEval,
			func(fn *parser.Function, args map[string]value) (value, error) {
				return libEval.evalFunction(fn, args)
			})
		if err != nil {
			return nil, err
		}
		e.steps = append(e.steps, libEval.steps...)
		if rs, ok := result.(*manifold.SolidFuture); ok {
			e.trackSolid(mc.Pos, rs)
		}
		return result, nil

	case *structVal:
		// Builtin struct methods (e.g. Mesh._face_normals)
		if strings.HasPrefix(mc.Method, "_") {
			if result, err := structBuiltinMethod(r, mc.Method, stripNamedArgs(args)); err == nil {
				return result, nil
			} else if !strings.HasPrefix(err.Error(), "no builtin method") {
				return nil, e.wrapErr(mc.Pos, err)
			}
		}

		// User-defined methods for this struct type
		candidates := findMethods(e.prog.Sources[e.currentKey].Functions, mc.Method, r.typeName, len(args))
		if result, err, ok := e.resolveOverload(mc.Pos, mc.Method, candidates, args, e,
			func(fn *parser.Function, args map[string]value) (value, error) {
				return e.evalMethodFunction(fn, r, args)
			}); ok {
			if rs, ok := result.(*manifold.SolidFuture); ok {
				e.trackSolid(mc.Pos, rs)
			}
			return result, err
		}

		// Library methods (if struct came from a library)
		if r.lib != nil {
			libCandidates := findMethods(e.prog.Sources[e.prog.Resolve(r.lib.path)].Functions, mc.Method, r.typeName, len(args))
			if len(libCandidates) > 0 {
				libEval := e.newLibEval(r.lib)
				result, err, _ := e.resolveOverload(mc.Pos, mc.Method, libCandidates, args, libEval,
					func(fn *parser.Function, args map[string]value) (value, error) {
						return libEval.evalMethodFunction(fn, r, args)
					})
				if err != nil {
					return nil, err
				}
				e.steps = append(e.steps, libEval.steps...)
				if rs, ok := result.(*manifold.SolidFuture); ok {
					e.trackSolid(mc.Pos, rs)
				}
				return result, nil
			}
		}

		// Stdlib methods for this struct type (e.g. Box methods)
		stdCandidates := findMethods(e.stdMethods[r.typeName], mc.Method, "*", len(args))
		if result, err, ok := e.resolveOverload(mc.Pos, mc.Method, stdCandidates, args, e, stdlibCall(r)); ok {
			if rs, ok := result.(*manifold.SolidFuture); ok {
				e.trackSolid(mc.Pos, rs)
			}
			return result, err
		}

		return nil, e.errAt(mc.Pos, "struct %s has no method %q", r.typeName, mc.Method)

	case string:
		if !strings.HasPrefix(mc.Method, "_") {
			candidates := findMethods(e.stdMethods["String"], mc.Method, "*", len(args))
			if result, err, ok := e.resolveOverload(mc.Pos, mc.Method, candidates, args, e, stdlibCall(r)); ok {
				return result, err
			}
		}
		result, err := stringMethod(r, mc.Method, stripNamedArgs(args))
		if err != nil {
			return nil, e.wrapErr(mc.Pos, err)
		}
		return result, nil

	default:
		return nil, e.errAt(mc.Pos, "cannot call method %s on %s", mc.Method, typeName(receiver))
	}
}
