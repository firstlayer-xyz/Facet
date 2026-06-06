package emit

import (
	"context"
	"strings"
	"testing"

	"facet/pkg/fctlang/checker"
	"facet/pkg/fctlang/formatter"
	"facet/pkg/fctlang/loader"
	"facet/pkg/fctlang/parser"
)

// The BOSL2 attachment runtime must be valid Facet on its own: it has to parse,
// round-trip through the formatter (as every transpile does), and type-check
// against the stdlib. This test exercises the runtime through a representative
// Main that uses position and attach, so a syntax or type error in the embedded
// library source is caught here rather than surfacing in every BOSL2 transpile.
func TestBosl2RuntimeTypechecks(t *testing.T) {
	main := `fn Main() Solid {
    return b2_cuboid(size: Vec3{x: 20 mm, y: 20 mm, z: 10 mm})
        .attach(pa: B2Anchor{x: 0, y: 0, z: 1}, ca: B2Anchor{x: 0, y: 0, z: -1}, child: b2_cyl(h: 8 mm, r: 3 mm))
        .position(a: B2Anchor{x: 1, y: 0, z: 1}, child: b2_sphere(r: 2 mm))
        .Solid()
}
`
	src := bosl2Runtime + "\n" + main

	// Parse + format, mirroring scad.Transpile's reformat stage.
	ast, err := parser.Parse(src, "<bosl2-runtime>", parser.SourceUser)
	if err != nil {
		t.Fatalf("runtime does not parse: %v", err)
	}
	formatted := formatter.Format(ast)

	prog, err := loader.Load(context.Background(), formatted, "<bosl2-runtime>", parser.SourceUser, "", nil)
	if err != nil {
		t.Fatalf("runtime does not load: %v\n%s", err, formatted)
	}
	if errs := checker.Check(prog).Errors; len(errs) > 0 {
		var b strings.Builder
		for _, e := range errs {
			b.WriteString(e.Error())
			b.WriteString("\n")
		}
		t.Fatalf("runtime fails type-check:\n%s\n--- source ---\n%s", b.String(), formatted)
	}
}
