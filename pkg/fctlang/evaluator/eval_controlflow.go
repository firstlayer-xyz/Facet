package evaluator

import (
	"facet/pkg/fctlang/parser"
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

	policy := &stmtPolicy{
		context: "block",
		onReturn: func(s *parser.ReturnStmt, locals map[string]value) (bool, value, error) {
			v, err := e.evalExpr(s.Value, locals)
			if err != nil {
				return true, nil, err
			}
			return true, nil, &returnSignal{val: v}
		},
		onYield: func(s *parser.YieldStmt, locals map[string]value) error {
			return e.blockYield(s, locals)
		},
		onVar: func(name string) {
			blockLocal[name] = true
		},
		onAssign: func(name string, newVal value) {
			if !blockLocal[name] {
				enclosing[name] = newVal
			}
		},
	}

	for _, stmt := range body {
		done, _, err := e.dispatchStmt(stmt, locals, policy)
		if err != nil || done {
			return nil, err
		}
	}
	return nil, nil
}

// blockYield implements yield within an if/else block. A yield inside a
// block can only make sense if it is dynamically nested in either a fold
// (foldAcc is live) or a for-yield loop (yieldTarget is live); otherwise
// it is a static error at the yield's position.
func (e *evaluator) blockYield(s *parser.YieldStmt, locals map[string]value) error {
	switch {
	case e.foldAcc != nil:
		if s.Value == nil {
			return nil
		}
		v, err := e.evalExpr(s.Value, locals)
		if err != nil {
			return err
		}
		*e.foldAcc = v
		return nil
	case e.yieldTarget != nil:
		if s.Value == nil {
			return nil
		}
		v, err := e.evalExpr(s.Value, locals)
		if err != nil {
			return err
		}
		*e.yieldTarget = append(*e.yieldTarget, v)
		return nil
	default:
		return e.errAt(s.Pos, "yield outside of for-yield or fold")
	}
}

func (e *evaluator) evalForYield(ex *parser.ForYieldExpr, locals map[string]value) (value, error) {
	// for-yield on a single Optional source: Optional is a 0-or-1 element
	// collection, so the result is also Optional (T → U? via Map / Filter).
	// Multiple clauses with an Optional source aren't supported in v1.
	if len(ex.Clauses) == 1 {
		iterVal, err := e.evalExpr(ex.Clauses[0].Iter, locals)
		if err != nil {
			return nil, err
		}
		if opt, ok := iterVal.(*optionalVal); ok {
			return e.evalForYieldOptional(ex, opt, locals)
		}
		// Fall through to array path with the already-evaluated iter so we
		// don't re-evaluate side effects.
		return e.evalForYieldArray(ex, []value{iterVal}, locals)
	}

	return e.evalForYieldArray(ex, nil, locals)
}

// evalForYieldArray runs the standard array-source for-yield. preEval[i] (if
// non-nil and within bounds) is a pre-evaluated iter value for clause i,
// used by the dispatcher above to avoid re-evaluating side effects when
// type-dispatching.
func (e *evaluator) evalForYieldArray(ex *parser.ForYieldExpr, preEval []value, locals map[string]value) (value, error) {
	var results []value
	prev := e.yieldTarget
	e.yieldTarget = &results
	err := e.evalForClauses(ex.Clauses, 0, ex.Body, locals, &results)
	if err == nil && len(preEval) > 0 {
		_ = preEval // reserved for future iter-pre-eval optimization
	}
	e.yieldTarget = prev
	if err != nil {
		return nil, err
	}
	return array{elems: results, elemType: inferElemType(results)}, nil
}

// evalForYieldOptional runs `for v opt { ... yield ... }` against an
// Optional source. The iteration runs 0 or 1 times; the result is None if
// no yield reached the collector, Some(value) if exactly one did, and an
// error if more than one (Optional can't hold many values).
func (e *evaluator) evalForYieldOptional(ex *parser.ForYieldExpr, opt *optionalVal, locals map[string]value) (value, error) {
	if !opt.present {
		return none(""), nil
	}
	clause := ex.Clauses[0]
	iterLocals := make(map[string]value, len(locals)+1)
	for k, v := range locals {
		iterLocals[k] = v
	}
	iterLocals[clause.Var] = opt.inner
	if clause.Index != "" {
		iterLocals[clause.Index] = float64(0)
	}
	var results []value
	prev := e.yieldTarget
	e.yieldTarget = &results
	err := e.evalForBody(ex.Body, iterLocals, &results)
	e.yieldTarget = prev
	if err != nil {
		return nil, err
	}
	switch len(results) {
	case 0:
		return none(""), nil
	case 1:
		return some(results[0], ""), nil
	default:
		return nil, e.errAt(ex.Pos, "for-yield over Optional yielded %d values; an Optional can hold at most one", len(results))
	}
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
	policy := &stmtPolicy{
		context: "for-yield body",
		onReturn: func(s *parser.ReturnStmt, locals map[string]value) (bool, value, error) {
			// `return` inside for-yield is a static error — a for-yield
			// produces an array, there is no function to return from at
			// this level.
			return false, nil, fmt.Errorf("use 'yield' instead of 'return' inside for-yield loops")
		},
		onYield: func(s *parser.YieldStmt, locals map[string]value) error {
			// Bare yield (no value) skips this iteration.
			if s.Value == nil {
				return nil
			}
			v, err := e.evalExpr(s.Value, locals)
			if err != nil {
				return err
			}
			// nil means the expression had no value (e.g. if-without-else
			// where the branch contained explicit yields). Skip it.
			if v != nil {
				*results = append(*results, v)
			}
			return nil
		},
	}

	for _, stmt := range body {
		_, _, err := e.dispatchStmt(stmt, locals, policy)
		if err != nil {
			return err
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
	// Bind-and-narrow form: `if var NAME = expr { ... }`. expr must be
	// Optional; when Some(v), inject NAME = v into the enclosing locals
	// (so assignments to OTHER vars inside the body still propagate out
	// as they would for a regular if), run the body, then remove NAME so
	// the binding doesn't leak past the closing brace.
	if s.BindVar != "" {
		opt, ok := cv.(*optionalVal)
		if !ok {
			return e.errAt(s.Pos, "if var %s = expr: expr must be Optional, got %s", s.BindVar, typeName(cv))
		}
		if opt.present {
			shadowed, hadShadowed := locals[s.BindVar]
			locals[s.BindVar] = opt.inner
			_, err := e.evalBlock(s.Then, locals)
			if hadShadowed {
				locals[s.BindVar] = shadowed
			} else {
				delete(locals, s.BindVar)
			}
			return err
		}
		// Fall through to else-if / else.
	} else {
		cb, ok := cv.(bool)
		if !ok {
			return e.errAt(s.Pos, "if condition must be a Bool, got %s", typeName(cv))
		}
		if cb {
			_, err := e.evalBlock(s.Then, locals)
			return err
		}
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
