package evaluator

import (
	"facet/app/pkg/fctlang/loader"
	"facet/app/pkg/fctlang/parser"
	"fmt"

	"facet/app/pkg/manifold"
)

// evalNamedArgs evaluates call arguments and builds a map[string]value keyed by
// the NamedArg name. If any argument is not a NamedArg (bare positional arg from
// internal/stdlib code), it falls back to positional mapping using paramNames.
// paramNames provides the positional-to-name mapping for bare args.
func (e *evaluator) evalNamedArgs(args []parser.Expr, locals map[string]value, paramNames []string) (map[string]value, error) {
	result := make(map[string]value, len(args))
	for i, argExpr := range args {
		na, ok := argExpr.(*parser.NamedArg)
		if ok {
			v, err := e.evalExpr(na.Value, locals)
			if err != nil {
				return nil, err
			}
			result[na.Name] = v
		} else {
			// Bare positional arg — map using paramNames
			v, err := e.evalExpr(argExpr, locals)
			if err != nil {
				return nil, err
			}
			if i < len(paramNames) {
				result[paramNames[i]] = v
			} else {
				// More args than params — store by index as fallback key
				result[fmt.Sprintf("_%d", i)] = v
			}
		}
	}
	return result, nil
}

// evalPositionalArgs evaluates call arguments into a positional []value slice,
// stripping NamedArg wrappers if present (for builtins that don't use names).
func (e *evaluator) evalPositionalArgs(args []parser.Expr, locals map[string]value) ([]value, error) {
	result := make([]value, len(args))
	for i, argExpr := range args {
		// If it's a NamedArg, evaluate just the value (builtins are positional).
		if na, ok := argExpr.(*parser.NamedArg); ok {
			v, err := e.evalExpr(na.Value, locals)
			if err != nil {
				return nil, err
			}
			result[i] = v
		} else {
			v, err := e.evalExpr(argExpr, locals)
			if err != nil {
				return nil, err
			}
			result[i] = v
		}
	}
	return result, nil
}

// paramNamesFromFunctions extracts the union of param names from a slice of function
// candidates, returning the param names from the first candidate (for positional mapping).
func paramNamesFromFunctions(fns []*parser.Function) []string {
	if len(fns) == 0 {
		return nil
	}
	names := make([]string, len(fns[0].Params))
	for i, p := range fns[0].Params {
		names[i] = p.Name
	}
	return names
}

// fvParamNames extracts param names from a functionVal for positional mapping.
func fvParamNames(fv *functionVal) []string {
	names := make([]string, len(fv.params))
	for i, p := range fv.params {
		names[i] = p.Name
	}
	return names
}

func (e *evaluator) evalCall(call *parser.CallExpr, locals map[string]value) (value, error) {
	if err := e.ctx.Err(); err != nil {
		return nil, err
	}

	// Check if the callee is a functionVal stored in local or global scope.
	if fv, ok := unwrap(locals[call.Name]).(*functionVal); ok {
		argMap, err := e.evalNamedArgs(call.Args, locals, fvParamNames(fv))
		if err != nil {
			return nil, err
		}
		v, err := e.callFunctionVal(fv, argMap)
		if err != nil {
			return nil, e.wrapErr(call.Pos, err)
		}
		return v, nil
	}
	if fv, ok := unwrap(e.globals[call.Name]).(*functionVal); ok {
		argMap, err := e.evalNamedArgs(call.Args, locals, fvParamNames(fv))
		if err != nil {
			return nil, err
		}
		v, err := e.callFunctionVal(fv, argMap)
		if err != nil {
			return nil, e.wrapErr(call.Pos, err)
		}
		return v, nil
	}

	// Try internal _snake_case builtins via registry (positional args)
	if fn, ok := builtinRegistry[call.Name]; ok {
		positionalArgs, err := e.evalPositionalArgs(call.Args, locals)
		if err != nil {
			return nil, err
		}
		v, err := fn(e, positionalArgs)
		if err != nil {
			return nil, e.wrapErr(call.Pos, err)
		}
		return v, nil
	}

	// Collect all function candidates first (to get param names for positional mapping).
	// nArgs=-1 skips arity filtering; evalCall filters by arity after evaluating args.
	userCandidates, userFallback := parser.CollectCandidates(
		e.prog.Sources[e.currentKey].Functions, call.Name, -1, true)
	stdCandidates, stdFallback := parser.CollectCandidates(e.stdFuncs, call.Name, -1, false)

	// Build param names for positional mapping from the first available candidate.
	var paramNames []string
	if len(userCandidates) > 0 {
		paramNames = paramNamesFromFunctions(userCandidates)
	} else if len(stdCandidates) > 0 {
		paramNames = paramNamesFromFunctions(stdCandidates)
	}

	// Build named args map
	argMap, err := e.evalNamedArgs(call.Args, locals, paramNames)
	if err != nil {
		return nil, err
	}

	// Filter user candidates by arity
	var userArityMatch []*parser.Function
	for _, fn := range userCandidates {
		if fn.ArgsInRange(len(argMap)) {
			userArityMatch = append(userArityMatch, fn)
		}
	}
	if v, ok, err := e.resolveCall(call, userArityMatch, argMap, locals, false); err != nil {
		return nil, err
	} else if ok {
		return v, nil
	}

	// Filter stdlib candidates by arity
	var stdArityMatch []*parser.Function
	for _, fn := range stdCandidates {
		if fn.ArgsInRange(len(argMap)) {
			stdArityMatch = append(stdArityMatch, fn)
		}
	}
	if v, ok, err := e.resolveCall(call, stdArityMatch, argMap, locals, true); err != nil {
		return nil, err
	} else if ok {
		return v, nil
	}

	// No arity match — report arity error if the function name exists
	fb := userFallback
	if fb == nil {
		fb = stdFallback
	}
	if fb != nil {
		required := 0
		for _, p := range fb.Params {
			if p.Default == nil {
				required++
			}
		}
		if required == len(fb.Params) {
			return nil, e.errAt(call.Pos, "%s() expects %d arguments, got %d",
				call.Name, len(fb.Params), len(argMap))
		}
		return nil, e.errAt(call.Pos, "%s() expects %d to %d arguments, got %d",
			call.Name, required, len(fb.Params), len(argMap))
	}

	return nil, e.errAt(call.Pos, "unknown function %q", call.Name)
}

// resolveCall delegates to resolveOverload for direct function calls.
func (e *evaluator) resolveCall(call *parser.CallExpr, candidates []*parser.Function, args map[string]value, locals map[string]value, stdlib bool) (value, bool, error) {
	result, err, ok := e.resolveOverload(call.Pos, call.Name, candidates, args, e,
		func(fn *parser.Function, resolvedArgs map[string]value) (value, error) {
			return e.callResolved(call, fn, resolvedArgs, stdlib)
		})
	if ok {
		return result, true, err
	}
	return nil, false, err
}

// callResolved evaluates a resolved function call, handling stdlib file
// attribution and debug step recording.
func (e *evaluator) callResolved(call *parser.CallExpr, fn *parser.Function, args map[string]value, stdlib bool) (value, error) {
	if stdlib {
		savedFile := e.file
		e.file = loader.StdlibPath
		result, err := e.evalFunction(fn, args)
		e.file = savedFile
		if err != nil {
			return nil, e.wrapErr(call.Pos, err)
		}
		if s, ok := result.(*manifold.SolidFuture); ok {
			e.trackSolid(call.Pos, s)
			e.recordStep(call.Name, call.Pos, debugRole{"result", s})
		} else if sk, ok := result.(*manifold.SketchFuture); ok {
			e.recordStep(call.Name, call.Pos, debugRole{"result", sk})
		}
		return result, nil
	}
	result, err := e.evalFunction(fn, args)
	if err != nil {
		return nil, e.wrapErr(call.Pos, err)
	}
	if s, ok := result.(*manifold.SolidFuture); ok {
		e.trackSolid(call.Pos, s)
	}
	return result, nil
}
