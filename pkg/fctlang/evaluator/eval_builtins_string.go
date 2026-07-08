package evaluator

import (
	"fmt"
	"regexp"
	"strings"
)

func init() {
	builtinRegistry["_sub_str"] = stringMethod("_sub_str", func(s string, args []value) (value, error) {
		const name = "_sub_str"
		if len(args) != 2 {
			return nil, fmt.Errorf("%s() expects 2 arguments, got %d", name, len(args))
		}
		start, err := requireNumber(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		length, err := requireNumber(name, 2, args[1])
		if err != nil {
			return nil, err
		}
		runes := []rune(s)
		// A negative start or length was silently clamped (start→0, length→"")
		// rather than reported — the same class the array-index/slice operators
		// hard-error on. Reject them; a start past the end is still an empty
		// slice, and an over-long length still clamps to the string end.
		if start < 0 {
			return nil, fmt.Errorf("%s() start must be non-negative, got %v", name, start)
		}
		if length < 0 {
			return nil, fmt.Errorf("%s() length must be non-negative, got %v", name, length)
		}
		si := int(start)
		li := int(length)
		if si > len(runes) {
			return "", nil
		}
		end := si + li
		if end > len(runes) {
			end = len(runes)
		}
		return string(runes[si:end]), nil
	})

	builtinRegistry["_has_prefix"] = stringMethod("_has_prefix", func(s string, args []value) (value, error) {
		const name = "_has_prefix"
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args))
		}
		prefix, err := requireString(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		return strings.HasPrefix(s, prefix), nil
	})

	builtinRegistry["_has_suffix"] = stringMethod("_has_suffix", func(s string, args []value) (value, error) {
		const name = "_has_suffix"
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args))
		}
		suffix, err := requireString(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		return strings.HasSuffix(s, suffix), nil
	})

	builtinRegistry["_match"] = stringMethod("_match", func(s string, args []value) (value, error) {
		const name = "_match"
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args))
		}
		pattern, err := requireString(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("%s(): invalid regex: %v", name, err)
		}
		matches := re.FindStringSubmatch(s)
		if matches == nil {
			return array{elems: []value{}, elemType: "String"}, nil
		}
		if len(matches) > 255 {
			return nil, fmt.Errorf("%s(): too many submatches (max 255)", name)
		}
		elems := make([]value, len(matches))
		for i, m := range matches {
			elems[i] = m
		}
		return array{elems: elems, elemType: "String"}, nil
	})

	builtinRegistry["_to_upper"] = stringMethod("_to_upper", func(s string, args []value) (value, error) {
		const name = "_to_upper"
		if len(args) != 0 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args))
		}
		return strings.ToUpper(s), nil
	})

	builtinRegistry["_to_lower"] = stringMethod("_to_lower", func(s string, args []value) (value, error) {
		const name = "_to_lower"
		if len(args) != 0 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args))
		}
		return strings.ToLower(s), nil
	})

	builtinRegistry["_trim_str"] = stringMethod("_trim_str", func(s string, args []value) (value, error) {
		const name = "_trim_str"
		if len(args) != 0 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args))
		}
		return strings.TrimSpace(s), nil
	})

	builtinRegistry["_replace"] = stringMethod("_replace", func(s string, args []value) (value, error) {
		const name = "_replace"
		if len(args) != 2 {
			return nil, fmt.Errorf("%s() expects 2 arguments, got %d", name, len(args))
		}
		old, err := requireString(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		newStr, err := requireString(name, 2, args[1])
		if err != nil {
			return nil, err
		}
		return strings.ReplaceAll(s, old, newStr), nil
	})

	builtinRegistry["_index_of"] = stringMethod("_index_of", func(s string, args []value) (value, error) {
		const name = "_index_of"
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args))
		}
		substr, err := requireString(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		// Return a rune index, not a byte offset: SubStr/Length operate on runes,
		// so a byte offset would mis-index any string with multi-byte characters.
		// strings.Index returns -1 when absent; preserve it (the stdlib maps it to
		// None).
		idx := strings.Index(s, substr)
		if idx < 0 {
			return float64(-1), nil
		}
		return float64(len([]rune(s[:idx]))), nil
	})

	builtinRegistry["_contains"] = stringMethod("_contains", func(s string, args []value) (value, error) {
		const name = "_contains"
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args))
		}
		substr, err := requireString(name, 1, args[0])
		if err != nil {
			return nil, err
		}
		return strings.Contains(s, substr), nil
	})

	builtinRegistry["_length"] = stringMethod("_length", func(s string, args []value) (value, error) {
		const name = "_length"
		if len(args) != 0 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args))
		}
		return float64(len([]rune(s))), nil
	})
}
