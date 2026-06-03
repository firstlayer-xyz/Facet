package evaluator

import (
	"fmt"
)

// requireLength extracts the mm value from a length argument.
// Also accepts bare numbers (float64), treating them as mm.
func requireLength(funcName string, argNum int, v value) (float64, error) {
	v = unwrap(v)
	switch n := v.(type) {
	case length:
		return n.mm, nil
	case float64:
		return n, nil
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
		return n, nil
	}
	return 0, fmt.Errorf("%s() argument %d must be a Number, got %s (use Number(from: x) to convert Length explicitly)", funcName, argNum, typeName(v))
}

// requireAngle extracts the degree value from an angle argument.
func requireAngle(funcName string, argNum int, v value) (float64, error) {
	v = unwrap(v)
	a, ok := v.(angle)
	if !ok {
		return 0, fmt.Errorf("%s() argument %d must be an Angle (e.g. 45 deg), got %s", funcName, argNum, typeName(v))
	}
	return a.deg, nil
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
