package evaluator

import (
	"facet/app/pkg/fctlang/parser"
	"fmt"
)

// returnSignal is a sentinel error used to propagate a function-level return
// through nested block/if/fold expressions. When `return` is encountered inside
// a block or if-expression, it wraps the value in returnSignal and propagates
// upward until caught by execBody (the function-level executor).
type returnSignal struct {
	val value
}

func (r *returnSignal) Error() string { return "return" }

// evalBlock executes a block of statements. A `return` inside a block produces
// a returnSignal error that propagates to the enclosing function (C/Go semantics).
//
// New variables declared with `var` are block-local and don't leak out.
// Assignments to existing enclosing variables propagate to the enclosing scope.
func (e *evaluator) evalBlock(body []parser.Stmt, enclosing map[string]value) (value, error) {
	// Copy enclosing scope so block vars don't leak out.
	locals := make(map[string]value, len(enclosing))
	for k, v := range enclosing {
		locals[k] = v
	}
	// Track which vars are declared inside this block (don't propagate).
	blockLocal := make(map[string]bool)

	for _, stmt := range body {
		if err := e.ctx.Err(); err != nil {
			return nil, err
		}
		switch s := stmt.(type) {
		case *parser.ReturnStmt:
			v, err := e.evalExpr(s.Value, locals)
			if err != nil {
				return nil, err
			}
			return nil, &returnSignal{val: v}
		case *parser.YieldStmt:
			if e.foldAcc != nil {
				// Inside fold: yield sets the accumulator
				if s.Value == nil {
					continue
				}
				v, err := e.evalExpr(s.Value, locals)
				if err != nil {
					return nil, err
				}
				*e.foldAcc = v
			} else if e.yieldTarget != nil {
				// Inside for-yield: yield appends to results
				if s.Value == nil {
					continue
				}
				v, err := e.evalExpr(s.Value, locals)
				if err != nil {
					return nil, err
				}
				*e.yieldTarget = append(*e.yieldTarget, v)
			} else {
				return nil, e.errAt(s.Pos, "yield outside of for-yield or fold")
			}
		case *parser.VarStmt:
			v, err := e.evalExpr(s.Value, locals)
			if err != nil {
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
			blockLocal[s.Name] = true
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
				return nil, err
			}
			// Re-validate constraint if the binding is constrained.
			var newVal value
			if con, isCon := getConstraint(existing); isCon {
				newVal = copyValue(v)
				if cerr := e.validateConstraint(con.name, con.constraint, newVal, locals); cerr != nil {
					return nil, e.wrapErr(s.Pos, cerr)
				}
				newVal = &constrainedVal{inner: newVal, constraint: con.constraint, name: con.name}
			} else {
				newVal = copyValue(v)
			}
			locals[s.Name] = newVal
			e.trackIfSolid(s.Pos, newVal)
			// Propagate to enclosing scope if not a block-local var.
			if !blockLocal[s.Name] {
				enclosing[s.Name] = newVal
			}
		case *parser.FieldAssignStmt:
			if ident, ok := s.Receiver.(*parser.IdentExpr); ok {
				if _, ok := locals[ident.Name].(*constVal); ok {
					return nil, e.errAt(s.Pos, "cannot mutate field on const %q", ident.Name)
				}
			}
			if err := e.evalFieldAssign(s, locals); err != nil {
				return nil, err
			}
		case *parser.IfStmt:
			if err := e.evalIfStmt(s, locals); err != nil {
				return nil, err
			}
		case *parser.AssertStmt:
			if err := e.evalAssert(s, locals); err != nil {
				return nil, err
			}
		case *parser.ExprStmt:
			if _, err := e.evalExpr(s.Expr, locals); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unexpected %s statement in block", stmtKind(stmt))
		}
	}
	return nil, nil
}

func (e *evaluator) evalForYield(ex *parser.ForYieldExpr, locals map[string]value) (value, error) {
	var results []value
	prev := e.yieldTarget
	e.yieldTarget = &results
	err := e.evalForClauses(ex.Clauses, 0, ex.Body, locals, &results)
	e.yieldTarget = prev
	if err != nil {
		return nil, err
	}
	return array{elems: results, elemType: inferElemType(results)}, nil
}

// evalForClauses recursively iterates over for-yield clauses (cartesian product).
// When all clauses are bound, it executes the body and collects yielded values.
func (e *evaluator) evalForClauses(clauses []*parser.ForClause, idx int, body []parser.Stmt, locals map[string]value, results *[]value) error {
	if idx >= len(clauses) {
		// All clauses bound — execute body
		return e.evalForBody(body, locals, results)
	}

	clause := clauses[idx]
	iterVal, err := e.evalExpr(clause.Iter, locals)
	if err != nil {
		return err
	}
	arr, ok := iterVal.(array)
	if !ok {
		return e.errAt(clause.Pos, "for-yield: expected Array to iterate over, got %s", typeName(iterVal))
	}

	for i, elem := range arr.elems {
		if err := e.ctx.Err(); err != nil {
			return err
		}
		iterLocals := make(map[string]value, len(locals)+2)
		for k, v := range locals {
			iterLocals[k] = v
		}
		if clause.Index != "" {
			iterLocals[clause.Index] = float64(i)
		}
		iterLocals[clause.Var] = elem

		if err := e.evalForClauses(clauses, idx+1, body, iterLocals, results); err != nil {
			return err
		}
	}
	return nil
}

// evalForBody executes the body of a for-yield loop, collecting yielded values.
func (e *evaluator) evalForBody(body []parser.Stmt, locals map[string]value, results *[]value) error {
	for _, stmt := range body {
		if err := e.ctx.Err(); err != nil {
			return err
		}
		switch s := stmt.(type) {
		case *parser.YieldStmt:
			// Bare yield; — skip this iteration
			if s.Value == nil {
				continue
			}
			v, err := e.evalExpr(s.Value, locals)
			if err != nil {
				return err
			}
			// nil means the expression had no value (e.g. if-without-else where
			// the branch contained explicit yields). Skip it.
			if v != nil {
				*results = append(*results, v)
			}
		case *parser.VarStmt:
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
		case *parser.AssignStmt:
			existing, ok := locals[s.Name]
			if !ok {
				return e.errAt(s.Pos, "cannot assign to undefined variable %q", s.Name)
			}
			if isConst(existing) {
				return e.errAt(s.Pos, "cannot reassign const %q", s.Name)
			}
			v, err := e.evalExpr(s.Value, locals)
			if err != nil {
				return err
			}
			if con, isCon := getConstraint(existing); isCon {
				newVal := copyValue(v)
				if cerr := e.validateConstraint(con.name, con.constraint, newVal, locals); cerr != nil {
					return e.wrapErr(s.Pos, cerr)
				}
				locals[s.Name] = &constrainedVal{inner: newVal, constraint: con.constraint, name: con.name}
			} else {
				locals[s.Name] = copyValue(v)
			}
			e.trackIfSolid(s.Pos, locals[s.Name])
		case *parser.FieldAssignStmt:
			if ident, ok := s.Receiver.(*parser.IdentExpr); ok {
				if _, ok := locals[ident.Name].(*constVal); ok {
					return e.errAt(s.Pos, "cannot mutate field on const %q", ident.Name)
				}
			}
			if err := e.evalFieldAssign(s, locals); err != nil {
				return err
			}
		case *parser.IfStmt:
			if err := e.evalIfStmt(s, locals); err != nil {
				return err
			}
		case *parser.AssertStmt:
			if err := e.evalAssert(s, locals); err != nil {
				return err
			}
		case *parser.ExprStmt:
			if _, err := e.evalExpr(s.Expr, locals); err != nil {
				return err
			}
		case *parser.ReturnStmt:
			return fmt.Errorf("use 'yield' instead of 'return' inside for-yield loops")
		default:
			return fmt.Errorf("unexpected %s statement in for-yield body", stmtKind(stmt))
		}
	}
	return nil
}

func (e *evaluator) evalAssert(s *parser.AssertStmt, locals map[string]value) error {
	// "assert EXPR where CONSTRAINT" form
	if s.Constraint != nil {
		val, err := e.evalExpr(s.Value, locals)
		if err != nil {
			return err
		}
		if err := e.validateConstraint("assert", s.Constraint, val, locals); err != nil {
			return e.wrapErr(s.Pos, err)
		}
		return nil
	}

	// "assert COND [, MSG]" form
	cv, err := e.evalExpr(s.Cond, locals)
	if err != nil {
		return err
	}
	b, ok := cv.(bool)
	if !ok {
		return e.errAt(s.Pos, "assert condition must be a Bool, got %s", typeName(cv))
	}
	if !b {
		if s.Message != nil {
			mv, err := e.evalExpr(s.Message, locals)
			if err != nil {
				return err
			}
			if msg, ok := mv.(string); ok {
				return e.errAt(s.Pos, "assertion failed: %s", msg)
			}
			return e.errAt(s.Pos, "assertion failed: %v", mv)
		}
		return e.errAt(s.Pos, "assertion failed")
	}
	return nil
}

func (e *evaluator) evalFold(ex *parser.FoldExpr, locals map[string]value) (value, error) {
	iterVal, err := e.evalExpr(ex.Iter, locals)
	if err != nil {
		return nil, err
	}
	arr, ok := iterVal.(array)
	if !ok {
		return nil, e.errAt(ex.Pos, "fold: expected Array to iterate over, got %s", typeName(iterVal))
	}
	if len(arr.elems) == 0 {
		return nil, e.errAt(ex.Pos, "fold: cannot fold an empty array")
	}

	// First element is the initial accumulator
	acc := unwrap(arr.elems[0])

	// Save and set foldAcc so yield writes to the accumulator.
	prevFoldAcc := e.foldAcc
	e.foldAcc = &acc
	prevYield := e.yieldTarget
	e.yieldTarget = nil // prevent for-yield yield from firing inside fold

	for _, elem := range arr.elems[1:] {
		if err := e.ctx.Err(); err != nil {
			e.foldAcc = prevFoldAcc
			e.yieldTarget = prevYield
			return nil, err
		}
		// Create iteration scope with named acc and elem vars
		iterLocals := make(map[string]value, len(locals)+2)
		for k, v := range locals {
			iterLocals[k] = v
		}
		iterLocals[ex.AccVar] = acc
		iterLocals[ex.ElemVar] = elem

		_, err := e.evalBlock(ex.Body, iterLocals)
		if err != nil {
			e.foldAcc = prevFoldAcc
			e.yieldTarget = prevYield
			// Propagate returnSignal — return inside fold exits the function.
			return nil, err
		}
	}

	e.foldAcc = prevFoldAcc
	e.yieldTarget = prevYield
	return acc, nil
}

func (e *evaluator) evalIfStmt(s *parser.IfStmt, locals map[string]value) error {
	cv, err := e.evalExpr(s.Cond, locals)
	if err != nil {
		return err
	}
	cb, ok := cv.(bool)
	if !ok {
		return e.errAt(s.Pos, "if condition must be a Bool, got %s", typeName(cv))
	}
	if cb {
		_, err := e.evalBlock(s.Then, locals)
		return err
	}
	for _, eif := range s.ElseIfs {
		cv, err := e.evalExpr(eif.Cond, locals)
		if err != nil {
			return err
		}
		cb, ok := cv.(bool)
		if !ok {
			return e.errAt(eif.Pos, "else-if condition must be a Bool, got %s", typeName(cv))
		}
		if cb {
			_, err := e.evalBlock(eif.Body, locals)
			return err
		}
	}
	if s.Else != nil {
		_, err := e.evalBlock(s.Else, locals)
		return err
	}
	return nil
}
