package evaluator

import (
	"facet/app/pkg/fctlang/parser"
	"facet/app/pkg/manifold"
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
	for _, stmt := range stmts {
		if err := e.ctx.Err(); err != nil {
			return nil, err
		}
		switch s := stmt.(type) {
		case *parser.ReturnStmt:
			v, err := e.evalExpr(s.Value, locals)
			if err != nil {
				if v, ok, err2 := e.catchReturn(err, retType, locals); ok {
					return v, err2
				}
				return nil, err
			}
			return e.funcReturn(retType, v, locals)
		case *parser.VarStmt:
			v, err := e.evalExpr(s.Value, locals)
			if err != nil {
				if v, ok, err2 := e.catchReturn(err, retType, locals); ok {
					return v, err2
				}
				return nil, err
			}
			if s.Constraint != nil {
				if cerr := e.validateConstraint(s.Name, s.Constraint, v, locals); cerr != nil {
					return nil, e.wrapErr(s.Pos, cerr)
				}
			}
			cv := copyValue(v)
			if s.Constraint != nil {
				cv = &constrainedVal{inner: cv, constraint: s.Constraint, name: s.Name}
			}
			if s.IsConst {
				locals[s.Name] = &constVal{inner: cv}
			} else {
				locals[s.Name] = cv
			}
			e.trackIfSolid(s.Pos, cv)
		case *parser.AssignStmt:
			existing, ok := locals[s.Name]
			if !ok {
				return nil, e.errAt(s.Pos, "cannot assign to undefined variable %q", s.Name)
			}
			if isConst(existing) {
				return nil, e.errAt(s.Pos, "cannot reassign const %q", s.Name)
			}
			v, err := e.evalExpr(s.Value, locals)
			if err != nil {
				if v, ok, err2 := e.catchReturn(err, retType, locals); ok {
					return v, err2
				}
				return nil, err
			}
			// Re-validate constraint if the binding is constrained.
			if cv, isCon := getConstraint(existing); isCon {
				newVal := copyValue(v)
				if cerr := e.validateConstraint(cv.name, cv.constraint, newVal, locals); cerr != nil {
					return nil, e.wrapErr(s.Pos, cerr)
				}
				locals[s.Name] = &constrainedVal{inner: newVal, constraint: cv.constraint, name: cv.name}
			} else {
				locals[s.Name] = copyValue(v)
			}
			e.trackIfSolid(s.Pos, locals[s.Name])
		case *parser.FieldAssignStmt:
			if err := e.evalFieldAssign(s, locals); err != nil {
				if v, ok, err2 := e.catchReturn(err, retType, locals); ok {
					return v, err2
				}
				return nil, err
			}
		case *parser.AssertStmt:
			if err := e.evalAssert(s, locals); err != nil {
				if v, ok, err2 := e.catchReturn(err, retType, locals); ok {
					return v, err2
				}
				return nil, err
			}
		case *parser.IfStmt:
			if err := e.evalIfStmt(s, locals); err != nil {
				if v, ok, err2 := e.catchReturn(err, retType, locals); ok {
					return v, err2
				}
				return nil, err
			}
		case *parser.ExprStmt:
			if _, err := e.evalExpr(s.Expr, locals); err != nil {
				if v, ok, err2 := e.catchReturn(err, retType, locals); ok {
					return v, err2
				}
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unexpected %s statement in body", stmtKind(stmt))
		}
	}
	return nil, nil
}

const maxCallDepth = 1000
const maxRangeSize = 10_000_000

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
		v := unwrap(args[param.Name])
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
		v := unwrap(args[param.Name])
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
	scope := make(map[string]value, len(e.globals)+len(fv.captured)+len(fv.params))
	for k, v := range e.globals {
		scope[k] = v
	}
	for k, v := range fv.captured {
		scope[k] = v
	}
	// Validate and bind args by name
	paramNames := make(map[string]bool, len(fv.params))
	for _, p := range fv.params {
		paramNames[p.Name] = true
	}
	for name, val := range args {
		if !paramNames[name] {
			return nil, fmt.Errorf("function has no parameter named %q", name)
		}
		scope[name] = val
	}
	return e.execBody(fv.body, fv.retType, scope)
}

// evalFieldAssign evaluates a struct field assignment: receiver.field = value
func (e *evaluator) evalFieldAssign(s *parser.FieldAssignStmt, locals map[string]value) error {
	// Check if receiver is a const binding.
	if ident, ok := s.Receiver.(*parser.IdentExpr); ok {
		if v, exists := locals[ident.Name]; exists && isConst(v) {
			return e.errAt(s.Pos, "cannot mutate field on const %q", ident.Name)
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
