package evaluator

import (
	"sort"
	"strings"

	"facet/pkg/fctlang/parser"
	"facet/pkg/manifold"
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

// funcHasParam reports whether fn declares a parameter named name. A linear
// scan is cheaper than a map for the small parameter lists Facet functions
// have, and allocates nothing on the hot path.
func funcHasParam(fn *parser.Function, name string) bool {
	for _, p := range fn.Params {
		if p.Name == name {
			return true
		}
	}
	return false
}

// paramMatchScore scores how specifically a runtime value of type gotType
// satisfies a parameter declared as paramType. It mirrors the checker's
// findOverload scoring (check_call.go): an exact match is most specific, a
// Number→Length/Angle coercion less so, and an Any/var param least. It never
// disqualifies — the sort in resolveOverload only orders attempts, and coerceArgs
// is the final arbiter of a match. (Kept a small independent copy rather than
// coupling the evaluator to the checker package across the boundary.)
func paramMatchScore(paramType, gotType string) int {
	switch paramType {
	case "Any", "[]Any":
		return 0
	}
	if paramType == gotType {
		return 2
	}
	if (paramType == "Length" || paramType == "Angle") && gotType == "Number" {
		return 1
	}
	return 0
}

// overloadSpecificity sums paramMatchScore over the parameters an overload
// actually receives an argument for (a parameter left to its default contributes
// nothing), so resolveOverload can try the most-specific candidate first.
func overloadSpecificity(fn *parser.Function, args map[string]value) int {
	score := 0
	for _, p := range fn.Params {
		if v, ok := args[p.Name]; ok {
			score += paramMatchScore(p.Type, typeName(unwrap(v)))
		}
	}
	return score
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

	// Try candidates most-specific-first so dispatch is independent of
	// declaration order and agrees with the checker's overload resolution
	// (findOverload, check_call.go). Without this, a looser overload (e.g. an
	// `Any` param) declared before an exact one shadowed it at runtime but not in
	// the editor. coerceArgs below is still the final arbiter of a match; the sort
	// only orders the attempts. Stable so equal-specificity candidates keep
	// declaration order. Skipped for the common single-candidate case.
	if len(candidates) > 1 {
		sorted := make([]*parser.Function, len(candidates))
		copy(sorted, candidates)
		sort.SliceStable(sorted, func(i, j int) bool {
			return overloadSpecificity(sorted[i], args) > overloadSpecificity(sorted[j], args)
		})
		candidates = sorted
	}

	singleCandidate := len(candidates) == 1
	for _, fn := range candidates {
		// Validate that all provided arg names match declared parameters. A
		// linear scan over the (small) parameter list avoids allocating a
		// lookup map on every call — this path runs once per nested call inside
		// hot loops such as Solid.Warp.
		badName := false
		for name := range args {
			if !funcHasParam(fn, name) {
				badName = true
				break
			}
		}
		if badName {
			if !singleCandidate {
				continue
			}
			// Single candidate — produce a specific error for bad names.
			for name := range args {
				if !funcHasParam(fn, name) {
					return nil, e.errAt(pos, "%s() has no parameter named %q", method, name), true
				}
			}
		}

		// With multiple candidates we copy the args map: fillDefaults/coerceArgs
		// mutate it in place, and a failed candidate falls through to the next,
		// which must see the original arg names and values. With a single
		// candidate there is no fallthrough, and resolveOverload returns ok=true
		// on this path (callers return immediately and never read args again),
		// so mutating the caller's map directly is safe and skips an allocation.
		argMap := args
		if !singleCandidate {
			argMap = make(map[string]value, len(args))
			for k, v := range args {
				argMap[k] = v
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
	argTypeNames := make([]string, 0, len(args))
	for name, v := range args {
		argTypeNames = append(argTypeNames, name+": "+typeName(v))
	}
	return nil, e.errAt(pos, "no matching overload for %s(%s)",
		method, strings.Join(argTypeNames, ", ")), true
}

// newLibEval creates a sub-evaluator scoped to a library's program, with the
// given globals and stdlib-globals maps (method dispatch passes the lib's cached
// globals; evalLibExpr passes a fresh map it is about to populate). opFuncs is
// rebuilt for the library's source (stdlib + the lib's own operator functions)
// so a lib body that uses Vec3 + Vec3 or its own custom operators dispatches
// correctly. libLoadStack is inherited so circular-import detection survives
// nested method-triggered lib loads.
func (e *evaluator) newLibEval(lib *libRef, globals, stdGlobals map[string]value) *evaluator {
	diskPath := e.prog.Resolve(lib.path)
	return &evaluator{
		ctx:          e.ctx,
		prog:         e.prog,
		currentKey:   diskPath,
		globals:      globals,
		stdGlobals:   stdGlobals,
		debug:        e.debug,
		libEvalCache: e.libEvalCache,
		libLoadStack: e.libLoadStack,
		file:         diskPath,
		libSources:   e.libSources,
		stdFuncs:     e.stdFuncs,
		stdMethods:   e.stdMethods,
		structDecls:  buildStructDecls(e.prog, diskPath),
		opFuncs:      buildOpFuncs(e.prog, diskPath),
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

	argMap, err := e.evalNamedArgs(mc.Args, locals)
	if err != nil {
		return nil, err
	}

	// stdlibCall returns a callback that evaluates fn as a stdlib method on receiver.
	stdlibCall := func(receiver value) func(fn *parser.Function, args map[string]value) (value, error) {
		return func(fn *parser.Function, args map[string]value) (value, error) {
			defer e.enterStdlib()()
			return e.evalMethodFunction(fn, receiver, args)
		}
	}

	// `opt?.Method(args)`: None short-circuits; Some(v) unwraps and the
	// return value is re-wrapped in Some via chainOptionalWrap.
	chainOptionalWrap := false
	if mc.Optional {
		opt, ok := receiver.(*optionalVal)
		if !ok {
			return nil, e.errAt(mc.Pos, "?. operator requires an Optional receiver, got %s", typeName(receiver))
		}
		if !opt.present {
			return none(""), nil
		}
		receiver = opt.inner
		chainOptionalWrap = true
	}

	// Optional has no methods. The closed Optional API is the `??` operator,
	// `== nil` / `!= nil` for presence checks, and `if var x = opt { … }` for
	// scoped extraction — all three are language-level and don't need a
	// method surface that could later drift out of sync with the checker.
	if _, ok := receiver.(*optionalVal); ok {
		return nil, e.errAt(mc.Pos, "Optional has no methods; use ?? for fallback, == nil / != nil for presence, or `if var x = opt { ... }` to bind the inner value")
	}

	// dispatch is the original method-lookup body; result is wrapped if
	// we entered via `?.` so the surrounding type stays Optional.
	result, dispatchErr := e.dispatchMethodCall(mc, receiver, argMap, stdlibCall)
	if dispatchErr != nil {
		return nil, dispatchErr
	}
	if chainOptionalWrap {
		return some(result, ""), nil
	}
	return result, nil
}

// dispatchMethodCall evaluates `receiver.Method(args)` for a definite
// receiver. Optional methods and the optional-chaining unwrap happen in
// the caller.
func (e *evaluator) dispatchMethodCall(
	mc *parser.MethodCallExpr,
	receiver value,
	argMap map[string]value,
	stdlibCall func(receiver value) func(fn *parser.Function, args map[string]value) (value, error),
) (value, error) {

	switch r := receiver.(type) {
	case *manifold.Solid:
		candidates := findMethods(e.stdMethods["Solid"], mc.Method, "*", len(argMap))
		if result, err, ok := e.resolveOverload(mc.Pos, mc.Method, candidates, argMap, e, stdlibCall(r)); ok {
			if err != nil {
				return nil, err
			}
			if rs, ok := result.(*manifold.Solid); ok {
				e.trackSolid(mc.Pos, rs)
				e.recordStep(mc.Method, mc.Pos, debugEntry{"input", r}, debugEntry{"result", rs})
			}
			return result, nil
		}
		return nil, e.errAt(mc.Pos, "Solid has no method %q", mc.Method)

	case *manifold.Sketch:
		candidates := findMethods(e.stdMethods["Sketch"], mc.Method, "*", len(argMap))
		if result, err, ok := e.resolveOverload(mc.Pos, mc.Method, candidates, argMap, e, stdlibCall(r)); ok {
			if err != nil {
				return nil, err
			}
			if rs, ok := result.(*manifold.Solid); ok {
				e.trackSolid(mc.Pos, rs)
				e.recordStep(mc.Method, mc.Pos, debugEntry{"input", r}, debugEntry{"result", rs})
			} else if rsk, ok := result.(*manifold.Sketch); ok {
				e.recordStep(mc.Method, mc.Pos, debugEntry{"input", r}, debugEntry{"result", rsk})
			}
			return result, nil
		}
		return nil, e.errAt(mc.Pos, "Sketch has no method %q", mc.Method)

	case *libRef:
		candidates := findMethods(e.prog.Sources[e.prog.Resolve(r.path)].Functions(), mc.Method, "", len(argMap))
		if len(candidates) == 0 {
			return nil, e.errAt(mc.Pos, "library has no function %q", mc.Method)
		}
		libEval := e.newLibEval(r, e.libEvalCache[r.path], e.stdGlobals)
		result, err, _ := e.resolveOverload(mc.Pos, mc.Method, candidates, argMap, libEval,
			func(fn *parser.Function, args map[string]value) (value, error) {
				return libEval.evalFunction(fn, args)
			})
		if err != nil {
			return nil, err
		}
		e.steps = append(e.steps, libEval.steps...)
		if rs, ok := result.(*manifold.Solid); ok {
			e.trackSolid(mc.Pos, rs)
		}
		return result, nil

	case *structVal:
		// User-defined methods for this struct type
		candidates := findMethods(e.prog.Sources[e.currentKey].Functions(), mc.Method, r.typeName, len(argMap))
		if result, err, ok := e.resolveOverload(mc.Pos, mc.Method, candidates, argMap, e,
			func(fn *parser.Function, args map[string]value) (value, error) {
				return e.evalMethodFunction(fn, r, args)
			}); ok {
			if rs, ok := result.(*manifold.Solid); ok {
				e.trackSolid(mc.Pos, rs)
			}
			return result, err
		}

		// Library methods (if struct came from a library)
		if r.lib != nil {
			libCandidates := findMethods(e.prog.Sources[e.prog.Resolve(r.lib.path)].Functions(), mc.Method, r.typeName, len(argMap))
			if len(libCandidates) > 0 {
				libEval := e.newLibEval(r.lib, e.libEvalCache[r.lib.path], e.stdGlobals)
				result, err, _ := e.resolveOverload(mc.Pos, mc.Method, libCandidates, argMap, libEval,
					func(fn *parser.Function, args map[string]value) (value, error) {
						return libEval.evalMethodFunction(fn, r, args)
					})
				if err != nil {
					return nil, err
				}
				e.steps = append(e.steps, libEval.steps...)
				if rs, ok := result.(*manifold.Solid); ok {
					e.trackSolid(mc.Pos, rs)
				}
				return result, nil
			}
		}

		// Stdlib methods for this struct type (e.g. Box methods)
		stdCandidates := findMethods(e.stdMethods[r.typeName], mc.Method, "*", len(argMap))
		if result, err, ok := e.resolveOverload(mc.Pos, mc.Method, stdCandidates, argMap, e, stdlibCall(r)); ok {
			if rs, ok := result.(*manifold.Solid); ok {
				e.trackSolid(mc.Pos, rs)
			}
			return result, err
		}

		return nil, e.errAt(mc.Pos, "struct %s has no method %q", r.typeName, mc.Method)

	case string:
		candidates := findMethods(e.stdMethods["String"], mc.Method, "*", len(argMap))
		if result, err, ok := e.resolveOverload(mc.Pos, mc.Method, candidates, argMap, e, stdlibCall(r)); ok {
			return result, err
		}
		return nil, e.errAt(mc.Pos, "String has no method %q", mc.Method)

	default:
		return nil, e.errAt(mc.Pos, "cannot call method %s on %s", mc.Method, typeName(receiver))
	}
}
