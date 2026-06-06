package scad

import (
	"strings"
	"testing"
)

// An `include <BOSL2/std.scad>` is a gate, not a file to resolve: it is dropped
// from the output and the rest of the file transpiles normally. (Here the body
// is a plain OpenSCAD primitive, isolating the gate from BOSL2 vocabulary.)
func TestBOSL2_IncludeIsDropped(t *testing.T) {
	src := "include <BOSL2/std.scad>\ncube(10);\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("BOSL2 include should transpile, got: %v", err)
	}
	if strings.Contains(res.Facet, "include") || strings.Contains(res.Facet, "BOSL2") {
		t.Fatalf("include line should be dropped from output:\n%s", res.Facet)
	}
	if !strings.Contains(res.Facet, "Cube(") {
		t.Fatalf("body should still transpile:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// `use <BOSL2/std.scad>` is gated the same way as include.
func TestBOSL2_UseIsDropped(t *testing.T) {
	src := "use <BOSL2/std.scad>\ncube(10);\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("BOSL2 use should transpile, got: %v", err)
	}
	if strings.Contains(res.Facet, "BOSL2") {
		t.Fatalf("use line should be dropped:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// BOSL2's cuboid is centered on the origin by default (anchor=CENTER), unlike
// OpenSCAD's corner-origin cube. A vector size maps to a centered Cube.
func TestBOSL2_CuboidVector(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\ncuboid([20, 10, 5]);\n", "part.scad")
	if err != nil {
		t.Fatalf("cuboid should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "Cube(x: 20 mm, y: 10 mm, z: 5 mm).AlignCenter(pos: Vec3{})") {
		t.Fatalf("expected centered Cube:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// A scalar cuboid size builds a centered cube of equal sides.
func TestBOSL2_CuboidScalar(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\ncuboid(8);\n", "part.scad")
	if err != nil {
		t.Fatalf("cuboid scalar should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "Cube(s: 8 mm).AlignCenter(pos: Vec3{})") {
		t.Fatalf("expected centered cube:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// BOSL2's cyl is centered on the origin in every axis (anchor=CENTER), unlike
// OpenSCAD's cylinder which is centered only in X/Y.
func TestBOSL2_Cyl(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\ncyl(h=10, r=4);\n", "part.scad")
	if err != nil {
		t.Fatalf("cyl should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "Cylinder(r: 4 mm, h: 10 mm") || !strings.Contains(res.Facet, ".AlignCenter(pos: Vec3{})") {
		t.Fatalf("expected centered Cylinder:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// up/down/right/left/back/fwd are single-axis translations. up is +Z, down is
// -Z, fwd is -Y — the child geometry gets a trailing .Move on that axis.
func TestBOSL2_AxisMoves(t *testing.T) {
	cases := []struct {
		src  string
		want string
	}{
		{"up(5) cuboid(2);", ".Move(z: 5 mm)"},
		{"down(5) cuboid(2);", ".Move(z: -5 mm)"},
		{"right(3) cuboid(2);", ".Move(x: 3 mm)"},
		{"left(3) cuboid(2);", ".Move(x: -3 mm)"},
		{"back(4) cuboid(2);", ".Move(y: 4 mm)"},
		{"fwd(4) cuboid(2);", ".Move(y: -4 mm)"},
	}
	for _, c := range cases {
		res, err := Transpile("include <BOSL2/std.scad>\n"+c.src+"\n", "part.scad")
		if err != nil {
			t.Fatalf("%q should transpile, got: %v", c.src, err)
		}
		if !strings.Contains(res.Facet, c.want) {
			t.Fatalf("%q: expected %q in:\n%s", c.src, c.want, res.Facet)
		}
		assertTypeChecks(t, res.Facet)
	}
}

// move([x,y,z]) is BOSL2's vector translation — the same mapping as OpenSCAD's
// translate.
func TestBOSL2_Move(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\nmove([1, 2, 3]) cuboid(2);\n", "part.scad")
	if err != nil {
		t.Fatalf("move should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, ".Move(x: 1 mm, y: 2 mm, z: 3 mm)") {
		t.Fatalf("expected vector move:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// xrot/yrot/zrot rotate the child about a single axis by a degree angle.
func TestBOSL2_AxisRotations(t *testing.T) {
	cases := []struct {
		src  string
		want string
	}{
		{"xrot(90) cuboid(2);", ".Rotate(x: 90 deg)"},
		{"yrot(45) cuboid(2);", ".Rotate(y: 45 deg)"},
		{"zrot(30) cuboid(2);", ".Rotate(z: 30 deg)"},
	}
	for _, c := range cases {
		res, err := Transpile("include <BOSL2/std.scad>\n"+c.src+"\n", "part.scad")
		if err != nil {
			t.Fatalf("%q should transpile, got: %v", c.src, err)
		}
		if !strings.Contains(res.Facet, c.want) {
			t.Fatalf("%q: expected %q in:\n%s", c.src, c.want, res.Facet)
		}
		assertTypeChecks(t, res.Facet)
	}
}

// position(anchor) on a box parent emits a B2 chain into the attachment runtime:
// the parent becomes a b2_cuboid and the child a positioned b2_cyl, with the
// anchor resolved to a B2Anchor literal (TOP -> z:1). The geometry math lives in
// the runtime; the runtime typecheck (emit package) covers its correctness.
func TestBOSL2_PositionTop(t *testing.T) {
	src := "include <BOSL2/std.scad>\ncuboid([20, 20, 10]) position(TOP) cyl(h=4, r=2);\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("position(TOP) should transpile, got: %v", err)
	}
	// Fragments are checked individually because the formatter may wrap a long
	// call across lines.
	for _, want := range []string{
		"b2_cuboid(size: Vec3{x: 20 mm, y: 20 mm, z: 10 mm})",
		".position(",
		"a: B2Anchor{x: 0, y: 0, z: 1}",
		"child: b2_cyl(h: 4 mm, r: 2 mm)",
		".Solid()",
		"type B2", // runtime preamble injected
	} {
		if !strings.Contains(res.Facet, want) {
			t.Fatalf("expected %q in:\n%s", want, res.Facet)
		}
	}
	assertTypeChecks(t, res.Facet)
}

// A combined anchor (RIGHT+TOP) sums its component directions into one B2Anchor.
func TestBOSL2_PositionCombinedAnchor(t *testing.T) {
	src := "include <BOSL2/std.scad>\ncuboid([20, 20, 10]) position(RIGHT+TOP) cyl(h=4, r=2);\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("position(RIGHT+TOP) should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "a: B2Anchor{x: 1, y: 0, z: 1}") {
		t.Fatalf("expected combined anchor B2Anchor{x:1,y:0,z:1} in:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// attach(TOP, BOTTOM) mates the child's BOTTOM onto the parent's TOP with no
// reorientation, emitted as a B2 .attach with both anchors resolved.
func TestBOSL2_AttachTopBottom(t *testing.T) {
	src := "include <BOSL2/std.scad>\ncuboid([20, 20, 10]) attach(TOP, BOTTOM) cyl(h=8, r=3);\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("attach(TOP, BOTTOM) should transpile, got: %v", err)
	}
	for _, want := range []string{".attach(", "pa: B2Anchor{x: 0, y: 0, z: 1}", "ca: B2Anchor{x: 0, y: 0, z: -1}", "child: b2_cyl(h: 8 mm, r: 3 mm)"} {
		if !strings.Contains(res.Facet, want) {
			t.Fatalf("expected %q in:\n%s", want, res.Facet)
		}
	}
	assertTypeChecks(t, res.Facet)
}

// The single-anchor attach(P) reorients the child to point out of the parent's
// P face (its +Z axis is rotated to P). For TOP that rotation is the identity,
// so it still stacks on top — emitted as a B2 attachReorient.
func TestBOSL2_AttachTopShorthand(t *testing.T) {
	src := "include <BOSL2/std.scad>\ncuboid([20, 20, 10]) attach(TOP) cyl(h=8, r=3);\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("attach(TOP) should transpile, got: %v", err)
	}
	for _, want := range []string{".attachReorient(", "pa: B2Anchor{x: 0, y: 0, z: 1}", "child: b2_cyl(h: 8 mm, r: 3 mm)"} {
		if !strings.Contains(res.Facet, want) {
			t.Fatalf("expected %q (shorthand) in:\n%s", want, res.Facet)
		}
	}
	assertTypeChecks(t, res.Facet)
}

// attach(RIGHT) reorients the child so its axis points out the +X face — the
// case the no-reorientation path couldn't express. Emitted as attachReorient
// with the parent anchor RIGHT; the runtime applies the +Z->+X rotation.
func TestBOSL2_AttachReorientRight(t *testing.T) {
	src := "include <BOSL2/std.scad>\ncuboid([20, 20, 20]) attach(RIGHT) cyl(h=10, r=2);\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("attach(RIGHT) should transpile, got: %v", err)
	}
	for _, want := range []string{".attachReorient(", "pa: B2Anchor{x: 1, y: 0, z: 0}", "child: b2_cyl(h: 10 mm, r: 2 mm)"} {
		if !strings.Contains(res.Facet, want) {
			t.Fatalf("expected %q in:\n%s", want, res.Facet)
		}
	}
	assertTypeChecks(t, res.Facet)
}

// A single-anchor attach with a combined (non-axis) anchor has no single
// out-pointing direction, so it is a located error rather than a wrong guess.
func TestBOSL2_AttachReorientCombinedErrors(t *testing.T) {
	src := "include <BOSL2/std.scad>\ncuboid([20, 20, 20]) attach(RIGHT+TOP) cyl(h=10, r=2);\n"
	res, err := Transpile(src, "part.scad")
	if err == nil {
		t.Fatalf("1-arg attach with a combined anchor should error, got:\n%s", res.Facet)
	}
	if !strings.Contains(err.Error(), "attach") {
		t.Fatalf("error should mention attach, got: %v", err)
	}
}

// An attach that needs the child reoriented (child anchor not anti-parallel to
// the parent anchor) is not yet supported — a located error, never a wrong
// silent emission (no fallbacks).
func TestBOSL2_AttachReorientErrors(t *testing.T) {
	src := "include <BOSL2/std.scad>\ncuboid([20, 20, 10]) attach(RIGHT, BOTTOM) cyl(h=8, r=3);\n"
	res, err := Transpile(src, "part.scad")
	if err == nil {
		t.Fatalf("reorienting attach should error, got:\n%s", res.Facet)
	}
	if res.Facet != "" {
		t.Fatalf("expected no output on error, got:\n%s", res.Facet)
	}
	if !strings.Contains(err.Error(), "attach") {
		t.Fatalf("error should mention attach, got: %v", err)
	}
}

// Attachments on a shape that isn't a supported attachment parent must error,
// not be silently dropped — the child carries geometry the user expects to see.
func TestBOSL2_AttachUnsupportedParentErrors(t *testing.T) {
	src := "include <BOSL2/std.scad>\nsphere(5) attach(TOP) cube(2);\n"
	res, err := Transpile(src, "part.scad")
	if err == nil {
		t.Fatalf("attachment on unsupported parent should error, got:\n%s", res.Facet)
	}
	if res.Facet != "" {
		t.Fatalf("expected no output on error, got:\n%s", res.Facet)
	}
	if !strings.Contains(err.Error(), "attach") {
		t.Fatalf("error should mention the attachment, got: %v", err)
	}
}

// A position/attach with no enclosing shape is invalid — a located error with a
// clear message rather than the generic "unknown module" error.
func TestBOSL2_TopLevelAttachErrors(t *testing.T) {
	src := "include <BOSL2/std.scad>\nattach(TOP) cuboid(2);\n"
	_, err := Transpile(src, "part.scad")
	if err == nil {
		t.Fatalf("top-level attach should error")
	}
	if !strings.Contains(err.Error(), "attachable parent") {
		t.Fatalf("error should explain the missing parent, got: %v", err)
	}
}

// A BOSL2 construct we haven't implemented (here prismoid) is a located error
// with no output — never a placeholder or silent drop (no fallbacks).
func TestBOSL2_UnsupportedConstructErrors(t *testing.T) {
	src := "include <BOSL2/std.scad>\nprismoid(size1=[10, 10], size2=[5, 5], h=8);\n"
	res, err := Transpile(src, "part.scad")
	if err == nil {
		t.Fatalf("unsupported BOSL2 construct should error, got:\n%s", res.Facet)
	}
	if res.Facet != "" {
		t.Fatalf("expected no output on error, got:\n%s", res.Facet)
	}
	if !strings.Contains(err.Error(), "prismoid") || !strings.Contains(err.Error(), "2:1") {
		t.Fatalf("error should name the construct and location, got: %v", err)
	}
}

// A non-BOSL2 include cannot be resolved (we only special-case BOSL2), so it is
// a located error with no output — never a silent drop (no fallbacks).
func TestBOSL2_NonBOSL2IncludeErrors(t *testing.T) {
	src := "include <myhelpers.scad>\ncube(10);\n"
	res, err := Transpile(src, "part.scad")
	if err == nil {
		t.Fatalf("non-BOSL2 include should error, got output:\n%s", res.Facet)
	}
	if res.Facet != "" {
		t.Fatalf("expected no output on error, got:\n%s", res.Facet)
	}
	if !strings.Contains(err.Error(), "myhelpers.scad") || !strings.Contains(err.Error(), "1:1") {
		t.Fatalf("error should name the path and location, got: %v", err)
	}
}
