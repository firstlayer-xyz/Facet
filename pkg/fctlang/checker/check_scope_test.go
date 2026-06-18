package checker

import "testing"

// A struct field default may legitimately reference a module-level global.
// checkStructDefaults must infer the default against the source's global env
// (stdlib + module globals), not a bare stdlib env, or it rejects the reference
// as undefined.
func TestCheckStructDefaultReferencesGlobal(t *testing.T) {
	expectNoErrors(t, `
const baseW = 10 mm

type Widget {
    w Length = baseW
}

fn Main() Solid {
    var x = Widget{}
    return Cube(s: Vec3{x: x.w, y: 1 mm, z: 1 mm})
}
`)
}

// In a multi-clause for-yield, a later clause's iterable may reference an earlier
// clause's loop variable. Each clause's iterable must be inferred against the env
// that accumulates the prior clauses' bindings, not the outer env.
func TestCheckMultiClauseForYieldReferencesEarlierVar(t *testing.T) {
	expectNoErrors(t, `
fn Main() {
    var rows = [[1, 2], [3, 4]]
    var flat = for row rows, x row {
        yield x
    }
    return flat
}
`)
}
