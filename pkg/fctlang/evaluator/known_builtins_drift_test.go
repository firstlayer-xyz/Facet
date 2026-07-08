package evaluator

import (
	"testing"

	fctchecker "facet/pkg/fctlang/checker"
)

// TestKnownBuiltinsMatchesRegistry keeps checker.KnownBuiltins in exact sync
// with the evaluator's builtinRegistry. The checker uses that set to reject a
// typo'd _-builtin at compile time, so a one-sided addition (a new builtin
// without a KnownBuiltins entry, or vice versa) must fail loudly here rather
// than silently regress the check.
func TestKnownBuiltinsMatchesRegistry(t *testing.T) {
	for name := range builtinRegistry {
		if !fctchecker.KnownBuiltins[name] {
			t.Errorf("builtin %q is registered but missing from checker.KnownBuiltins", name)
		}
	}
	for name := range fctchecker.KnownBuiltins {
		if _, ok := builtinRegistry[name]; !ok {
			t.Errorf("checker.KnownBuiltins lists %q but it is not a registered builtin", name)
		}
	}
}
