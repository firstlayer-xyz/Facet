package evaluator

import (
	"facet/pkg/manifold"
	"fmt"
)

// Method-builtin adapters. Every method builtin opens by asserting that the
// receiver (args[0]) has the expected type; method factors out that assertion
// so each body starts from a typed receiver and the arguments after it. The
// body keeps its own arity check, because the arity error messages vary per
// method (singular vs plural, parenthetical parameter hints). The receiver is
// stripped from the args passed to the body, so body args are 1-based from the
// user's perspective: args[0] is the first user argument.

// method adapts a typed-receiver body to a builtinFn: it asserts args[0] is a
// T (naming the expected type `want` in the error) and passes the remaining
// args to body.
func method[T value](name, want string, body func(T, []value) (value, error)) builtinFn {
	return func(_ *evaluator, args []value) (value, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("%s: missing receiver", name)
		}
		r, ok := args[0].(T)
		if !ok {
			return nil, fmt.Errorf("%s: expected %s, got %s", name, want, typeName(args[0]))
		}
		return body(r, args[1:])
	}
}

func structMethod(name string, body func(*structVal, []value) (value, error)) builtinFn {
	return method(name, "struct", body)
}

func solidMethod(name string, body func(*manifold.Solid, []value) (value, error)) builtinFn {
	return method(name, "Solid", body)
}

func sketchMethod(name string, body func(*manifold.Sketch, []value) (value, error)) builtinFn {
	return method(name, "Sketch", body)
}

func stringMethod(name string, body func(string, []value) (value, error)) builtinFn {
	return method(name, "String", body)
}
