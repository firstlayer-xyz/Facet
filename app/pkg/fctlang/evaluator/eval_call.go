package evaluator

import (
	"facet/app/pkg/fctlang/loader"
	"facet/app/pkg/fctlang/parser"
	"facet/app/pkg/manifold"
)

func (e *evaluator) evalCall(call *parser.CallExpr, locals map[string]value) (value, error) {
	if err := e.ctx.Err(); err != nil {
		return nil, err
	}

	// Evaluate arguments
	args := make([]value, len(call.Args))
	for i, argExpr := range call.Args {
		v, err := e.evalExpr(argExpr, locals)
		if err != nil {
			return nil, err
		}
		args[i] = v
	}

	// Check if the callee is a functionVal stored in local or global scope.
	if fv, ok := unwrap(locals[call.Name]).(*functionVal); ok {
		v, err := e.callFunctionVal(fv, args)
		if err != nil {
			return nil, e.wrapErr(call.Pos, err)
		}
		return v, nil
	}
	if fv, ok := unwrap(e.globals[call.Name]).(*functionVal); ok {
		v, err := e.callFunctionVal(fv, args)
		if err != nil {
			return nil, e.wrapErr(call.Pos, err)
		}
		return v, nil
	}

	// Try internal _snake_case builtins via registry
	if fn, ok := builtinRegistry[call.Name]; ok {
		v, err := fn(e, stripNamedArgs(args))
		if err != nil {
			return nil, e.wrapErr(call.Pos, err)
		}
		return v, nil
	}

	// Collect user-defined function candidates (match by name, no receiver, and arg count)
	var userCandidates []*parser.Function
	var userFallback *parser.Function
	for _, fn := range e.prog.Sources[e.currentKey].Functions {
		if fn.Name == call.Name && fn.ReceiverType == "" {
			if fn.ArgsInRange(len(args)) {
				userCandidates = append(userCandidates, fn)
			} else if userFallback == nil {
				userFallback = fn
			}
		}
	}
	if v, ok, err := e.resolveCall(call, userCandidates, args, locals, false); err != nil {
		return nil, err
	} else if ok {
		return v, nil
	}

	// Collect stdlib function candidates
	var stdCandidates []*parser.Function
	var stdFallback *parser.Function
	for _, fn := range e.stdFuncs {
		if fn.Name == call.Name {
			if fn.ArgsInRange(len(args)) {
				stdCandidates = append(stdCandidates, fn)
			} else if stdFallback == nil {
				stdFallback = fn
			}
		}
	}
	if v, ok, err := e.resolveCall(call, stdCandidates, args, locals, true); err != nil {
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
				call.Name, len(fb.Params), len(args))
		}
		return nil, e.errAt(call.Pos, "%s() expects %d to %d arguments, got %d",
			call.Name, required, len(fb.Params), len(args))
	}

	return nil, e.errAt(call.Pos, "unknown function %q", call.Name)
}

// resolveCall delegates to resolveOverload for direct function calls.
func (e *evaluator) resolveCall(call *parser.CallExpr, candidates []*parser.Function, args []value, locals map[string]value, stdlib bool) (value, bool, error) {
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


