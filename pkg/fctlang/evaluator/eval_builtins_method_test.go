package evaluator

import (
	"strings"
	"testing"

	"facet/pkg/manifold"
)

// The method-builtin adapters index args[0] for the receiver; a call with no
// args must return a clear "missing receiver" error rather than panicking on the
// out-of-range index. (The guard fires before the evaluator is touched, so a nil
// *evaluator is fine here.)
func TestMethodAdapterMissingReceiver(t *testing.T) {
	adapters := map[string]builtinFn{
		"struct": structMethod("S", func(*structVal, []value) (value, error) { return nil, nil }),
		"solid":  solidMethod("So", func(*manifold.Solid, []value) (value, error) { return nil, nil }),
		"sketch": sketchMethod("Sk", func(*manifold.Sketch, []value) (value, error) { return nil, nil }),
		"string": stringMethod("St", func(string, []value) (value, error) { return nil, nil }),
	}
	for kind, fn := range adapters {
		_, err := fn(nil, nil)
		if err == nil || !strings.Contains(err.Error(), "missing receiver") {
			t.Errorf("%s adapter with no args: err = %v, want a 'missing receiver' error", kind, err)
		}
	}
}
