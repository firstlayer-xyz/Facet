package evaluator

import (
	"fmt"
	"math"
	"strconv"
	"time"
)

func mathMinMax(name string, args []value, isMax bool) (value, error) {
	// Coerce Number → Length/Angle so mixed calls like Min(5 mm, 0) work.
	coerceNumericArgs(args)
	switch a := args[0].(type) {
	case length:
		b, ok := args[1].(length)
		if !ok {
			return nil, fmt.Errorf("%s() arguments must be the same type, got %s and %s", name, typeName(args[0]), typeName(args[1]))
		}
		if isMax {
			if a.mm > b.mm {
				return a, nil
			}
			return b, nil
		}
		if a.mm < b.mm {
			return a, nil
		}
		return b, nil
	case float64:
		bn, ok := args[1].(float64)
		if !ok {
			return nil, fmt.Errorf("%s() arguments must be the same type, got %s and %s", name, typeName(args[0]), typeName(args[1]))
		}
		if isMax {
			if a > bn {
				return a, nil
			}
			return bn, nil
		}
		if a < bn {
			return a, nil
		}
		return bn, nil
	case angle:
		b, ok := args[1].(angle)
		if !ok {
			return nil, fmt.Errorf("%s() arguments must be the same type, got %s and %s", name, typeName(args[0]), typeName(args[1]))
		}
		if isMax {
			if a.deg > b.deg {
				return a, nil
			}
			return b, nil
		}
		if a.deg < b.deg {
			return a, nil
		}
		return b, nil
	default:
		return nil, fmt.Errorf("%s() arguments must be numeric, got %s", name, typeName(args[0]))
	}
}

func mathAbs(v value) (value, error) {
	switch a := v.(type) {
	case length:
		return length{mm: math.Abs(a.mm)}, nil
	case float64:
		return math.Abs(a), nil
	case angle:
		return angle{deg: math.Abs(a.deg)}, nil
	default:
		return nil, fmt.Errorf("Abs() argument must be numeric, got %s", typeName(v))
	}
}

// ---------------------------------------------------------------------------
// Trig builtins
// ---------------------------------------------------------------------------

func builtinSin(_ *evaluator, args []value) (value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("_sin() expects 1 argument, got %d", len(args))
	}
	deg, err := requireAngle("_sin", 1, args[0])
	if err != nil {
		return nil, err
	}
	return math.Sin(deg * math.Pi / 180), nil
}

func builtinCos(_ *evaluator, args []value) (value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("_cos() expects 1 argument, got %d", len(args))
	}
	deg, err := requireAngle("_cos", 1, args[0])
	if err != nil {
		return nil, err
	}
	return math.Cos(deg * math.Pi / 180), nil
}

func builtinTan(_ *evaluator, args []value) (value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("_tan() expects 1 argument, got %d", len(args))
	}
	deg, err := requireAngle("_tan", 1, args[0])
	if err != nil {
		return nil, err
	}
	return math.Tan(deg * math.Pi / 180), nil
}

func builtinAsin(_ *evaluator, args []value) (value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("_asin() expects 1 argument, got %d", len(args))
	}
	n, err := requireNumber("_asin", 1, args[0])
	if err != nil {
		return nil, err
	}
	if n < -1 || n > 1 {
		return nil, fmt.Errorf("_asin() argument out of range [-1, 1]: %g", n)
	}
	return angle{deg: math.Asin(n) * 180 / math.Pi}, nil
}

func builtinAcos(_ *evaluator, args []value) (value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("_acos() expects 1 argument, got %d", len(args))
	}
	n, err := requireNumber("_acos", 1, args[0])
	if err != nil {
		return nil, err
	}
	if n < -1 || n > 1 {
		return nil, fmt.Errorf("_acos() argument out of range [-1, 1]: %g", n)
	}
	return angle{deg: math.Acos(n) * 180 / math.Pi}, nil
}

func builtinAtan2(_ *evaluator, args []value) (value, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("_atan2() expects 2 arguments, got %d", len(args))
	}
	y, err := requireNumber("_atan2", 1, args[0])
	if err != nil {
		return nil, err
	}
	x, err := requireNumber("_atan2", 2, args[1])
	if err != nil {
		return nil, err
	}
	return angle{deg: math.Atan2(y, x) * 180 / math.Pi}, nil
}

// ---------------------------------------------------------------------------
// Math builtins (wrapped for registry)
// ---------------------------------------------------------------------------

func builtinMin(args []value) (value, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("_min() expects 2 arguments, got %d", len(args))
	}
	return mathMinMax("_min", args, false)
}

func builtinMax(args []value) (value, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("_max() expects 2 arguments, got %d", len(args))
	}
	return mathMinMax("_max", args, true)
}

func builtinAbs(args []value) (value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("_abs() expects 1 argument, got %d", len(args))
	}
	return mathAbs(args[0])
}

func builtinSqrt(_ *evaluator, args []value) (value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("_sqrt() expects 1 argument, got %d", len(args))
	}
	n, err := requireNumber("_sqrt", 1, args[0])
	if err != nil {
		return nil, err
	}
	return math.Sqrt(n), nil
}

func builtinPow(_ *evaluator, args []value) (value, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("_pow() expects 2 arguments, got %d", len(args))
	}
	base, err := requireNumber("_pow", 1, args[0])
	if err != nil {
		return nil, err
	}
	exp, err := requireNumber("_pow", 2, args[1])
	if err != nil {
		return nil, err
	}
	return math.Pow(base, exp), nil
}

func builtinFloor(_ *evaluator, args []value) (value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("_floor() expects 1 argument, got %d", len(args))
	}
	n, err := requireNumber("_floor", 1, args[0])
	if err != nil {
		return nil, err
	}
	return math.Floor(n), nil
}

func builtinCeil(_ *evaluator, args []value) (value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("_ceil() expects 1 argument, got %d", len(args))
	}
	n, err := requireNumber("_ceil", 1, args[0])
	if err != nil {
		return nil, err
	}
	return math.Ceil(n), nil
}

func builtinRound(_ *evaluator, args []value) (value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("_round() expects 1 argument, got %d", len(args))
	}
	n, err := requireNumber("_round", 1, args[0])
	if err != nil {
		return nil, err
	}
	return math.Round(n), nil
}

func builtinLerp(args []value) (value, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf("_lerp() expects 3 arguments, got %d", len(args))
	}
	return mathLerp(args)
}

// ---------------------------------------------------------------------------
// Conversion builtins
// ---------------------------------------------------------------------------

func builtinString(_ *evaluator, args []value) (value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("_string() expects 1 argument, got %d", len(args))
	}
	switch v := args[0].(type) {
	case string:
		return v, nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case length:
		return strconv.FormatFloat(v.mm, 'f', -1, 64), nil
	case angle:
		return strconv.FormatFloat(v.deg, 'f', -1, 64), nil
	case bool:
		if v {
			return "true", nil
		}
		return "false", nil
	default:
		return nil, fmt.Errorf("_string() expects Length, Angle, Number, Bool, or String, got %s", typeName(args[0]))
	}
}

func builtinNumber(_ *evaluator, args []value) (value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("_number() expects 1 argument, got %d", len(args))
	}
	switch v := args[0].(type) {
	case length:
		return v.mm, nil
	case angle:
		return v.deg, nil
	case float64:
		return v, nil
	case string:
		n, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, fmt.Errorf("_number() cannot parse %q as a number", v)
		}
		return n, nil
	default:
		return nil, fmt.Errorf("_number() expects Length, Angle, Number, or String, got %s", typeName(args[0]))
	}
}

func builtinSize(_ *evaluator, args []value) (value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("_size() expects 1 argument, got %d", len(args))
	}
	switch v := args[0].(type) {
	case array:
		return float64(len(v.elems)), nil
	case string:
		return float64(len([]rune(v))), nil
	default:
		return nil, fmt.Errorf("_size() expects Array or String, got %s", typeName(args[0]))
	}
}

// ---------------------------------------------------------------------------
// Time builtins
// ---------------------------------------------------------------------------

func builtinUtcDate(_ *evaluator, args []value) (value, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf("_utc_date() expects 0 arguments, got %d", len(args))
	}
	return time.Now().UTC().Format("1/2/2006"), nil
}

func builtinUtcTime(_ *evaluator, args []value) (value, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf("_utc_time() expects 0 arguments, got %d", len(args))
	}
	return time.Now().UTC().Format("15:04:05"), nil
}
