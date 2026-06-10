package evaluator

import (
	"fmt"
	"math"
)

// requireFinite rejects NaN/Inf. Builtin numeric arguments flow into the C++
// geometry kernel, whose `x <= 0` style guards pass NaN (every comparison with
// NaN is false) — the model would then silently vanish instead of erroring.
func requireFinite(funcName string, argNum int, n float64) (float64, error) {
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return 0, fmt.Errorf("%s() argument %d must be finite, got %v", funcName, argNum, n)
	}
	return n, nil
}

// requireLength extracts the mm value from a length argument.
// Also accepts bare numbers (float64), treating them as mm.
func requireLength(funcName string, argNum int, v value) (float64, error) {
	v = unwrap(v)
	switch n := v.(type) {
	case length:
		return requireFinite(funcName, argNum, n.mm)
	case float64:
		return requireFinite(funcName, argNum, n)
	default:
		return 0, fmt.Errorf("%s() argument %d must be a Length (e.g. 10 mm), got %s", funcName, argNum, typeName(v))
	}
}

// requireNumber extracts a plain numeric value from an argument.
// Only accepts float64 — Length is a distinct type under strict units, and
// must be converted explicitly with Number(from: x) at the call site.
func requireNumber(funcName string, argNum int, v value) (float64, error) {
	v = unwrap(v)
	if n, ok := v.(float64); ok {
		return requireFinite(funcName, argNum, n)
	}
	return 0, fmt.Errorf("%s() argument %d must be a Number, got %s (use Number(from: x) to convert Length explicitly)", funcName, argNum, typeName(v))
}

// requireCount converts an already-extracted Number into a non-negative integer
// count (circular segments, subdivision factor), rejecting non-finite, negative,
// or above-max values. Such counts flow straight into the C++ geometry kernel,
// which does not clamp, so an unbounded value would OOM/hang the host; this
// turns that into a clean error at the boundary. Zero is allowed — for segment
// counts it means "auto/default".
func requireCount(funcName string, argNum int, n float64, max int) (int, error) {
	switch {
	case math.IsNaN(n) || math.IsInf(n, 0):
		return 0, fmt.Errorf("%s() argument %d must be a finite count, got %v", funcName, argNum, n)
	case n < 0:
		return 0, fmt.Errorf("%s() argument %d must be a non-negative count, got %v", funcName, argNum, n)
	case n > float64(max):
		return 0, fmt.Errorf("%s() argument %d (%v) exceeds the maximum supported count of %d", funcName, argNum, n, max)
	}
	return int(n), nil
}

// requireAngle extracts the degree value from an angle argument.
func requireAngle(funcName string, argNum int, v value) (float64, error) {
	v = unwrap(v)
	a, ok := v.(angle)
	if !ok {
		return 0, fmt.Errorf("%s() argument %d must be an Angle (e.g. 45 deg), got %s", funcName, argNum, typeName(v))
	}
	return requireFinite(funcName, argNum, a.deg)
}

// requireString extracts a string from a value.
func requireString(funcName string, argNum int, v value) (string, error) {
	v = unwrap(v)
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%s() argument %d must be a String, got %s", funcName, argNum, typeName(v))
	}
	return s, nil
}

// optionalLengthMM extracts the mm value from an Optional Length argument,
// returning nil for None so builtins can apply their own default instead of a
// sentinel. A bare (non-Optional) Length is treated as present.
func optionalLengthMM(funcName string, argNum int, v value) (*float64, error) {
	if opt, ok := unwrap(v).(*optionalVal); ok {
		if !opt.present {
			return nil, nil
		}
		v = opt.inner
	}
	mm, err := requireLength(funcName, argNum, v)
	if err != nil {
		return nil, err
	}
	return &mm, nil
}

// optionalString extracts the string from an Optional String argument,
// returning nil for None. A bare (non-Optional) String is treated as present.
func optionalString(funcName string, argNum int, v value) (*string, error) {
	if opt, ok := unwrap(v).(*optionalVal); ok {
		if !opt.present {
			return nil, nil
		}
		v = opt.inner
	}
	s, err := requireString(funcName, argNum, v)
	if err != nil {
		return nil, err
	}
	return &s, nil
}
