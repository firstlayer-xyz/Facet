package evaluator

import (
	"fmt"

	"facet/pkg/fctlang/loader"
	"facet/pkg/fctlang/parser"
	"facet/pkg/manifold"
)

// evalNamedArgs evaluates call arguments and builds a map[string]value keyed by
// the NamedArg name. All arguments must be named (e.g. `name: value`); a
// repeated name is an error rather than a silent last-wins overwrite.
func (e *evaluator) evalNamedArgs(args []parser.Expr, locals map[string]value) (map[string]value, error) {
	result := make(map[string]value, len(args))
	for _, argExpr := range args {
		na, ok := argExpr.(*parser.NamedArg)
		if !ok {
			return nil, fmt.Errorf("arguments must be named (e.g. name: value)")
		}
		if _, dup := result[na.Name]; dup {
			return nil, fmt.Errorf("duplicate argument %q", na.Name)
		}
		v, err := e.evalExpr(na.Value, locals)
		if err != nil {
			return nil, err
		}
		result[na.Name] = v
	}
	return result, nil
}

// evalPositionalArgs evaluates call arguments into a positional []value slice,
// stripping NamedArg wrappers if present (for builtins that don't use names).
func (e *evaluator) evalPositionalArgs(args []parser.Expr, locals map[string]value) ([]value, error) {
	result := make([]value, len(args))
	for i, argExpr := range args {
		// Builtins are positional; strip a NamedArg wrapper to its value.
		if na, ok := argExpr.(*parser.NamedArg); ok {
			argExpr = na.Value
		}
		v, err := e.evalExpr(argExpr, locals)
		if err != nil {
			return nil, err
		}
		result[i] = v
	}
	return result, nil
}


func (e *evaluator) evalBuiltinCall(call *parser.BuiltinCallExpr, locals map[string]value) (value, error) {
	if err := e.ctx.Err(); err != nil {
		return nil, err
	}
	fn, ok := builtinRegistry[call.Name]
	if !ok {
		return nil, e.errAt(call.Pos, "unknown builtin %q", call.Name)
	}
	args, err := e.evalPositionalArgs(call.Args, locals)
	if err != nil {
		return nil, err
	}
	v, err := fn(e, args)
	if err != nil {
		return nil, e.wrapErr(call.Pos, err)
	}
	return v, nil
}

func (e *evaluator) evalCall(call *parser.CallExpr, locals map[string]value) (value, error) {
	if err := e.ctx.Err(); err != nil {
		return nil, err
	}

	// Check if the callee is a functionVal stored in local or global scope.
	if fv, ok := unwrap(locals[call.Name]).(*functionVal); ok {
		argMap, err := e.evalNamedArgs(call.Args, locals)
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
		argMap, err := e.evalNamedArgs(call.Args, locals)
		if err != nil {
			return nil, err
		}
		v, err := e.callFunctionVal(fv, argMap)
		if err != nil {
			return nil, e.wrapErr(call.Pos, err)
		}
		return v, nil
	}

	// Collect all function candidates.
	// nArgs=-1 skips arity filtering; evalCall filters by arity after evaluating args.
	userCandidates, userFallback := parser.CollectCandidates(
		e.prog.Sources[e.currentKey].Functions(), call.Name, -1, true)
	stdCandidates, stdFallback := parser.CollectCandidates(e.stdFuncs, call.Name, -1, false)

	// Build named args map — all arguments must be named
	argMap, err := e.evalNamedArgs(call.Args, locals)
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
			if p.IsRequired() {
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
	// Capture the call site's file before any override so the debug step is
	// attributed to the caller's source, not the function body's file.
	callSiteFile := e.file
	if stdlib {
		e.file = loader.StdlibPath
		defer func() { e.file = callSiteFile }()
	}
	result, err := e.evalFunction(fn, args)
	if err != nil {
		return nil, e.wrapErr(call.Pos, err)
	}
	// Temporarily restore call-site file so recordStep uses the right file.
	prevFile := e.file
	e.file = callSiteFile
	if s, ok := result.(*manifold.Solid); ok {
		e.trackSolid(call.Pos, s)
		e.recordStep(call.Name, call.Pos, debugEntry{Role: "result", Shape: s})
	} else if sk, ok := result.(*manifold.Sketch); ok {
		e.recordStep(call.Name, call.Pos, debugEntry{Role: "result", Shape: sk})
	}
	e.file = prevFile
	return result, nil
}
