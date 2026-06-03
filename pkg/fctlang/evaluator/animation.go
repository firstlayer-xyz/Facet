package evaluator

import (
	"fmt"
	"sync"

	"facet/pkg/manifold"
)

// Animation is an evaluated, time-driven model. It retains the evaluator that
// produced it so the frame closure's captured invariant setup (and globals)
// stay live and are reused on every Frame call — the model body runs once, not
// per frame. Frame produces the Solid for a given instant.
type Animation struct {
	mu      sync.Mutex
	e       *evaluator
	frame   *functionVal
	argName string // the frame lambda's sole parameter name (time, in ms)
}

// Frame evaluates the model at timeMs (milliseconds; the frame lambda decides
// how to interpret it) and returns the Solid for that instant.
// Frame is concurrency-safe: concurrent calls on the same handle are serialized.
func (a *Animation) Frame(timeMs float64) (*manifold.Solid, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	v, err := a.e.callFunctionVal(a.frame, map[string]value{a.argName: timeMs})
	if err != nil {
		return nil, err
	}
	solid, ok := v.(*manifold.Solid)
	if !ok {
		return nil, fmt.Errorf("animation frame returned %s, expected Solid", typeName(v))
	}
	return solid, nil
}
