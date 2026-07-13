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
	fv, ok := unwrap(locals[call.Name]).(*functionVal)
	if !ok {
		fv, ok = unwrap(e.globals[call.Name]).(*functionVal)
	}
	if ok {
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

	// Filter both candidate sets by arity
	var userArityMatch []*parser.Function
	for _, fn := range userCandidates {
		if fn.ArgsInRange(len(argMap)) {
			userArityMatch = append(userArityMatch, fn)
		}
	}
	var stdArityMatch []*parser.Function
	for _, fn := range stdCandidates {
		if fn.ArgsInRange(len(argMap)) {
			stdArityMatch = append(stdArityMatch, fn)
		}
	}

	// User code resolves user functions first (they may deliberately shadow a
	// stdlib name). A STDLIB body resolves stdlib-first: its internal calls are
	// lexically stdlib code, and a same-named user function must not hijack
	// them (e.g. a user Sqrt() corrupting Normalize()).
	first, second := userArityMatch, stdArityMatch
	firstIsStd, secondIsStd := false, true
	if e.inStdlib {
		first, second = stdArityMatch, userArityMatch
		firstIsStd, secondIsStd = true, false
	}
	if v, ok, err := e.resolveCall(call, first, argMap, locals, firstIsStd); err != nil {
		return nil, err
	} else if ok {
		return v, nil
	}
	if v, ok, err := e.resolveCall(call, second, argMap, locals, secondIsStd); err != nil {
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

// enterStdlib switches the evaluator into stdlib-execution context: error/debug
// attribution moves to the stdlib file, free names resolve against the hermetic
// stdlib globals (a user `var PI = 3` must not leak in), and evalCall prefers
// stdlib candidates (a user function named like a stdlib helper must not hijack
// stdlib internals). Returns the restore func for defer.
func (e *evaluator) enterStdlib() func() {
	savedFile, savedGlobals, savedIn := e.file, e.globals, e.inStdlib
	e.file = loader.StdlibPath
	if e.stdGlobals != nil {
		e.globals = e.stdGlobals
	}
	e.inStdlib = true
	return func() {
		e.file, e.globals, e.inStdlib = savedFile, savedGlobals, savedIn
	}
}

// callResolved evaluates a resolved function call, handling stdlib file
// attribution and debug step recording.
func (e *evaluator) callResolved(call *parser.CallExpr, fn *parser.Function, args map[string]value, stdlib bool) (value, error) {
	// Capture the call site's file before any override so the debug step is
	// attributed to the caller's source, not the function body's file.
	callSiteFile := e.file
	if stdlib {
		defer e.enterStdlib()()
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
