package evaluator

import (
	"facet/pkg/fctlang/parser"
	"facet/pkg/manifold"
	"fmt"
)

// stmtKind returns a user-facing description of a statement node.
func stmtKind(s parser.Stmt) string {
	switch s.(type) {
	case *parser.ExprStmt:
		return "expression"
	case *parser.ReturnStmt:
		return "return"
	case *parser.VarStmt:
		return "variable declaration"
	case *parser.YieldStmt:
		return "yield"
	case *parser.AssignStmt:
		return "assignment"
	case *parser.FieldAssignStmt:
		return "field assignment"
	case *parser.IfStmt:
		return "if"
	case *parser.AssertStmt:
		return "assert"
	default:
		return "unknown"
	}
}

// fillDefaults fills missing optional parameters with their evaluated default values.
// It mutates args in-place.
func (e *evaluator) fillDefaults(fn *parser.Function, args map[string]value, locals map[string]value) error {
	for _, p := range fn.Params {
		if _, ok := args[p.Name]; ok {
			continue // already provided
		}
		if p.Default == nil {
			if parser.IsOptionalType(p.Type) {
				// An omitted optional parameter binds None (bare nil — the
				// inner type is stamped on by coerceArgs against p.Type).
				args[p.Name] = none("")
				continue
			}
			return fmt.Errorf("%s() missing required parameter %q", fn.Name, p.Name)
		}
		v, err := e.evalExpr(p.Default, locals)
		if err != nil {
			return err
		}
		args[p.Name] = v
	}
	return nil
}

// funcReturn applies return type coercion and checking for a function-level return.
func (e *evaluator) funcReturn(retType string, v value, locals map[string]value) (value, error) {
	if retType != "" {
		v = e.coerceToType(retType, v, locals)
		if !checkType(retType, v) {
			return nil, fmt.Errorf("declared return type %s, but returned %s", retType, typeName(v))
		}
	}
	return v, nil
}

// catchReturn checks if an error is a returnSignal and, if so, handles it as a
// function-level return with type coercion. Returns (value, true, nil) if caught,
// or (nil, false, err) to propagate.
func (e *evaluator) catchReturn(err error, retType string, locals map[string]value) (value, bool, error) {
	if rs, ok := err.(*returnSignal); ok {
		v, err := e.funcReturn(retType, rs.val, locals)
		return v, true, err
	}
	return nil, false, err
}

// execBody executes a slice of statements with the given local scope, enforcing retType on
// return values. It is the shared implementation used by evalFunction, evalMethodFunction,
// and callFunctionVal. It catches returnSignal errors from nested blocks/ifs and treats
// them as function-level returns.
func (e *evaluator) execBody(stmts []parser.Stmt, retType string, locals map[string]value) (value, error) {
	// yield is lexically scoped to comprehension bodies, but it routes through
	// the evaluator-global yieldTarget/foldAcc, which have dynamic extent. Cut
	// both at the function boundary so a function called from inside a
	// comprehension can never inject values into the caller's loop (e.g. a
	// lambda body smuggling a yield).
	prevYield, prevFold := e.yieldTarget, e.foldAcc
	e.yieldTarget, e.foldAcc = nil, nil
	defer func() {
		e.yieldTarget = prevYield
		e.foldAcc = prevFold
	}()
	policy := &stmtPolicy{
		context:           "body",
		catchReturnSignal: true,
		retType:           retType,
		// YieldStmt intentionally has no handler: yield at the top level of
		// a function body is unexpected and dispatchStmt's default branch
		// produces "unexpected yield statement in body" — same message the
		// pre-unification switch emitted.
		onReturn: func(s *parser.ReturnStmt, locals map[string]value) (bool, value, error) {
			v, err := e.evalExpr(s.Value, locals)
			if err != nil {
				if rv, ok, err2 := e.catchReturn(err, retType, locals); ok {
					return true, rv, err2
				}
				return true, nil, err
			}
			rv, rerr := e.funcReturn(retType, v, locals)
			return true, rv, rerr
		},
	}
	for _, stmt := range stmts {
		done, rv, err := e.dispatchStmt(stmt, locals, policy)
		if err != nil {
			return nil, err
		}
		if done {
			return rv, nil
		}
	}
	return nil, nil
}

const maxCallDepth = 1000
const maxRangeSize = 10_000_000

// maxSegments caps circular resolution (segments/slices for Sphere, Cylinder,
// Circle, Revolve, Extrude). A sphere has ~segments² triangles and the C++
// kernel does not clamp, so an unbounded count from source (e.g. 1e8) would
// OOM/hang the host. 10000 is far beyond any real model (~10⁸ triangles at the
// cap). maxRefine caps Refine's per-edge subdivision factor (triangle count
// grows ~n²). Both are conservative ceilings, tunable if a model legitimately
// needs more.
const maxSegments = 10000
const maxRefine = 1000

func (e *evaluator) evalFunction(fn *parser.Function, args map[string]value) (value, error) {
	e.callDepth++
	if e.callDepth > maxCallDepth {
		e.callDepth--
		return nil, e.errAt(fn.Pos, "maximum call depth exceeded (%d) — possible infinite recursion", maxCallDepth)
	}
	defer func() { e.callDepth-- }()
	locals := make(map[string]value, len(e.globals)+len(fn.Params))
	for k, v := range e.globals {
		locals[k] = v
	}
	for _, param := range fn.Params {
		bound, ok := args[param.Name]
		if !ok {
			// Defence in depth: callers (resolveOverload, run) always pass a
			// complete arg map. A missing entry here is an evaluator-internal
			// bug — surface it loudly rather than binding nil and crashing
			// further down with a less-actionable error.
			return nil, e.errAt(fn.Pos, "%s() missing internal argument %q (evaluator bug — please report)", fn.Name, param.Name)
		}
		// Value semantics: a struct argument binds a copy, so a field
		// assignment in the body affects only the parameter, never the caller.
		v := copyValue(bound)
		if param.Constraint != nil {
			if err := e.validateConstraint(param.Name, param.Constraint, v, locals); err != nil {
				return nil, err
			}
			locals[param.Name] = &constrainedVal{inner: v, constraint: param.Constraint, name: param.Name}
		} else {
			locals[param.Name] = v
		}
	}
	result, err := e.execBody(fn.Body, fn.ReturnType, locals)
	if err == nil {
		if rs, ok := result.(*manifold.Solid); ok {
			e.trackSolid(fn.Pos, rs)
		}
	}
	return result, err
}

// evalMethodFunction evaluates a stdlib method definition, injecting `self` into the local scope.
func (e *evaluator) evalMethodFunction(fn *parser.Function, self value, args map[string]value) (value, error) {
	e.callDepth++
	if e.callDepth > maxCallDepth {
		e.callDepth--
		return nil, e.errAt(fn.Pos, "maximum call depth exceeded (%d) — possible infinite recursion", maxCallDepth)
	}
	defer func() { e.callDepth-- }()
	if err := e.fillDefaults(fn, args, e.globals); err != nil {
		return nil, err
	}
	if err := e.coerceArgs(fn.Name, fn.Params, args, e.globals); err != nil {
		return nil, err
	}
	locals := make(map[string]value, len(e.globals)+len(fn.Params)+1)
	for k, v := range e.globals {
		locals[k] = v
	}
	locals["self"] = self
	for _, param := range fn.Params {
		// Params copy (value semantics); self deliberately does NOT — a method
		// mutating self is the language's one in-place mutation channel, and
		// the mutation persists to the receiver variable.
		v := copyValue(args[param.Name])
		if param.Constraint != nil {
			if err := e.validateConstraint(param.Name, param.Constraint, v, locals); err != nil {
				return nil, err
			}
			locals[param.Name] = &constrainedVal{inner: v, constraint: param.Constraint, name: param.Name}
		} else {
			locals[param.Name] = v
		}
	}
	result, err := e.execBody(fn.Body, fn.ReturnType, locals)
	if err == nil {
		if rs, ok := result.(*manifold.Solid); ok {
			e.trackSolid(fn.Pos, rs)
		}
	}
	return result, err
}

// callFunctionVal evaluates a first-class function (lambda) call.
// Scope is built as: globals → captured (overrides) → args.
// Globals are included as fallback so recursive lambdas can reference themselves by name.
func (e *evaluator) callFunctionVal(fv *functionVal, args map[string]value) (value, error) {
	e.callDepth++
	if e.callDepth > maxCallDepth {
		e.callDepth--
		return nil, fmt.Errorf("maximum call depth exceeded (%d) — possible infinite recursion", maxCallDepth)
	}
	defer func() { e.callDepth-- }()
	if len(args) != len(fv.params) {
		return nil, fmt.Errorf("function expects %d arguments, got %d", len(fv.params), len(args))
	}
	// Validate arg names match declared parameters before any coercion runs.
	paramNames := make(map[string]bool, len(fv.params))
	for _, p := range fv.params {
		paramNames[p.Name] = true
	}
	for name := range args {
		if !paramNames[name] {
			return nil, fmt.Errorf("function has no parameter named %q", name)
		}
	}
	// Coerce each arg to its declared parameter type — same rule as
	// evalFunction / evalMethodFunction. Without this a lambda declared
	// `fn(x Number)` called with a Length value would see a Length in the
	// body and any numeric op would fail with a confusing type error.
	//
	// Free names resolve against the lambda's DEFINING globals, not the
	// invoking evaluator's — a library or stdlib body calling a user lambda
	// must not substitute its own globals for the user's. (Every functionVal
	// is built by the LambdaExpr case with globals set.)
	defGlobals := fv.globals
	if err := e.coerceArgs("lambda", fv.params, args, defGlobals); err != nil {
		return nil, err
	}
	scope := make(map[string]value, len(defGlobals)+len(fv.captured)+len(fv.params))
	for k, v := range defGlobals {
		scope[k] = v
	}
	// Each CALL gets fresh copies of the captured snapshot: a body field
	// assignment must not persist into the next invocation (an Animation frame
	// closure mutating a captured struct made Frame(t) history-dependent).
	for k, v := range fv.captured {
		scope[k] = copyValue(v)
	}
	for name, val := range args {
		scope[name] = copyValue(val)
	}
	return e.execBody(fv.body, fv.retType, scope)
}

// evalFieldAssign evaluates a struct field assignment: receiver.field = value
func (e *evaluator) evalFieldAssign(s *parser.FieldAssignStmt, locals map[string]value) error {
	// Arrays are immutable: their backing slice is shared across bindings
	// (copyValue passes arrays through), so writing through an element would
	// co-mutate every binding of the array. Rejected, not copied.
	if parser.ReceiverHasIndex(s.Receiver) {
		return e.errAt(s.Pos, "cannot assign through an array element — arrays are immutable (build a new array with a comprehension)")
	}
	if root := parser.ReceiverRoot(s.Receiver); root != nil {
		if v, exists := locals[root.Name]; exists {
			// Deep const: cfg.inner.x = … is as much a mutation of const cfg
			// as cfg.x = ….
			if isConst(v) {
				return e.errAt(s.Pos, "cannot mutate field on const %q", root.Name)
			}
			// A module-level struct reaches function locals by reference (the
			// globals map is the scope fallback). Mutating it from a function
			// would be spooky action at a distance — reassign at top level or
			// work on a local copy instead.
			if gv, isGlobal := e.globals[root.Name]; isGlobal && unwrap(gv) == unwrap(v) {
				if _, isStruct := unwrap(v).(*structVal); isStruct {
					return e.errAt(s.Pos, "cannot mutate module-level %q from inside a function (assign it to a local first, or reassign it at top level)", root.Name)
				}
			}
		}
	}
	recv, err := e.evalExpr(s.Receiver, locals)
	if err != nil {
		return err
	}
	sv, ok := unwrap(recv).(*structVal)
	if !ok {
		return e.errAt(s.Pos, "cannot assign field %q on %s", s.Field, typeName(recv))
	}
	if _, exists := sv.fields[s.Field]; !exists {
		return e.errAt(s.Pos, "struct %s has no field %q", sv.typeName, s.Field)
	}
	v, err := e.evalExpr(s.Value, locals)
	if err != nil {
		return err
	}
	// Type-check and coerce against the declared field type.
	if sv.decl != nil {
		for _, f := range sv.decl.Fields {
			if f.Name == s.Field {
				v = e.coerceToType(f.Type, v, locals)
				if !checkType(f.Type, v) {
					return e.errAt(s.Pos, "cannot assign %s to field %q of type %s", typeName(v), s.Field, f.Type)
				}
				if f.Constraint != nil {
					if cerr := e.validateConstraint(f.Name, f.Constraint, v, locals); cerr != nil {
						return e.wrapErr(s.Pos, cerr)
					}
				}
				break
			}
		}
	}
	sv.fields[s.Field] = v
	return nil
}
