package evaluator

import (
	"facet/app/pkg/fctlang/loader"
	"facet/app/pkg/fctlang/parser"
	"strings"

	"facet/app/pkg/manifold"
)

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
// args is a map[string]value keyed by parameter name.
// resolver is the evaluator used for fillDefaults/coerceArgs.
// call is invoked with the matched function and resolved args as a map.
// Returns (nil, nil, false) if candidates is empty.
func (e *evaluator) resolveOverload(
	pos parser.Pos,
	method string,
	candidates []*parser.Function,
	args map[string]value,
	resolver *evaluator,
	call func(fn *parser.Function, args map[string]value) (value, error),
) (value, error, bool) {
	if len(candidates) == 0 {
		return nil, nil, false
	}

	for _, fn := range candidates {
		// Validate that all provided arg names match declared parameters.
		paramNames := make(map[string]bool, len(fn.Params))
		for _, p := range fn.Params {
			paramNames[p.Name] = true
		}
		badName := false
		for name := range args {
			if !paramNames[name] {
				badName = true
				break
			}
		}
		if badName {
			if len(candidates) > 1 {
				continue
			}
			// Single candidate — produce a specific error for bad names.
			for name := range args {
				if !paramNames[name] {
					return nil, e.errAt(pos, "%s() has no parameter named %q", method, name), true
				}
			}
		}

		// Copy the args map so fillDefaults/coerceArgs don't mutate the caller's map
		// when we need to try the next candidate.
		argMap := make(map[string]value, len(args))
		for k, v := range args {
			argMap[k] = v
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
	argTypeNames := make([]string, 0, len(args))
	for name, v := range args {
		argTypeNames = append(argTypeNames, name+": "+typeName(v))
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
		file:         diskPath,
		libSources:   e.libSources,
		stdFuncs:     e.stdFuncs,
		stdMethods:   e.stdMethods,
		structDecls:  buildStructDecls(e.prog, diskPath),
		currentLib:   lib,
		solidTracks:  e.solidTracks,
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

	// Build named args map for stdlib/user method dispatch.
	// Pass nil for paramNames; bare positional args in method calls will be
	// mapped positionally when the receiver type is known (inside resolveOverload).
	argMap, namedErr := e.evalNamedArgs(mc.Args, locals, nil)
	if namedErr != nil {
		return nil, namedErr
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

	// evalPositional lazily evaluates positional args for builtin methods.
	evalPositional := func() ([]value, error) {
		return e.evalPositionalArgs(mc.Args, locals)
	}

	switch r := receiver.(type) {
	case *manifold.SolidFuture:
		if !strings.HasPrefix(mc.Method, "_") {
			candidates := findMethods(e.stdMethods["Solid"], mc.Method, "*", len(argMap))
			if result, err, ok := e.resolveOverload(mc.Pos, mc.Method, candidates, argMap, e, stdlibCall(r)); ok {
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
		posArgs, err := evalPositional()
		if err != nil {
			return nil, err
		}
		result, err := solidMethod(e, r, mc.Method, posArgs)
		if err != nil {
			return nil, e.wrapErr(mc.Pos, err)
		}
		if rs, ok := result.(*manifold.SolidFuture); ok {
			e.trackSolid(mc.Pos, rs)
		}
		return result, nil

	case *manifold.SketchFuture:
		if !strings.HasPrefix(mc.Method, "_") {
			candidates := findMethods(e.stdMethods["Sketch"], mc.Method, "*", len(argMap))
			if result, err, ok := e.resolveOverload(mc.Pos, mc.Method, candidates, argMap, e, stdlibCall(r)); ok {
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
		posArgs, err := evalPositional()
		if err != nil {
			return nil, err
		}
		result, err := sketchMethod(r, mc.Method, posArgs)
		if err != nil {
			return nil, e.wrapErr(mc.Pos, err)
		}
		return result, nil

	case *libRef:
		candidates := findMethods(e.prog.Sources[e.prog.Resolve(r.path)].Functions, mc.Method, "", len(argMap))
		if len(candidates) == 0 {
			return nil, e.errAt(mc.Pos, "library has no function %q", mc.Method)
		}
		libEval := e.newLibEval(r)
		result, err, _ := e.resolveOverload(mc.Pos, mc.Method, candidates, argMap, libEval,
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
			posArgs, posErr := evalPositional()
			if posErr != nil {
				return nil, posErr
			}
			if result, err := structBuiltinMethod(r, mc.Method, posArgs); err == nil {
				return result, nil
			} else if !strings.HasPrefix(err.Error(), "no builtin method") {
				return nil, e.wrapErr(mc.Pos, err)
			}
		}

		// User-defined methods for this struct type
		candidates := findMethods(e.prog.Sources[e.currentKey].Functions, mc.Method, r.typeName, len(argMap))
		if result, err, ok := e.resolveOverload(mc.Pos, mc.Method, candidates, argMap, e,
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
			libCandidates := findMethods(e.prog.Sources[e.prog.Resolve(r.lib.path)].Functions, mc.Method, r.typeName, len(argMap))
			if len(libCandidates) > 0 {
				libEval := e.newLibEval(r.lib)
				result, err, _ := e.resolveOverload(mc.Pos, mc.Method, libCandidates, argMap, libEval,
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
		stdCandidates := findMethods(e.stdMethods[r.typeName], mc.Method, "*", len(argMap))
		if result, err, ok := e.resolveOverload(mc.Pos, mc.Method, stdCandidates, argMap, e, stdlibCall(r)); ok {
			if rs, ok := result.(*manifold.SolidFuture); ok {
				e.trackSolid(mc.Pos, rs)
			}
			return result, err
		}

		return nil, e.errAt(mc.Pos, "struct %s has no method %q", r.typeName, mc.Method)

	case string:
		if !strings.HasPrefix(mc.Method, "_") {
			candidates := findMethods(e.stdMethods["String"], mc.Method, "*", len(argMap))
			if result, err, ok := e.resolveOverload(mc.Pos, mc.Method, candidates, argMap, e, stdlibCall(r)); ok {
				return result, err
			}
		}
		posArgs, err := evalPositional()
		if err != nil {
			return nil, err
		}
		result, err := stringMethod(r, mc.Method, posArgs)
		if err != nil {
			return nil, e.wrapErr(mc.Pos, err)
		}
		return result, nil

	default:
		return nil, e.errAt(mc.Pos, "cannot call method %s on %s", mc.Method, typeName(receiver))
	}
}
