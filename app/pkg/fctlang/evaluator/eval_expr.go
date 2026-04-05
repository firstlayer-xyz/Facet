package evaluator

import (
	"facet/app/pkg/fctlang/parser"
	"facet/app/pkg/manifold"
	"fmt"
	"math"
)

func (e *evaluator) evalExpr(expr parser.Expr, locals map[string]value) (value, error) {
	switch ex := expr.(type) {
	case *parser.NumberLit:
		return ex.Value, nil

	case *parser.BoolLit:
		return ex.Value, nil

	case *parser.IdentExpr:
		v, ok := locals[ex.Name]
		if !ok {
			return nil, e.errAt(ex.Pos, "undefined variable %q", ex.Name)
		}
		val := unwrap(v)
		if sf, ok := val.(*manifold.Solid); ok {
			e.trackSolid(ex.Pos, sf)
		}
		return val, nil

	case *parser.NamedArg:
		// NamedArg should be handled at the call site (evalCall/evalMethodCall),
		// not as a standalone expression. If we reach here, evaluate just the value.
		return e.evalExpr(ex.Value, locals)

	case *parser.UnaryExpr:
		return e.evalUnary(ex, locals)

	case *parser.BinaryExpr:
		return e.evalBinary(ex, locals)

	case *parser.CallExpr:
		return e.evalCall(ex, locals)

	case *parser.BuiltinCallExpr:
		return e.evalBuiltinCall(ex, locals)

	case *parser.MethodCallExpr:
		return e.evalMethodCall(ex, locals)

	case *parser.ArrayLitExpr:
		elems := make([]value, len(ex.Elems))
		for i, elem := range ex.Elems {
			v, err := e.evalExpr(elem, locals)
			if err != nil {
				return nil, err
			}
			elems[i] = v
		}
		// Typed array constructor: coerce each element to TypeName
		if ex.TypeName != "" {
			for i, v := range elems {
				// Sub-arrays (bare [...] inside TypeName[...]) inherit the type
				// prefix and are allowed as-is — they form nested arrays
				// e.g. Number[[1,2],[3,4]] → [][]Number.
				if _, isArr := v.(array); isArr {
					continue
				}
				elems[i] = e.coerceToType(ex.TypeName, v, locals)
				if !checkType(ex.TypeName, elems[i]) {
					return nil, e.errAt(ex.Pos, "typed array %s[]: element %d must be %s, got %s",
						ex.TypeName, i, ex.TypeName, typeName(elems[i]))
				}
			}
		}
		et := ex.TypeName
		if et == "" {
			et = inferElemType(elems)
		}
		return array{elems: elems, elemType: et}, nil

	case *parser.RangeExpr:
		sv, err := e.evalExpr(ex.Start, locals)
		if err != nil {
			return nil, err
		}
		ev, err := e.evalExpr(ex.End, locals)
		if err != nil {
			return nil, err
		}
		startF, ok := asNumber(sv)
		if !ok {
			return nil, e.errAt(ex.Pos, "range start must be a Number, got %s", typeName(sv))
		}
		endF, ok := asNumber(ev)
		if !ok {
			return nil, e.errAt(ex.Pos, "range end must be a Number, got %s", typeName(ev))
		}
		if math.IsNaN(startF) || math.IsInf(startF, 0) {
			return nil, e.errAt(ex.Pos, "range start cannot be NaN or infinity")
		}
		if math.IsNaN(endF) || math.IsInf(endF, 0) {
			return nil, e.errAt(ex.Pos, "range end cannot be NaN or infinity")
		}
		// Determine step
		stepF := 1.0
		if ex.Step != nil {
			stepV, err := e.evalExpr(ex.Step, locals)
			if err != nil {
				return nil, err
			}
			stepF, ok = asNumber(stepV)
			if !ok {
				return nil, e.errAt(ex.Pos, "range step must be a Number, got %s", typeName(stepV))
			}
			if stepF == 0 {
				return nil, e.errAt(ex.Pos, "range step cannot be zero")
			}
		} else {
			// Auto-infer direction
			if startF > endF {
				stepF = -1.0
			}
		}
		// Validate step direction
		if stepF > 0 && startF > endF {
			return nil, e.errAt(ex.Pos, "range step is positive but start (%v) > end (%v)", startF, endF)
		}
		if stepF < 0 && startF < endF {
			return nil, e.errAt(ex.Pos, "range step is negative but start (%v) < end (%v)", startF, endF)
		}
		// Estimate range size and reject if too large
		estimatedSize := int(math.Abs((endF-startF)/stepF)) + 1
		if estimatedSize > maxRangeSize {
			return nil, e.errAt(ex.Pos, "range would produce %d elements (limit %d)", estimatedSize, maxRangeSize)
		}
		// Use counter-based iteration to avoid floating-point accumulation drift.
		// Instead of i += step (which drifts), compute i = start + count * step.
		var elems []value
		for count := 0; ; count++ {
			if err := e.ctx.Err(); err != nil {
				return nil, err
			}
			i := startF + float64(count)*stepF
			if stepF > 0 {
				if ex.Exclusive {
					if i >= endF {
						break
					}
				} else {
					if i > endF {
						break
					}
				}
			} else {
				if ex.Exclusive {
					if i <= endF {
						break
					}
				} else {
					if i < endF {
						break
					}
				}
			}
			elems = append(elems, i)
		}
		return array{elems: elems, elemType: "Number"}, nil

	case *parser.ForYieldExpr:
		return e.evalForYield(ex, locals)

	case *parser.FoldExpr:
		return e.evalFold(ex, locals)

	case *parser.StringLit:
		return ex.Value, nil

	case *parser.LibExpr:
		return e.evalLibExpr(ex)

	case *parser.StructLitExpr:
		return e.evalStructLit(ex, locals)

	case *parser.IndexExpr:
		recv, err := e.evalExpr(ex.Receiver, locals)
		if err != nil {
			return nil, err
		}
		arr, ok := unwrap(recv).(array)
		if !ok {
			return nil, e.errAt(ex.Pos, "cannot index %s (expected Array)", typeName(recv))
		}
		idx, err := e.evalExpr(ex.Index, locals)
		if err != nil {
			return nil, err
		}
		n, ok := unwrap(idx).(float64)
		if !ok {
			return nil, e.errAt(ex.Pos, "index must be a Number, got %s", typeName(idx))
		}
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return nil, e.errAt(ex.Pos, "index must be a finite number")
		}
		if n != math.Floor(n) {
			return nil, e.errAt(ex.Pos, "array index must be an integer, got %v", n)
		}
		i := int(n)
		// Negative indexing: -1 = last, -2 = second to last, etc.
		if i < 0 {
			i += len(arr.elems)
		}
		if i < 0 || i >= len(arr.elems) {
			return nil, e.errAt(ex.Pos, "index %d out of range (length %d)", int(n), len(arr.elems))
		}
		return arr.elems[i], nil

	case *parser.SliceExpr:
		recv, err := e.evalExpr(ex.Receiver, locals)
		if err != nil {
			return nil, err
		}
		arr, ok := unwrap(recv).(array)
		if !ok {
			return nil, e.errAt(ex.Pos, "cannot slice %s (expected Array)", typeName(recv))
		}
		length := len(arr.elems)
		start := 0
		end := length
		if ex.Start != nil {
			sv, err := e.evalExpr(ex.Start, locals)
			if err != nil {
				return nil, err
			}
			sn, ok := unwrap(sv).(float64)
			if !ok {
				return nil, e.errAt(ex.Pos, "slice start must be a Number, got %s", typeName(sv))
			}
			start = int(sn)
			if start < 0 {
				start += length
			}
		}
		if ex.End != nil {
			ev, err := e.evalExpr(ex.End, locals)
			if err != nil {
				return nil, err
			}
			en, ok := unwrap(ev).(float64)
			if !ok {
				return nil, e.errAt(ex.Pos, "slice end must be a Number, got %s", typeName(ev))
			}
			end = int(en)
			if end < 0 {
				end += length
			}
		}
		// Clamp to bounds
		if start < 0 {
			start = 0
		}
		if end > length {
			end = length
		}
		if start >= end {
			return array{elems: nil, elemType: arr.elemType}, nil
		}
		sliced := make([]value, end-start)
		copy(sliced, arr.elems[start:end])
		return array{elems: sliced, elemType: arr.elemType}, nil

	case *parser.FieldAccessExpr:
		return e.evalFieldAccess(ex, locals)

	case *parser.UnitExpr:
		v, err := e.evalExpr(ex.Expr, locals)
		if err != nil {
			return nil, err
		}
		n, ok := unwrap(v).(float64)
		if !ok {
			return nil, e.errAt(ex.Pos, "cannot apply unit %q to %s (expected Number)", ex.Unit, typeName(v))
		}
		if ex.IsAngle {
			return angle{deg: n * ex.Factor}, nil
		}
		return length{mm: n * ex.Factor}, nil

	case *parser.LambdaExpr:
		captured := make(map[string]value, len(locals))
		for k, v := range locals {
			captured[k] = copyValue(v)
		}
		return &functionVal{
			params:   ex.Params,
			retType:  ex.ReturnType,
			body:     ex.Body,
			captured: captured,
		}, nil

	default:
		return nil, fmt.Errorf("unsupported expression type %T", expr)
	}
}
