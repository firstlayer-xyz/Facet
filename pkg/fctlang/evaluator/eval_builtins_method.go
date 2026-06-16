package evaluator

import (
	"facet/pkg/manifold"
	"fmt"
)

// Method-builtin adapters. Every method builtin opens by asserting that the
// receiver (args[0]) has the expected type; these helpers factor out that
// assertion so each body starts from a typed receiver and the arguments after
// it. The body keeps its own arity check, because the arity error messages vary
// per method (singular vs plural, parenthetical parameter hints). The receiver
// is stripped from the args passed to the body, so body args are 1-based from
// the user's perspective: args[0] is the first user argument.

func structMethod(name string, body func(*structVal, []value) (value, error)) builtinFn {
	return func(e *evaluator, args []value) (value, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("%s: missing receiver", name)
		}
		sv, ok := args[0].(*structVal)
		if !ok {
			return nil, fmt.Errorf("%s: expected struct, got %s", name, typeName(args[0]))
		}
		return body(sv, args[1:])
	}
}

func solidMethod(name string, body func(*manifold.Solid, []value) (value, error)) builtinFn {
	return func(e *evaluator, args []value) (value, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("%s: missing receiver", name)
		}
		r, ok := args[0].(*manifold.Solid)
		if !ok {
			return nil, fmt.Errorf("%s: expected Solid, got %s", name, typeName(args[0]))
		}
		return body(r, args[1:])
	}
}

func sketchMethod(name string, body func(*manifold.Sketch, []value) (value, error)) builtinFn {
	return func(e *evaluator, args []value) (value, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("%s: missing receiver", name)
		}
		pf, ok := args[0].(*manifold.Sketch)
		if !ok {
			return nil, fmt.Errorf("%s: expected Sketch, got %s", name, typeName(args[0]))
		}
		return body(pf, args[1:])
	}
}

func stringMethod(name string, body func(string, []value) (value, error)) builtinFn {
	return func(e *evaluator, args []value) (value, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("%s: missing receiver", name)
		}
		s, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("%s: expected String, got %s", name, typeName(args[0]))
		}
		return body(s, args[1:])
	}
}
