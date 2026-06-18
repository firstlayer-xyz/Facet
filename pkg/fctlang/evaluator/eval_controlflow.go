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
// Block scoping is shadow-restore over the enclosing scope: statements execute
// directly against `locals`, so an assignment to an enclosing variable is
// visible at any nesting depth (an if inside an if inside a block all write the
// same map). A `var` declared in the block records what it shadowed and is
// rolled back on exit, so block-locals never leak and a shadowed enclosing
// binding is restored.
func (e *evaluator) evalBlock(body []parser.Stmt, locals map[string]value) (value, error) {
	type shadow struct {
		val value
		had bool
	}
	declared := make(map[string]shadow)
	defer func() {
		for name, sh := range declared {
			if sh.had {
				locals[name] = sh.val
			} else {
				delete(locals, name)
			}
		}
	}()

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
			// Called before bindVar commits, so the shadowed binding (or its
			// absence) is still observable. Only the first declaration of a name
			// records — rollback restores the pre-block state.
			if _, seen := declared[name]; !seen {
				old, had := locals[name]
				declared[name] = shadow{val: old, had: had}
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
		*e.foldAcc = copyValue(v)
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
	// The first clause's iterable is evaluated exactly once, here: it decides
	// between the Optional path (a single-clause loop over an Optional is
	// Map/Filter on T?) and the array path, which receives the value rather
	// than re-evaluating the expression (it may have side effects like solid
	// tracking).
	first, err := e.evalExpr(ex.Clauses[0].Iter, locals)
	if err != nil {
		return nil, err
	}
	if len(ex.Clauses) == 1 {
		if opt, ok := first.(*optionalVal); ok {
			return e.evalForYieldOptional(ex, opt, locals)
		}
	}
	return e.evalForYieldArray(ex, locals, first)
}

func (e *evaluator) evalForYieldArray(ex *parser.ForYieldExpr, locals map[string]value, first value) (value, error) {
	var results []value
	prevYield := e.yieldTarget
	e.yieldTarget = &results
	// A yield in this loop's body must collect here, not into an enclosing
	// fold's accumulator — clear foldAcc for the loop's extent (mirroring
	// evalFold, which clears yieldTarget).
	prevFold := e.foldAcc
	e.foldAcc = nil
	// defer the restores so a panic inside the body (CGo invariant, ctx cancel
	// via runtime panic, etc.) doesn't leave the global yieldTarget pointing
	// at this stack-local slice — a later, unrelated yield would otherwise
	// write to a freed slot.
	defer func() {
		e.yieldTarget = prevYield
		e.foldAcc = prevFold
	}()
	if err := e.evalForClauses(ex.Clauses, 0, first, ex.Body, locals, &results); err != nil {
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
	iterLocals[clause.Var] = copyValue(opt.inner)
	if clause.Index != "" {
		iterLocals[clause.Index] = float64(0)
	}
	var results []value
	prev := e.yieldTarget
	e.yieldTarget = &results
	// A yield in this loop's body must collect here, not into an enclosing fold's
	// accumulator — clear foldAcc for the loop's extent (mirroring evalForYieldArray).
	// blockYield checks foldAcc before yieldTarget, so leaving an enclosing fold's
	// foldAcc live would send these yields to the wrong place and leave results
	// empty, silently turning the result into None.
	prevFold := e.foldAcc
	e.foldAcc = nil
	defer func() {
		e.yieldTarget = prev
		e.foldAcc = prevFold
	}()
	if err := e.evalForBody(ex.Body, iterLocals, &results); err != nil {
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
// first is the already-evaluated iterable for clause 0 (evalForYield evaluates it
// once to choose the Optional/array path); deeper clauses evaluate their own.
func (e *evaluator) evalForClauses(clauses []*parser.ForClause, idx int, first value, body []parser.Stmt, locals map[string]value, results *[]value) error {
	if idx >= len(clauses) {
		// All clauses bound — execute body
		return e.evalForBody(body, locals, results)
	}

	clause := clauses[idx]
	iterVal := first
	if idx > 0 {
		var err error
		iterVal, err = e.evalExpr(clause.Iter, locals)
		if err != nil {
			return err
		}
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
		// Value semantics: the loop variable binds a COPY of the element, so a
		// field assignment on it cannot rewrite the source array.
		iterLocals[clause.Var] = copyValue(elem)

		if err := e.evalForClauses(clauses, idx+1, nil, body, iterLocals, results); err != nil {
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
			// Defence in depth: the parser rejects `return` inside a for-
			// yield body before we get here. If we ever reach this code
			// path, it's an evaluator bug rather than user input — surface
			// the same hint the parser uses so the error is actionable.
			return false, nil, fmt.Errorf("for-yield can only contribute via 'yield'; to exit the enclosing function, extract the loop into its own function and return from there")
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
				// Each per-clause range is capped (maxRangeSize), but a nested
				// for-yield is a cartesian product, so the *accumulated* result
				// count is bounded here to keep an adversarial product (e.g.
				// `for x [0:9999] for y [0:9999]`) from exhausting memory.
				if len(*results) >= maxRangeSize {
					return e.errAt(s.Pos, "for-yield produced too many elements (limit %d)", maxRangeSize)
				}
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
	acc := copyValue(arr.elems[0])

	// Save and set foldAcc so yield writes to the accumulator. defer the
	// restores so a panic inside the body (CGo invariant, runtime panic)
	// doesn't leave the globals pointing at this stack-local accumulator.
	prevFoldAcc := e.foldAcc
	e.foldAcc = &acc
	prevYield := e.yieldTarget
	e.yieldTarget = nil // prevent for-yield yield from firing inside fold
	defer func() {
		e.foldAcc = prevFoldAcc
		e.yieldTarget = prevYield
	}()

	for _, elem := range arr.elems[1:] {
		if err := e.ctx.Err(); err != nil {
			return nil, err
		}
		// Create iteration scope with named acc and elem vars
		iterLocals := make(map[string]value, len(locals)+2)
		for k, v := range locals {
			iterLocals[k] = v
		}
		iterLocals[ex.AccVar] = acc
		iterLocals[ex.ElemVar] = copyValue(elem)

		// Propagate returnSignal — return inside fold exits the function.
		if _, err := e.evalBlock(ex.Body, iterLocals); err != nil {
			return nil, err
		}
	}

	return acc, nil
}

func (e *evaluator) evalIfStmt(s *parser.IfStmt, locals map[string]value) error {
	cv, err := e.evalExpr(s.Cond, locals)
	if err != nil {
		return err
	}
	// `if var NAME = expr { ... }` binds NAME to the inner value of an
	// Optional when present. NAME shadows any outer binding for the
	// duration of the body and is restored on exit, so the binding does
	// not leak. Other assignments inside the body still propagate to the
	// enclosing scope because we mutate `locals` in place rather than
	// passing a copy to evalBlock.
	if s.BindVar != "" {
		opt, ok := cv.(*optionalVal)
		if !ok {
			return e.errAt(s.Pos, "if var %s = expr: expr must be Optional, got %s", s.BindVar, typeName(cv))
		}
		if opt.present {
			shadowed, hadShadowed := locals[s.BindVar]
			locals[s.BindVar] = copyValue(opt.inner)
			_, err := e.evalBlock(s.Then, locals)
			if hadShadowed {
				locals[s.BindVar] = shadowed
			} else {
				delete(locals, s.BindVar)
			}
			return err
		}
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
