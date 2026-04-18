package evaluator

import (
	"fmt"

	"facet/app/pkg/fctlang/parser"
)

// This file holds the unified statement dispatcher used by every place
// that walks a slice of statements: the top-level function body
// (execBody), interior blocks inside if/else (evalBlock), and the body
// of a for-yield comprehension (evalForBody). Before unification each
// site carried its own switch that handled VarStmt / AssignStmt /
// FieldAssignStmt / AssertStmt / IfStmt / ExprStmt identically and
// disagreed only on ReturnStmt, YieldStmt, and whether a VarStmt was
// block-local or whether an AssignStmt should propagate outward. Every
// fix for the shared cases had to land in three places; drift was the
// default outcome.
//
// The design: dispatchStmt handles the shared cases, and a stmtPolicy
// parameterises the divergent behaviour:
//
//   - onReturn / onYield handle control-flow statements (nil means
//     "this statement is unexpected in this context"), and
//   - onVar / onAssign are post-commit hooks for scope bookkeeping
//     (block-locality tracking, enclosing-scope propagation).
//
// catchReturnSignal is set only on the function-body executor, which
// is the level that converts a *returnSignal propagated from a nested
// block into an actual function return with retType coercion.

// stmtPolicy controls how dispatchStmt handles the statements that vary
// between executors. All fields are optional; nil/empty means "default".
type stmtPolicy struct {
	// context labels the executor for the default-branch error message
	// ("unexpected <stmt> statement in <context>"). Required.
	context string

	// onReturn, if non-nil, handles a ReturnStmt. It returns:
	//   done   — stop iterating (set by function-body executors; block
	//            executors return true too because a return always ends
	//            the statement list one way or another).
	//   retVal — the function-level return value to bubble up, when the
	//            executor is the one producing it.
	//   err    — propagated to the caller (may be a *returnSignal).
	// A nil onReturn means ReturnStmt is unexpected here.
	onReturn func(s *parser.ReturnStmt, locals map[string]value) (done bool, retVal value, err error)

	// onYield, if non-nil, handles a YieldStmt. It returns an error (may
	// be nil) to continue iterating. A nil onYield means YieldStmt is
	// unexpected here.
	onYield func(s *parser.YieldStmt, locals map[string]value) error

	// onVar, if non-nil, is invoked after a VarStmt has bound its value.
	// Used by evalBlock to record block-locality.
	onVar func(name string)

	// onAssign, if non-nil, is invoked after an AssignStmt has committed
	// its new value. Used by evalBlock to propagate the update outward.
	onAssign func(name string, newVal value)

	// catchReturnSignal controls whether a *returnSignal bubbling out of
	// a sub-expression/sub-statement should be treated as a function
	// return at this level. Only the function-body executor sets this.
	catchReturnSignal bool

	// retType is the declared return type of the enclosing function, used
	// for coercion when catchReturnSignal converts a *returnSignal into
	// a function return.
	retType string
}

// dispatchStmt runs one statement under the given policy.
//
// Returned tuple:
//   - done:   true when the caller should stop iterating (e.g. a return
//             was handled).
//   - retVal: the function-level return value, when done is true and the
//             policy produces one.
//   - err:    propagated to the caller as-is (may be a *returnSignal
//             destined for an outer execBody).
//
// Sub-evaluation errors first pass through catchReturnSignal so that a
// *returnSignal that escaped a nested call becomes a function return
// rather than an opaque error. Block-level executors leave
// catchReturnSignal=false, so the signal propagates upward untouched.
func (e *evaluator) dispatchStmt(stmt parser.Stmt, locals map[string]value, p *stmtPolicy) (bool, value, error) {
	if err := e.ctx.Err(); err != nil {
		return false, nil, err
	}
	switch s := stmt.(type) {
	case *parser.ReturnStmt:
		if p.onReturn == nil {
			return false, nil, e.unexpectedStmt(stmt, p.context)
		}
		return p.onReturn(s, locals)
	case *parser.YieldStmt:
		if p.onYield == nil {
			return false, nil, e.unexpectedStmt(stmt, p.context)
		}
		return false, nil, p.onYield(s, locals)
	case *parser.VarStmt:
		if err := e.bindVar(s, locals); err != nil {
			return e.maybeCatchReturn(err, p, locals)
		}
		if p.onVar != nil {
			p.onVar(s.Name)
		}
		return false, nil, nil
	case *parser.AssignStmt:
		newVal, err := e.reassignVar(s, locals)
		if err != nil {
			return e.maybeCatchReturn(err, p, locals)
		}
		if p.onAssign != nil {
			p.onAssign(s.Name, newVal)
		}
		return false, nil, nil
	case *parser.FieldAssignStmt:
		if err := e.evalFieldAssign(s, locals); err != nil {
			return e.maybeCatchReturn(err, p, locals)
		}
		return false, nil, nil
	case *parser.AssertStmt:
		if err := e.evalAssert(s, locals); err != nil {
			return e.maybeCatchReturn(err, p, locals)
		}
		return false, nil, nil
	case *parser.IfStmt:
		if err := e.evalIfStmt(s, locals); err != nil {
			return e.maybeCatchReturn(err, p, locals)
		}
		return false, nil, nil
	case *parser.ExprStmt:
		if _, err := e.evalExpr(s.Expr, locals); err != nil {
			return e.maybeCatchReturn(err, p, locals)
		}
		return false, nil, nil
	default:
		return false, nil, e.unexpectedStmt(stmt, p.context)
	}
}

// unexpectedStmt returns the "unexpected X statement in CONTEXT" error
// used by default branches and by policies that mark ReturnStmt/YieldStmt
// as unexpected at their dispatch site.
func (e *evaluator) unexpectedStmt(stmt parser.Stmt, context string) error {
	return fmt.Errorf("unexpected %s statement in %s", stmtKind(stmt), context)
}

// maybeCatchReturn converts a *returnSignal escaping from sub-evaluation
// into a function-level return when the policy asks for it. Policies
// that leave catchReturnSignal=false get the error back untouched so it
// can propagate to an outer executor.
func (e *evaluator) maybeCatchReturn(err error, p *stmtPolicy, locals map[string]value) (bool, value, error) {
	if !p.catchReturnSignal {
		return false, nil, err
	}
	rs, ok := err.(*returnSignal)
	if !ok {
		return false, nil, err
	}
	v, ferr := e.funcReturn(p.retType, rs.val, locals)
	return true, v, ferr
}

// bindVar evaluates a VarStmt's RHS, runs its constraint (if any),
// wraps the result with constrainedVal/constVal as appropriate, and
// stores it in locals. This is the body of every VarStmt branch that
// used to be duplicated across execBody, evalBlock, and evalForBody.
func (e *evaluator) bindVar(s *parser.VarStmt, locals map[string]value) error {
	v, err := e.evalExpr(s.Value, locals)
	if err != nil {
		return err
	}
	if s.Constraint != nil {
		if cerr := e.validateConstraint(s.Name, s.Constraint, v, locals); cerr != nil {
			return e.wrapErr(s.Pos, cerr)
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
	return nil
}

// reassignVar evaluates an AssignStmt's RHS, checks const-ness,
// re-validates the binding's constraint if it has one, and commits the
// new value to locals. The committed value is returned so executors
// that need to mirror the update into an enclosing scope can do so
// without re-reading locals.
func (e *evaluator) reassignVar(s *parser.AssignStmt, locals map[string]value) (value, error) {
	existing, ok := locals[s.Name]
	if !ok {
		return nil, e.errAt(s.Pos, "cannot assign to undefined variable %q", s.Name)
	}
	if isConst(existing) {
		return nil, e.errAt(s.Pos, "cannot reassign const %q", s.Name)
	}
	v, err := e.evalExpr(s.Value, locals)
	if err != nil {
		return nil, err
	}
	var newVal value
	if cv, isCon := getConstraint(existing); isCon {
		newVal = copyValue(v)
		if cerr := e.validateConstraint(cv.name, cv.constraint, newVal, locals); cerr != nil {
			return nil, e.wrapErr(s.Pos, cerr)
		}
		newVal = &constrainedVal{inner: newVal, constraint: cv.constraint, name: cv.name}
	} else {
		newVal = copyValue(v)
	}
	locals[s.Name] = newVal
	e.trackIfSolid(s.Pos, newVal)
	return newVal, nil
}
