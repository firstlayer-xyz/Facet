package evaluator

import (
	"fmt"
	"regexp"
	"strings"
)

func init() {
	builtinRegistry["_sub_str"] = func(_ *evaluator, args []value) (value, error) {
		const name = "_sub_str"
		s, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("%s: expected String, got %s", name, typeName(args[0]))
		}
		if len(args) != 3 {
			return nil, fmt.Errorf("%s() expects 2 arguments, got %d", name, len(args)-1)
		}
		start, err := requireNumber(name, 1, args[1])
		if err != nil {
			return nil, err
		}
		length, err := requireNumber(name, 2, args[2])
		if err != nil {
			return nil, err
		}
		runes := []rune(s)
		si := int(start)
		li := int(length)
		if si < 0 {
			si = 0
		}
		if si > len(runes) {
			return "", nil
		}
		if li < 0 {
			return "", nil
		}
		end := si + li
		if end > len(runes) {
			end = len(runes)
		}
		return string(runes[si:end]), nil
	}

	builtinRegistry["_has_prefix"] = func(_ *evaluator, args []value) (value, error) {
		const name = "_has_prefix"
		s, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("%s: expected String, got %s", name, typeName(args[0]))
		}
		if len(args) != 2 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args)-1)
		}
		prefix, err := requireString(name, 1, args[1])
		if err != nil {
			return nil, err
		}
		return strings.HasPrefix(s, prefix), nil
	}

	builtinRegistry["_has_suffix"] = func(_ *evaluator, args []value) (value, error) {
		const name = "_has_suffix"
		s, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("%s: expected String, got %s", name, typeName(args[0]))
		}
		if len(args) != 2 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args)-1)
		}
		suffix, err := requireString(name, 1, args[1])
		if err != nil {
			return nil, err
		}
		return strings.HasSuffix(s, suffix), nil
	}

	builtinRegistry["_match"] = func(_ *evaluator, args []value) (value, error) {
		const name = "_match"
		s, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("%s: expected String, got %s", name, typeName(args[0]))
		}
		if len(args) != 2 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args)-1)
		}
		pattern, err := requireString(name, 1, args[1])
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
	}

	builtinRegistry["_to_upper"] = func(_ *evaluator, args []value) (value, error) {
		const name = "_to_upper"
		s, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("%s: expected String, got %s", name, typeName(args[0]))
		}
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args)-1)
		}
		return strings.ToUpper(s), nil
	}

	builtinRegistry["_to_lower"] = func(_ *evaluator, args []value) (value, error) {
		const name = "_to_lower"
		s, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("%s: expected String, got %s", name, typeName(args[0]))
		}
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args)-1)
		}
		return strings.ToLower(s), nil
	}

	builtinRegistry["_trim_str"] = func(_ *evaluator, args []value) (value, error) {
		const name = "_trim_str"
		s, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("%s: expected String, got %s", name, typeName(args[0]))
		}
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args)-1)
		}
		return strings.TrimSpace(s), nil
	}

	builtinRegistry["_replace"] = func(_ *evaluator, args []value) (value, error) {
		const name = "_replace"
		s, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("%s: expected String, got %s", name, typeName(args[0]))
		}
		if len(args) != 3 {
			return nil, fmt.Errorf("%s() expects 2 arguments, got %d", name, len(args)-1)
		}
		old, err := requireString(name, 1, args[1])
		if err != nil {
			return nil, err
		}
		newStr, err := requireString(name, 2, args[2])
		if err != nil {
			return nil, err
		}
		return strings.ReplaceAll(s, old, newStr), nil
	}

	builtinRegistry["_index_of"] = func(_ *evaluator, args []value) (value, error) {
		const name = "_index_of"
		s, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("%s: expected String, got %s", name, typeName(args[0]))
		}
		if len(args) != 2 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args)-1)
		}
		substr, err := requireString(name, 1, args[1])
		if err != nil {
			return nil, err
		}
		return float64(strings.Index(s, substr)), nil
	}

	builtinRegistry["_contains"] = func(_ *evaluator, args []value) (value, error) {
		const name = "_contains"
		s, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("%s: expected String, got %s", name, typeName(args[0]))
		}
		if len(args) != 2 {
			return nil, fmt.Errorf("%s() expects 1 argument, got %d", name, len(args)-1)
		}
		substr, err := requireString(name, 1, args[1])
		if err != nil {
			return nil, err
		}
		return strings.Contains(s, substr), nil
	}

	builtinRegistry["_length"] = func(_ *evaluator, args []value) (value, error) {
		const name = "_length"
		s, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("%s: expected String, got %s", name, typeName(args[0]))
		}
		if len(args) != 1 {
			return nil, fmt.Errorf("%s() expects 0 arguments, got %d", name, len(args)-1)
		}
		return float64(len([]rune(s))), nil
	}
}
