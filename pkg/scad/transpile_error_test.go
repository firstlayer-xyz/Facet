package scad

import (
	"strings"
	"testing"
)

// OpenSCAD `let(name=value, …) expr` has no Facet equivalent, so each binding is
// inlined: substituted for its name at every use (later bindings and the body).
func TestTranspileLetExpr(t *testing.T) {
	// let as a list-comprehension body: x is inlined to (i * 2) in the yield.
	res, err := Transpile("pts = [for(i=[0:2]) let(x=i*2) x+1];\ntranslate([pts[0],0,0]) cube(1);\n", "part.scad")
	if err != nil {
		t.Fatalf("let in a comprehension should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "i * 2") {
		t.Fatalf("let binding should inline into the yield:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)

	// sequential bindings: b references the earlier a.
	res, err = Transpile("function f(n) = let(a=n, b=a+1) a+b;\ncube(f(2));\n", "part.scad")
	if err != nil {
		t.Fatalf("sequential let should transpile, got: %v", err)
	}
	assertTypeChecks(t, res.Facet)

	// A comprehension loop variable shadows a same-named let binding in the body:
	// the yield must be the loop's x, not the inlined (5).
	res, err = Transpile("vals = let(x=5) [for(x=[0:2]) x];\ntranslate([vals[0],0,0]) cube(1);\n", "part.scad")
	if err != nil {
		t.Fatalf("loop-var shadowing a let should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "yield x") || strings.Contains(res.Facet, "yield 5") {
		t.Fatalf("loop var must shadow the let binding in the body:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// An unsupported construct must fail the transpile with a located error and
// produce no output — never a placeholder.
func TestTranspileErrorsOnUnsupported(t *testing.T) {
	src := "minkowski() {\n  cube(10);\n  sphere(2);\n}\n"
	res, err := Transpile(src, "part.scad")
	if err == nil {
		t.Fatalf("expected an error for minkowski, got none; output:\n%s", res.Facet)
	}
	if res.Facet != "" {
		t.Fatalf("expected no output on error, got:\n%s", res.Facet)
	}
	if !strings.Contains(err.Error(), "minkowski") || !strings.Contains(err.Error(), "1:1") {
		t.Fatalf("error should name the construct and location, got: %v", err)
	}
}

// Local assignments in a geometry for-loop body (`for(i=…){ x = f(i); …x… }`)
// are OpenSCAD block-scoped bindings; they emit as Facet consts inside the
// for-yield body so the geometry can reference them.
func TestTranspileForBodyLocalAssign(t *testing.T) {
	res, err := Transpile("for(i=[0:2]){ x = i*3; y = x+1; translate([x,y,0]) cube(1); }\n", "part.scad")
	if err != nil {
		t.Fatalf("for-body local assign should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "const x = i * 3") || !strings.Contains(res.Facet, "const y = x + 1") {
		t.Fatalf("expected for-body assigns emitted as consts:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// A list comprehension `[for (var = range) body]` renders to a Facet
// `for var range { yield body }`. Used heavily in real OpenSCAD models that
// build polygon points by sampling a function over a range.
func TestTranspileListComprehensionInPolygon(t *testing.T) {
	src := "polygon([for (a = [0:0.1:1]) [a*10, a*5]]);\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("list comprehension should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "for a [0:1:0.1] { yield [a * 10, a * 5] }") {
		t.Fatalf("expected for-yield in output:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// A module body of a single statement (no braces) parses the same as a
// braced block. `module curve() polygon(...);` and `module curve() { polygon(...); }`
// produce identical Facet output.
func TestTranspileModuleSingleStatementBody(t *testing.T) {
	src := "module unit_sq() square([1,1]);\nunit_sq();\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("single-statement module body should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "fn unit_sq") {
		t.Fatalf("expected unit_sq definition:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// echo() and assert() are debug-only in OpenSCAD; the render-only Facet
// program drops them silently rather than refusing to transpile.
func TestTranspileEchoAssertSilentlyDropped(t *testing.T) {
	src := `echo("hello");` + "\nassert(true);\ncube(10);\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("echo/assert should be dropped, got: %v", err)
	}
	if strings.Contains(res.Facet, "echo") || strings.Contains(res.Facet, "assert") {
		t.Fatalf("echo/assert should not appear in output:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// A non-literal color value (a parameter or other expression) passes through
// to Color(hex:); the Facet runtime resolves a CSS name to its hex value via
// the shared colorname table.
func TestTranspileNonLiteralColorPassThrough(t *testing.T) {
	src := "module tinted(c) color(c) cube(5);\ntinted(\"red\");\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("non-literal color should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, ".Color(hex: c)") {
		t.Fatalf("expected runtime Color(hex:) call:\n%s", res.Facet)
	}
	if !strings.Contains(res.Facet, "c String") {
		t.Fatalf("expected parameter c classified as String:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// A "#rgb"/"#rrggbb" hex color is expanded to #RRGGBB and passed to Color(hex:).
// OpenSCAD/BOSL2 commonly use hex colors (e.g. color("#f77")).
func TestTranspileHexColor(t *testing.T) {
	cases := []struct{ in, want string }{
		{`color("#f77") cube(5);`, `Color(hex: "#FF7777")`},
		{`color("#FF7755") cube(5);`, `Color(hex: "#FF7755")`},
	}
	for _, c := range cases {
		res, err := Transpile(c.in+"\n", "part.scad")
		if err != nil {
			t.Fatalf("%q should transpile, got: %v", c.in, err)
		}
		if !strings.Contains(res.Facet, c.want) {
			t.Fatalf("%q: expected %q in:\n%s", c.in, c.want, res.Facet)
		}
		assertTypeChecks(t, res.Facet)
	}
}

// Common CSS color names beyond the basic 16 resolve (e.g. lightgray).
func TestTranspileExtendedCSSColors(t *testing.T) {
	for _, name := range []string{"lightgray", "lightgrey", "darkgray", "gold", "skyblue"} {
		res, err := Transpile(`color("`+name+`") cube(5);`+"\n", "part.scad")
		if err != nil {
			t.Fatalf("color(%q) should transpile, got: %v", name, err)
		}
		if !strings.Contains(res.Facet, "Color(hex:") {
			t.Fatalf("color(%q): expected Color(hex:) in:\n%s", name, res.Facet)
		}
	}
}

// A translate whose vector is a runtime expression (not a literal) indexes
// the expression per axis so Facet's Move receives Length-coercible Numbers.
// scale(s) with a SCALAR scales every axis uniformly. It was previously dropped
// entirely (the scale vanished), which silently under-sized models such as the
// icosphere (which relies on scale(radius)).
func TestTranspileScaleScalar(t *testing.T) {
	res, err := Transpile("scale(10) cube([1, 1, 1]);\n", "part.scad")
	if err != nil {
		t.Fatalf("scale(scalar) should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "Scale(x: 10, y: 10, z: 10)") {
		t.Fatalf("expected a uniform scalar scale:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// A scalar scale through a module parameter (the icosphere's scale(radius)) also
// broadcasts to all axes.
func TestTranspileScaleScalarVariable(t *testing.T) {
	res, err := Transpile("module f(r) scale(r) cube([1, 1, 1]);\nf(10);\n", "part.scad")
	if err != nil {
		t.Fatalf("got: %v", err)
	}
	if !strings.Contains(res.Facet, "Scale(x: r, y: r, z: r)") {
		t.Fatalf("expected scalar-variable broadcast:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// A scale vector with fewer than three components defaults the omitted axes to 1
// (no scaling), matching OpenSCAD — not 0, which would collapse the axis.
func TestTranspileScaleVectorPadsWithOne(t *testing.T) {
	res, err := Transpile("scale([2, 3]) cube([1, 1, 1]);\n", "part.scad")
	if err != nil {
		t.Fatalf("got: %v", err)
	}
	if !strings.Contains(res.Facet, "Scale(x: 2, y: 3, z: 1)") {
		t.Fatalf("expected omitted z to default to 1:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

func TestTranspileTranslateComputedVector(t *testing.T) {
	src := "module shift(s) translate(s * [1, 2, 0]) cube(3);\nshift(5);\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("computed translate should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "[0] * 1 mm") || !strings.Contains(res.Facet, "[1] * 1 mm") || !strings.Contains(res.Facet, "[2] * 1 mm") {
		t.Fatalf("expected per-axis indexing:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// mirror with a non-literal (computed) vector normal is indexed per axis rather
// than silently dropped — it used to fall through vecArg and vanish entirely.
func TestTranspileMirrorComputedNormal(t *testing.T) {
	res, err := Transpile("module m(s) mirror(s * [1, 0, 0]) cube([1, 2, 3]);\nm(1);\n", "part.scad")
	if err != nil {
		t.Fatalf("mirror with a computed normal should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "Mirror(") {
		t.Fatalf("mirror must not be dropped for a computed normal:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// mirror without a normal vector is a hard error, not a dropped operation
// (no fallbacks).
func TestTranspileMirrorWithoutNormalErrors(t *testing.T) {
	_, err := Transpile("mirror() cube(2);\n", "part.scad")
	if err == nil {
		t.Fatal("expected an error for mirror without a normal")
	}
	if !strings.Contains(err.Error(), "mirror") {
		t.Fatalf("expected a mirror error, got: %v", err)
	}
}

// rotate without an angle is a hard error rather than a silently dropped
// rotation.
func TestTranspileRotateWithoutAngleErrors(t *testing.T) {
	_, err := Transpile("rotate() cube(2);\n", "part.scad")
	if err == nil {
		t.Fatal("expected an error for rotate without an angle")
	}
	if !strings.Contains(err.Error(), "rotate") {
		t.Fatalf("expected a rotate error, got: %v", err)
	}
}

// offset(r=…) is approximated as mitered Offset(delta:…). The visual
// difference between rounded and mitered offsets is small for thin offsets
// (line outlines, fillets at small radii), and the alternative — refusing to
// transpile any model that uses `r` — blocks too many real OpenSCAD files.
func TestTranspileOffsetRAsDelta(t *testing.T) {
	res, err := Transpile("offset(r=5) circle(r=3);\n", "part.scad")
	if err != nil {
		t.Fatalf("offset(r=…) should transpile as Offset(delta:…), got: %v", err)
	}
	if !strings.Contains(res.Facet, "Offset(delta: 5 mm)") {
		t.Fatalf("expected Offset(delta: 5 mm) approximation:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

func TestTranspileErrorsOnUnknownColor(t *testing.T) {
	res, err := Transpile(`color("xyzzy") cube(10);`+"\n", "part.scad")
	if err == nil || res.Facet != "" {
		t.Fatalf("unknown color should fail with no output; err=%v out=%q", err, res.Facet)
	}
	if !strings.Contains(err.Error(), "xyzzy") {
		t.Fatalf("error should mention the color name, got: %v", err)
	}
}

// linear_extrude scale + center must be faithfully translated (taper / centering),
// not silently dropped, and the result must type-check.
func TestTranspileLinearExtrudeScaleAndCenter(t *testing.T) {
	res, err := Transpile("linear_extrude(height=10, scale=0.5, center=true) square([4,4]);\n", "part.scad")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Facet, "taperX: 0.5") || !strings.Contains(res.Facet, "taperY: 0.5") {
		t.Fatalf("scale=0.5 not translated to taper:\n%s", res.Facet)
	}
	if !strings.Contains(res.Facet, "AlignCenter") {
		t.Fatalf("center=true not translated:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// An argument the emitter does not translate must error, never be silently dropped.
func TestTranspileLinearExtrudeUnknownArgErrors(t *testing.T) {
	res, err := Transpile("linear_extrude(height=10, frobnicate=3) square([4,4]);\n", "part.scad")
	if err == nil || res.Facet != "" {
		t.Fatalf("unknown arg should fail with no output; err=%v out=%q", err, res.Facet)
	}
	if !strings.Contains(err.Error(), "frobnicate") {
		t.Fatalf("error should name the unsupported argument, got: %v", err)
	}
}

// rotate_extrude maps faithfully to Revolve (around Z) — it must NOT error.
func TestTranspileRotateExtrudeNoError(t *testing.T) {
	src := "rotate_extrude(angle=360) translate([10,0]) circle(2);\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("rotate_extrude should transpile cleanly, got: %v", err)
	}
	if !strings.Contains(res.Facet, "Revolve") {
		t.Fatalf("expected a Revolve in output, got:\n%s", res.Facet)
	}
}

// text(font=…) is faithfully translated to Facet's Text font parameter.
func TestTranspileTextFont(t *testing.T) {
	res, err := Transpile(`text("hi", font="Arial");`+"\n", "part.scad")
	if err != nil {
		t.Fatalf("text with font= should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, `font: "Arial"`) {
		t.Fatalf("font not translated:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// The named vector forms (translate(v=…), mirror(v=…)) translate the same as the
// positional forms — they must not be rejected as unsupported arguments.
func TestTranspileNamedVectorForms(t *testing.T) {
	res, err := Transpile("mirror(v=[1,0,0]) translate(v=[2,0,0]) cube(4);\n", "part.scad")
	if err != nil {
		t.Fatalf("named v= forms should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "Mirror(") || !strings.Contains(res.Facet, "Move(") {
		t.Fatalf("expected Mirror and Move in output:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// Per-call $fa/$fs compute the segment count from the shape's radius via
// OpenSCAD's fragment formula ceil(max(min(360/$fa, 2π·r/$fs), 5)).
func TestTranspilePerCallFaFs(t *testing.T) {
	res, err := Transpile("sphere(r=5, $fa=12);\n", "part.scad")
	if err != nil {
		t.Fatalf("sphere($fa=…) should transpile, got: %v", err)
	}
	// min(360/12, 2π·5/2) = min(30, 15.7) = 15.7 → ceil = 16
	if !strings.Contains(res.Facet, "segments: 16") {
		t.Fatalf("per-call $fa not applied via fragment formula (expected 16):\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// rotate_extrude($fn=…) maps to Revolve's segments parameter (added to the
// stdlib in #38) — it must be translated, not rejected.
func TestTranspileRotateExtrudeSegments(t *testing.T) {
	res, err := Transpile("rotate_extrude(angle=270, $fn=64) translate([10,0]) circle(2);\n", "part.scad")
	if err != nil {
		t.Fatalf("rotate_extrude with $fn should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "segments: 64") {
		t.Fatalf("$fn not translated to segments:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// A top-level $fn sets a global segment count applied to every curved primitive.
func TestTranspileGlobalFn(t *testing.T) {
	res, err := Transpile("$fn=64;\nsphere(5);\n", "part.scad")
	if err != nil {
		t.Fatalf("global $fn should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "segments: 64") {
		t.Fatalf("global $fn not applied:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// A top-level $fa sets a global resolution applied per shape via the fragment
// formula (radius-dependent, honoring the default $fs=2).
func TestTranspileGlobalFa(t *testing.T) {
	res, err := Transpile("$fa=6;\ncircle(10);\n", "part.scad")
	if err != nil {
		t.Fatalf("global $fa should transpile, got: %v", err)
	}
	// min(360/6, 2π·10/2) = min(60, 31.4) = 31.4 → ceil = 32
	if !strings.Contains(res.Facet, "segments: 32") {
		t.Fatalf("global $fa not applied via fragment formula (expected 32):\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// A per-call $fn overrides the global resolution.
func TestTranspileLocalFnOverridesGlobal(t *testing.T) {
	res, err := Transpile("$fn=64;\nsphere(r=5, $fn=8);\n", "part.scad")
	if err != nil {
		t.Fatalf("should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "segments: 8") || strings.Contains(res.Facet, "segments: 64") {
		t.Fatalf("local $fn should override global:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// polygon(points, paths) maps to Facet's Polygon(points, holes): path[0] is the
// outer outline; the rest are holes, resolved from index lists into points.
func TestTranspilePolygonHoles(t *testing.T) {
	src := "polygon(points=[[0,0],[10,0],[10,10],[0,10],[3,3],[7,3],[7,7],[3,7]], paths=[[0,1,2,3],[4,5,6,7]]);\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("polygon with holes should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "holes:") {
		t.Fatalf("expected holes in output:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// text(halign=, valign=) anchoring (stdlib #40) passes through to Facet's Text;
// OpenSCAD's halign/valign vocabulary matches Facet's.
func TestTranspileTextAlign(t *testing.T) {
	res, err := Transpile(`text("hi", halign="center", valign="top");`+"\n", "part.scad")
	if err != nil {
		t.Fatalf("text halign/valign should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, `halign: "center"`) || !strings.Contains(res.Facet, `valign: "top"`) {
		t.Fatalf("halign/valign not translated:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// color(...) with alpha uses Facet's [0,1] RGBA overload (stdlib #41 made
// Solid.Color carry opacity); without alpha it keeps the hex form.
func TestTranspileColorAlpha(t *testing.T) {
	// 4th vector component is alpha
	res, err := Transpile("color([1,0,0,0.5]) cube(10);\n", "part.scad")
	if err != nil {
		t.Fatalf("color vector with alpha should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "Color(r:") || !strings.Contains(res.Facet, "a: 0.5") {
		t.Fatalf("rgba color (with alpha) not translated:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)

	// named color + alpha argument
	res2, err := Transpile(`color("red", alpha=0.3) cube(10);`+"\n", "part.scad")
	if err != nil {
		t.Fatalf("named color with alpha should transpile, got: %v", err)
	}
	if !strings.Contains(res2.Facet, "Color(r:") || !strings.Contains(res2.Facet, "a: 0.3") {
		t.Fatalf("named-color alpha not translated:\n%s", res2.Facet)
	}
	assertTypeChecks(t, res2.Facet)
}

// A geometry module with scalar parameters becomes a Facet fn, and the call maps
// positional args to the parameter names (Phase 2). Arithmetic on a parameter
// stays unitless and coerces (base/2 → "base / 2", not "base / 2 mm").
func TestTranspileGeometryModule(t *testing.T) {
	src := "module halfpyramid(base, height) {\n" +
		"    linear_extrude(height, scale=0.01)\n" +
		"        translate([-base/2, 0, 0]) square([base, base/2]);\n" +
		"}\n" +
		"halfpyramid(20, 10);\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("geometry module should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "fn halfpyramid(") {
		t.Fatalf("expected a halfpyramid fn:\n%s", res.Facet)
	}
	if !strings.Contains(res.Facet, "halfpyramid(base: 20, height: 10)") {
		t.Fatalf("expected positional args mapped to names:\n%s", res.Facet)
	}
	if strings.Contains(res.Facet, "/ 2 mm") {
		t.Fatalf("unitless arithmetic wrongly tagged with mm:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// A scalar value function with a let-chain and a math built-in becomes a Facet
// fn; sin maps to Sin and the call maps positional args to names (Phase 2).
func TestTranspileValueFunction(t *testing.T) {
	src := "function f(x) = let(span=150, start=20) sin(x*span+start) / sin(start);\n" +
		"cube(f(1));\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("value function should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "fn f(x Number) Number") {
		t.Fatalf("expected value fn signature:\n%s", res.Facet)
	}
	if !strings.Contains(res.Facet, "Sin(a:") {
		t.Fatalf("expected sin->Sin mapping:\n%s", res.Facet)
	}
	if !strings.Contains(res.Facet, "f(x: 1)") {
		t.Fatalf("expected mapped call f(x: 1):\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// for(i=range) child -> Union(for-yield). SCAD inclusive ranges map directly.
func TestTranspileForLoop(t *testing.T) {
	src := "for (i = [0:3]) translate([i*5, 0, 0]) cube(2);\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("for loop should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "Union(arr: for i [0:3]") {
		t.Fatalf("expected Union(for-yield):\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// cube(size, center) accepts center as a positional argument (OpenSCAD form).
func TestTranspileCubePositionalCenter(t *testing.T) {
	res, err := Transpile("cube([2,3,4], true);\n", "part.scad")
	if err != nil {
		t.Fatalf("cube with positional center should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "AlignCenter") {
		t.Fatalf("positional center not applied:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// Vector parameters are classified []Number (from [0,0] defaults or v[0]
// indexing); .x/.y/.z map to [0]/[1]/[2]; atan2 gives an Angle return.
func TestTranspileVectorParams(t *testing.T) {
	src := "function length2D(v1, v2=[0,0]) = sqrt((v1[0]-v2[0])*(v1[0]-v2[0]) + (v1[1]-v2[1])*(v1[1]-v2[1]));\n" +
		"function getAngle2D(v1, v2=[0,0]) = atan2(v2[0]-v1[0], v2[1]-v1[1]);\n" +
		"module rod(v1=[0,0], v2=[0,0], t=6) {\n" +
		"    ang = getAngle2D(v1, v2);\n" +
		"    len = length2D(v1, v2);\n" +
		"    translate([v1[0], v1[1]]) rotate([0,0,-ang]) cube([t, len, t]);\n" +
		"}\n" +
		"rod([0,0], [10,5], 3);\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("vector-param module/functions should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "[]Number") {
		t.Fatalf("vector params not classified []Number:\n%s", res.Facet)
	}
	if !strings.Contains(res.Facet, "v1[0]") {
		t.Fatalf("expected v1[0] indexing in output:\n%s", res.Facet)
	}
	// Grouping must be preserved: (v1[0]-v2[0])*(v1[0]-v2[0]), not flattened.
	if !strings.Contains(res.Facet, "(v1[0] - v2[0]) * (v1[0] - v2[0])") {
		t.Fatalf("binary grouping not preserved:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// A whole-body ternary becomes an if/else-if/else chain that returns in every
// branch (the form Facet's conditional requires).
func TestTranspileTernaryFunction(t *testing.T) {
	src := "function clamp(x, lo, hi) = x < lo ? lo : (x > hi ? hi : x);\n" +
		"cube(clamp(5, 0, 3));\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("ternary function should transpile, got: %v", err)
	}
	// OpenSCAD's ?: maps directly to Facet's ternary (Facet now has one). The
	// formatter normalizes the redundant else-arm parens (right-associative).
	if !strings.Contains(res.Facet, "return x < lo ? lo : x > hi ? hi : x") {
		t.Fatalf("ternary not emitted as a Facet ternary:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// A ternary in assignment position (not the whole function body) is now a
// direct Facet ternary too — no var+if lowering needed.
func TestTranspileTernaryInAssignment(t *testing.T) {
	src := "module m(n) {\n" +
		"    s = n > 0 ? 2 : 4;\n" +
		"    cube(s);\n" +
		"}\n" +
		"m(1);\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("assignment-position ternary should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "const s = n > 0 ? 2 : 4") {
		t.Fatalf("assignment ternary not emitted directly:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// OpenSCAD search(match_value, vector) returns the LIST of matching indices, so
// it maps to IndicesOf(arr: vector, value: match_value) (the arguments swap).
// The list form keeps the idiomatic len(search(...)) / search(...)[0] working.
// The list parameter is classified []Number from being passed as search's list.
func TestTranspileSearch(t *testing.T) {
	src := "function find(xs, x) = len(search(x, xs)) > 0 ? search(x, xs)[0] : -1;\n" +
		"cube(find([1,2,3], 2));\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("search should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "IndicesOf(arr: xs, value: x)") {
		t.Fatalf("search not mapped to IndicesOf with swapped args:\n%s", res.Facet)
	}
	if !strings.Contains(res.Facet, "fn find(xs []Number, x Number)") {
		t.Fatalf("search's list parameter not classified []Number:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// The multi-argument search forms (num_returns_per_match, index_col_num) have no
// IndexOf equivalent and must error rather than silently drop the extra args.
func TestTranspileSearchMultiArgErrors(t *testing.T) {
	res, err := Transpile("function f(xs, x) = search(x, xs, 1);\ncube(f([1,2,3], 2));\n", "part.scad")
	if err == nil {
		t.Fatalf("3-argument search should error; output:\n%s", res.Facet)
	}
	if !strings.Contains(err.Error(), "search") {
		t.Fatalf("error should mention search: %v", err)
	}
}

// A nested array parameter (a list of lists) exceeds the binary scalar/vector
// model, so it's typed `Any` (dynamic). A directly double-indexed parameter
// (pts[0][1]) is the simplest signal.
func TestTranspileNestedArrayParam(t *testing.T) {
	res, err := Transpile("function second(pts) = pts[0][1];\ncube(second([[1,2],[3,4]]));\n", "part.scad")
	if err != nil {
		t.Fatalf("nested array param should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "fn second(pts Any)") {
		t.Fatalf("nested param not typed Any:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// Nestedness propagates across calls: passing an indexed parameter (rows[0]) to
// a vector parameter makes the base nested (Any), while the flat callee param
// stays []Number. This is the pattern icosphere uses (verts[i] -> getMiddlePoint).
func TestTranspileNestedInterProcedural(t *testing.T) {
	src := "function getX(p) = p[0];\n" +
		"function firstX(rows) = getX(rows[0]);\n" +
		"cube(firstX([[1,2],[3,4]]));\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("inter-procedural nesting should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "fn getX(p []Number)") {
		t.Fatalf("flat param should stay []Number:\n%s", res.Facet)
	}
	if !strings.Contains(res.Facet, "fn firstX(rows Any)") {
		t.Fatalf("base of an indexed-arg-to-vector should be Any:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// An accumulator parameter built by concat of a nested local is typed Any by
// the unified (scope-aware) nested inference. The local `s = seg(...)` returns a
// nested array (Any), so `nacc = concat(acc, s)` infers Any, and forwarding nacc
// to `acc` in the recursive call propagates Any to the parameter — a flow only a
// scope-aware pass (which tracks local-value types, not just parameters) can
// see. `items`/`i` stay precise, so Any is not over-applied.
func TestTranspileAccumulatorParamNested(t *testing.T) {
	src := "function seg(x) = [[x, x], [x, x]];\n" +
		"function accum(acc, items, i=0) =\n" +
		"  let (s = seg(items[i]), nacc = concat(acc, s))\n" +
		"  i >= len(items) - 1 ? nacc : accum(nacc, items, i + 1);\n" +
		"cube(accum([], [1, 2, 3]));\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("accumulator program should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "acc Any") {
		t.Fatalf("concat-accumulator parameter not typed Any:\n%s", res.Facet)
	}
	if !strings.Contains(res.Facet, "items []Number") {
		t.Fatalf("items should stay []Number (Any not over-applied):\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// A parameter passed to polyhedron's points/faces is a nested array
// ([][]Number), so it's typed Any. polyhedron is a built-in, handled separately
// from the user-function nesting propagation.
func TestTranspilePolyhedronParamNested(t *testing.T) {
	src := "module shell(pts, fcs) { polyhedron(pts, fcs); }\n" +
		"shell([[0,0,0],[1,0,0],[0,1,0],[0,0,1]], [[0,1,2],[0,2,3],[0,3,1],[1,3,2]]);\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("polyhedron-param module should transpile, got: %v", err)
	}
	// pts and fcs are independent Any slots and must stay separate: merging
	// them into `pts, fcs Any` would force both to one concrete type.
	if !strings.Contains(res.Facet, "fn shell(pts Any, fcs Any)") {
		t.Fatalf("polyhedron points/faces params not typed Any:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// A function returning a list of lists is typed Any (return-type nesting),
// mirroring the parameter side — needed for icosphere's addTris/recurseTris,
// which bundle (verts, tris) into a nested return.
func TestTranspileNestedReturnType(t *testing.T) {
	res, err := Transpile("function mk() = [[1,2],[3,4]];\ncube(mk()[0][0]);\n", "part.scad")
	if err != nil {
		t.Fatalf("nested return should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "fn mk() Any") {
		t.Fatalf("nested return not typed Any:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// A flat list return stays []Number — Any is not over-applied.
func TestTranspileFlatReturnStaysVector(t *testing.T) {
	res, err := Transpile("function flat() = [1,2,3];\ncube(flat()[0]);\n", "part.scad")
	if err != nil {
		t.Fatalf("flat return should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "fn flat() []Number") {
		t.Fatalf("flat return should stay []Number:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// polyhedron(points, faces) becomes a Mesh{...}.Solid(); points/faces flow
// through the scad_v3 and scad_faces helpers (the latter fan-triangulates).
func TestTranspilePolyhedron(t *testing.T) {
	src := "polyhedron(points=[[0,0,0],[1,0,0],[0,1,0],[0,0,1]], " +
		"faces=[[0,1,2],[0,2,3],[0,3,1],[1,3,2]]);\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("polyhedron should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "fn scad_v3(ps [][]Number) []Vec3") {
		t.Fatalf("expected scad_v3 helper:\n%s", res.Facet)
	}
	if !strings.Contains(res.Facet, "fn scad_faces(fs [][]Number) []Face") {
		t.Fatalf("expected scad_faces helper:\n%s", res.Facet)
	}
	// (The formatter may wrap long calls, so check the pieces, not exact spacing.)
	if !strings.Contains(res.Facet, "Mesh{vertices: scad_v3(") ||
		!strings.Contains(res.Facet, "scad_faces(") ||
		!strings.Contains(res.Facet, ".Solid()") {
		t.Fatalf("polyhedron not lowered to Mesh{...}.Solid():\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// polyhedron with points/faces held in variables routes the variables straight
// through the helpers (which accept any runtime [][]Number).
func TestTranspilePolyhedronVariables(t *testing.T) {
	src := "pts=[[0,0,0],[1,0,0],[0,1,0],[0,0,1]];\n" +
		"fcs=[[0,1,2],[0,2,3],[0,3,1],[1,3,2]];\n" +
		"polyhedron(pts, fcs);\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("polyhedron with variables should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "scad_v3(ps: pts)") || !strings.Contains(res.Facet, "scad_faces(fs: fcs)") {
		t.Fatalf("variable points/faces not passed through:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// OpenSCAD lets a local shadow a parameter (radius = radius/4). Facet has no
// shadowing, so the parameter is renamed and the reassignment becomes a const
// of the original name whose RHS references the renamed parameter.
func TestTranspileReassignedParam(t *testing.T) {
	src := "module ring(radius=6) { radius = radius/4; circle(r=radius); }\nring();\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("reassigned param should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "radius_arg Number = 6") {
		t.Fatalf("reassigned parameter not renamed:\n%s", res.Facet)
	}
	if !strings.Contains(res.Facet, "const radius = radius_arg / 4") {
		t.Fatalf("reassignment not lowered to a const of the original name:\n%s", res.Facet)
	}
	if !strings.Contains(res.Facet, "Circle(r: radius") {
		t.Fatalf("body should reference the const, not the renamed parameter:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// A module $fn parameter becomes the module-local segment count: it renames to
// scad_fn, scopes curved primitives in the body (circle gets segments: scad_fn),
// and a twisted linear_extrude derives its slices from $fn and the twist.
func TestTranspileModuleFnScoping(t *testing.T) {
	src := "module horn(twist=720, $fn=50) {\n" +
		"  linear_extrude(height=10, twist=twist, $fn=$fn) circle(r=2);\n" +
		"}\n" +
		"horn();\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("module $fn should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "scad_fn Number = 50") {
		t.Fatalf("$fn parameter not renamed to scad_fn:\n%s", res.Facet)
	}
	if !strings.Contains(res.Facet, "Circle(r: 2 mm, segments: scad_fn)") {
		t.Fatalf("circle did not inherit the module $fn:\n%s", res.Facet)
	}
	if !strings.Contains(res.Facet, "slices: Max(a: 1, b: Ceil(n: scad_fn * Abs(a: twist) / 360))") {
		t.Fatalf("extrude slices not derived from $fn and twist:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// A module that uses children() gains a `children []Solid` parameter;
// children(0) indexes it and the call site binds the passed geometry as an
// array.
func TestTranspileChildrenIndexed(t *testing.T) {
	src := "module twice() { children(0); translate([10,0,0]) children(0); }\n" +
		"twice() cube(2);\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("children(0) should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "fn twice(children []Solid)") {
		t.Fatalf("expected children []Solid parameter:\n%s", res.Facet)
	}
	if !strings.Contains(res.Facet, "children[0]") {
		t.Fatalf("children(0) not lowered to indexing:\n%s", res.Facet)
	}
	if !strings.Contains(res.Facet, "twice(children: [Cube(") {
		t.Fatalf("call site did not bind children array:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// children() with no index is the union of all passed children; multiple
// children at the call site are bound as a multi-element array.
func TestTranspileChildrenAll(t *testing.T) {
	src := "module wrap() { children(); }\nwrap() { cube(2); sphere(1); }\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("children() should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "Union(arr: children)") {
		t.Fatalf("children() not lowered to Union(arr:):\n%s", res.Facet)
	}
	if !strings.Contains(res.Facet, "Cube(") || !strings.Contains(res.Facet, "Sphere(") {
		t.Fatalf("both children not bound:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// Passing children to a module that does not use children() has no meaning and
// must error rather than silently drop the geometry.
func TestTranspileChildrenToNonChildrenModuleErrors(t *testing.T) {
	res, err := Transpile("module plain() { cube(2); }\nplain() sphere(1);\n", "part.scad")
	if err == nil {
		t.Fatalf("children to a non-children module should error; output:\n%s", res.Facet)
	}
	if !strings.Contains(err.Error(), "children") {
		t.Fatalf("error should mention children: %v", err)
	}
}

// A transform applied to a user module resolves the module's dimensionality
// from its body: a 2D module gets the 2D rotate form (Rotate(a:)), not the 3D
// form (Rotate(z:)) which a Sketch has no parameter for.
func TestTranspileUserModule2DDimensionality(t *testing.T) {
	src := "module shape() { square([2, 3]); }\n" +
		"rotate([0, 0, 45]) shape();\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("user-module rotate should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "Rotate(a: 45 deg)") {
		t.Fatalf("2D user module should use the 2D rotate form:\n%s", res.Facet)
	}
	// Would emit Rotate(z:) and fail type-check if dimensionality were unresolved.
	assertTypeChecks(t, res.Facet)
}

// OpenSCAD angles are plain degree-numbers; Facet has a distinct Angle type.
// Trig args convert to Angle (`* 1 deg` / `<n> deg`), inverse-trig results
// convert back to a degree-number (`Number(from: ...)`) so angle arithmetic
// stays in Number, and the geometry boundary (rotate) converts on the way out.
func TestTranspileAngleModel(t *testing.T) {
	src := "function bend(a, b) = atan2(b, a) + 90;\n" +
		"rotate([0, 0, bend(3, 4)]) cube([sin(30), 2, 2]);\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("angle model should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "Number(from: Atan2(y: b, x: a))") {
		t.Fatalf("atan2 result not converted to a degree-number:\n%s", res.Facet)
	}
	if !strings.Contains(res.Facet, "Sin(a: 30 deg)") {
		t.Fatalf("sin arg not converted to an Angle literal:\n%s", res.Facet)
	}
	if !strings.Contains(res.Facet, "* 1 deg") {
		t.Fatalf("rotate angle not converted to an Angle at the boundary:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// polygon(points) where points is a variable (a runtime [][]Number) converts
// to []Vec2 via the scad_v2 helper, which is emitted once and only when used.
func TestTranspilePolygonVariablePoints(t *testing.T) {
	src := "pts = [[0,0],[10,0],[0,10]];\n" +
		"linear_extrude(height=2) polygon(pts);\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("polygon with variable points should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "fn scad_v2(ps [][]Number) []Vec2") {
		t.Fatalf("expected scad_v2 helper:\n%s", res.Facet)
	}
	if !strings.Contains(res.Facet, "Polygon(points: scad_v2(ps: pts))") {
		t.Fatalf("expected points routed through scad_v2:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// The scad_v2 helper is emitted only when referenced: a literal-points polygon
// converts element-by-element and must not pull in the helper.
func TestTranspileLiteralPolygonNoHelper(t *testing.T) {
	res, err := Transpile("polygon([[0,0],[10,0],[0,10]]);\n", "part.scad")
	if err != nil {
		t.Fatalf("literal polygon should transpile, got: %v", err)
	}
	if strings.Contains(res.Facet, "scad_v2") {
		t.Fatalf("literal polygon should not emit scad_v2 helper:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// Variable points combined with paths (literal index lists) route each path
// through scad_v2_path, which indexes the runtime points at evaluation time.
// The indices stay literal — they are baked into the emitted call — and a
// single path collapses to Polygon(points: ...), matching the no-paths shape.
func TestTranspilePolygonVariablePointsSinglePath(t *testing.T) {
	src := "pts = [[0,0],[10,0],[0,10]];\npolygon(pts, [[0,1,2]]);\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("variable points with literal paths should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "fn scad_v2_path(ps [][]Number, indices []Number) []Vec2") {
		t.Fatalf("expected scad_v2_path helper:\n%s", res.Facet)
	}
	if !strings.Contains(res.Facet, "Polygon(points: scad_v2_path(ps: pts, indices: [0, 1, 2]))") {
		t.Fatalf("expected single-path polygon routed through scad_v2_path:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// A polygon with multiple paths emits the outer outline plus a typed `holes`
// list, each entry an independent scad_v2_path call against the same runtime
// points expression.
func TestTranspilePolygonVariablePointsMultiplePaths(t *testing.T) {
	src := "pts = [[0,0],[10,0],[10,10],[0,10],[3,3],[7,3],[7,7],[3,7]];\n" +
		"polygon(pts, [[0,1,2,3], [4,5,6,7]]);\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("variable points with multiple paths should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "scad_v2_path(ps: pts, indices: [0, 1, 2, 3])") {
		t.Fatalf("expected outer path call:\n%s", res.Facet)
	}
	if !strings.Contains(res.Facet, "scad_v2_path(ps: pts, indices: [4, 5, 6, 7])") {
		t.Fatalf("expected hole path call:\n%s", res.Facet)
	}
	if !strings.Contains(res.Facet, "holes: [scad_v2_path") {
		t.Fatalf("expected holes argument:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// A parameter never indexed in its own body, but forwarded to a helper whose
// parameter is a vector, is itself classified []Number (inter-procedural
// propagation), so the emitted call type-checks.
func TestTranspilePassThroughVectorParam(t *testing.T) {
	src := "function dist(a, b) = sqrt((a[0]-b[0])*(a[0]-b[0]) + (a[1]-b[1])*(a[1]-b[1]));\n" +
		"function midDist(p, q) = dist(p, q) / 2;\n" +
		"cube(midDist([0,0], [3,4]));\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("pass-through vector param should transpile, got: %v", err)
	}
	// p and q are never indexed in midDist, but flow into dist's vector params.
	// (The formatter groups same-typed adjacent params: `p, q []Number`.)
	if !strings.Contains(res.Facet, "fn midDist(p, q []Number)") {
		t.Fatalf("pass-through params not propagated to []Number:\n%s", res.Facet)
	}
	// The real guarantee: passing a scalar to dist's []Number params would fail
	// type-check, so this passing means both were classified []Number.
	assertTypeChecks(t, res.Facet)
}

// $t (OpenSCAD's animation clock) turns the program into a Facet Animation whose
// frame derives scad_t (0..1) from the wall clock. A $t-bearing parameter default
// can't live on a Facet parameter, so the parameter loses its default and the
// default is injected at each call site that omits the argument.
func TestTranspileAnimationTimeInDefault(t *testing.T) {
	src := "module spin(a = $t * 360) { rotate([0, 0, a]) cube(2); }\nspin();\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("$t should transpile, got: %v", err)
	}
	for _, want := range []string{
		"fn Main() Animation",
		"frame: fn(scad_ms Number) Solid",
		"const scad_t = scad_ms % 4000 / 4000",
		"fn spin(a Number) Solid", // the $t default is stripped from the parameter
		"spin(a: scad_t * 360)",   // and injected at the call site
	} {
		if !strings.Contains(res.Facet, want) {
			t.Fatalf("expected %q in:\n%s", want, res.Facet)
		}
	}
	assertTypeChecks(t, res.Facet)
}

// $t used directly in top-level geometry animates without any module threading:
// the frame computes scad_t and the body references it.
func TestTranspileAnimationTimeTopLevel(t *testing.T) {
	res, err := Transpile("rotate([0, 0, $t * 360]) cube(10);\n", "part.scad")
	if err != nil {
		t.Fatalf("$t should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "fn Main() Animation") {
		t.Fatalf("expected an Animation entry:\n%s", res.Facet)
	}
	if !strings.Contains(res.Facet, "Rotate(z: scad_t * 360 * 1 deg)") {
		t.Fatalf("expected $t in the frame body:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// A top-level variable that depends on $t is recomputed per frame: it moves
// inside the frame lambda (after scad_t), while $t-independent variables stay at
// module scope.
func TestTranspileAnimationTimeTopLevelVar(t *testing.T) {
	src := "size = 10;\nangle = $t * 360;\nrotate([0, 0, angle]) cube(size);\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("$t should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "const size = 10") {
		t.Fatalf("expected the $t-independent var at module scope:\n%s", res.Facet)
	}
	// angle depends on $t, so it is declared inside the frame, after scad_t.
	frame := res.Facet[strings.Index(res.Facet, "frame:"):]
	if !strings.Contains(frame, "const angle = scad_t * 360") {
		t.Fatalf("expected the $t-dependent var inside the frame:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// $t inside a module body threads scad_t through as a parameter, and call sites
// pass it down — mirroring how children() is threaded.
func TestTranspileAnimationTimeInBody(t *testing.T) {
	src := "module spin() { rotate([0, 0, $t * 360]) cube(2); }\nspin();\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("$t should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "fn spin(scad_t Number) Solid") {
		t.Fatalf("expected scad_t threaded into the module:\n%s", res.Facet)
	}
	if !strings.Contains(res.Facet, "spin(scad_t: scad_t)") {
		t.Fatalf("expected scad_t passed at the call site:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// $t inside a value function has no clean Facet form (a function can't receive
// the per-frame clock the way geometry does), so it is rejected rather than
// mistranslated.
func TestTranspileAnimationTimeInFunctionErrors(t *testing.T) {
	_, err := Transpile("function angle() = $t * 360;\nrotate([0, 0, angle()]) cube(2);\n", "part.scad")
	if err == nil {
		t.Fatal("expected an error for $t inside a function")
	}
	if !strings.Contains(err.Error(), "$t inside function") {
		t.Fatalf("expected a $t-in-function error, got: %v", err)
	}
}

// Animating 2D geometry would require the frame to return a Sketch, but an
// Animation frame returns a Solid, so a 2D model using $t is rejected.
func TestTranspileAnimationTime2DErrors(t *testing.T) {
	_, err := Transpile("rotate([0, 0, $t * 360]) square(10);\n", "part.scad")
	if err == nil {
		t.Fatal("expected an error animating 2D geometry")
	}
	if !strings.Contains(err.Error(), "requires 3D geometry") {
		t.Fatalf("expected a 3D-required error, got: %v", err)
	}
}

// $t appearing only inside a dropped assert()/echo() has no geometric effect:
// the construct is dropped, so the analysis must not thread scad_t for it (which
// would leave an undefined scad_t reference in Main). The model stays a static
// Solid.
func TestTranspileAnimationTimeInDroppedAssert(t *testing.T) {
	src := "module m() { assert($t < 1); cube(2); }\nm();\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("should transpile, got: %v", err)
	}
	if strings.Contains(res.Facet, "scad_t") {
		t.Fatalf("$t in a dropped assert must not thread scad_t:\n%s", res.Facet)
	}
	if !strings.Contains(res.Facet, "fn Main() Solid") {
		t.Fatalf("expected a static Solid (no animation):\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// A $t-bearing default that references another parameter cannot be injected at
// the call site — the other parameter is not in the caller's scope — so it is a
// hard error rather than an emitted undefined reference.
func TestTranspileAnimationTimeDefaultRefsParamErrors(t *testing.T) {
	src := "module spin(b, a = $t * b) { rotate([0, 0, a]) cube(b); }\nspin(5);\n"
	_, err := Transpile(src, "part.scad")
	if err == nil {
		t.Fatal("expected an error for a $t default referencing another parameter")
	}
	if !strings.Contains(err.Error(), "another parameter") {
		t.Fatalf("expected a default-references-parameter error, got: %v", err)
	}
}

// User identifiers may not collide with the transpiler's reserved scad_ prefix:
// scad_t/scad_ms drive the animation frame, and other scad_* names back emitted
// helpers, so a collision would silently shadow generated code and render wrong
// geometry. It is rejected at every binding site.
func TestTranspileReservedScadPrefixErrors(t *testing.T) {
	for _, src := range []string{
		"scad_t = 5;\nrotate([0, 0, $t * 360]) cube(scad_t);\n", // top-level var collides with the clock
		"module m(scad_t) { cube(scad_t); }\nm(5);\n",           // parameter
		"scad_ms = 3;\ncube(scad_ms);\n",                        // the frame parameter name
		"module scad_box() { cube(2); }\nscad_box();\n",         // a module name
	} {
		_, err := Transpile(src, "part.scad")
		if err == nil {
			t.Fatalf("expected a reserved-prefix error for:\n%s", src)
		}
		if !strings.Contains(err.Error(), "reserved") {
			t.Fatalf("expected a reserved scad_ error, got: %v\nsrc:\n%s", err, src)
		}
	}
}

// An unsupported special variable (e.g. $vpr, the viewport rotation) has no
// Facet meaning and must error rather than emit an invalid `$`-prefixed name.
func TestTranspileUnsupportedSpecialVarErrors(t *testing.T) {
	res, err := Transpile("cube($vpr);\n", "part.scad")
	if err == nil {
		t.Fatalf("$vpr should error; output:\n%s", res.Facet)
	}
	if !strings.Contains(err.Error(), "$vpr") {
		t.Fatalf("error should name the special variable: %v", err)
	}
}

// A geometry if/else-if/else in a module body becomes a return-bearing Facet
// conditional: each branch returns its own geometry, and `else if` folds.
func TestTranspileGeometryIf(t *testing.T) {
	src := "module pick(n) {\n" +
		"    if (n > 2) cube(3);\n" +
		"    else if (n > 0) cube(2);\n" +
		"    else sphere(1);\n" +
		"}\n" +
		"pick(1);\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("geometry if should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "if n > 2 {") {
		t.Fatalf("expected if-condition:\n%s", res.Facet)
	}
	if !strings.Contains(res.Facet, "else if n > 0 {") {
		t.Fatalf("expected else-if branch:\n%s", res.Facet)
	}
	if !strings.Contains(res.Facet, "} else {") {
		t.Fatalf("expected final else branch:\n%s", res.Facet)
	}
	if strings.Count(res.Facet, "return") < 4 { // 3 branches + Main
		t.Fatalf("each branch must return:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// A geometry `if` without an `else` leaves a path that returns nothing, so it
// must error rather than emit a fn that does not return on every path.
func TestTranspileGeometryIfWithoutElseErrors(t *testing.T) {
	res, err := Transpile("module m(n) { if (n > 0) cube(2); }\nm(1);\n", "part.scad")
	if err == nil {
		t.Fatalf("if without else should error; output:\n%s", res.Facet)
	}
	if !strings.Contains(err.Error(), "else") {
		t.Fatalf("error should mention the missing else: %v", err)
	}
}

// OpenSCAD numeric truthiness `if (n)` (n a Number) becomes `if n != 0`; a Bool
// local is used directly. The type is resolved from the definition's scope.
func TestTranspileNumericTruthiness(t *testing.T) {
	src := "module m(n) {\n" +
		"    flag = n > 5;\n" +
		"    if (n) cube(2);\n" + // n is a Number param → n != 0
		"    else if (flag) cube(3);\n" + // flag is a Bool local → used directly
		"    else sphere(1);\n" +
		"}\n" +
		"m(1);\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("numeric truthiness should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "if n != 0") {
		t.Fatalf("numeric condition not converted to `!= 0`:\n%s", res.Facet)
	}
	if !strings.Contains(res.Facet, "else if flag {") {
		t.Fatalf("Bool local condition should be used directly:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// OpenSCAD concat(a, b, ...) of lists maps to Facet's list + (which flattens).
func TestTranspileConcat(t *testing.T) {
	res, err := Transpile("function f() = concat([1,2], [3,4], [5]);\ncube(f()[0]);\n", "part.scad")
	if err != nil {
		t.Fatalf("concat should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "[1, 2] + [3, 4] + [5]") {
		t.Fatalf("concat not mapped to + :\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}
