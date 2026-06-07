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

// A BOSL2 construct we haven't implemented (here teardrop) is a located error
// with no output — never a placeholder or silent drop (no fallbacks).
func TestBOSL2_UnsupportedConstructErrors(t *testing.T) {
	src := "include <BOSL2/std.scad>\nteardrop(r=5, h=10);\n"
	res, err := Transpile(src, "part.scad")
	if err == nil {
		t.Fatalf("unsupported BOSL2 construct should error, got:\n%s", res.Facet)
	}
	if res.Facet != "" {
		t.Fatalf("expected no output on error, got:\n%s", res.Facet)
	}
	if !strings.Contains(err.Error(), "teardrop") || !strings.Contains(err.Error(), "2:1") {
		t.Fatalf("error should name the construct and location, got: %v", err)
	}
}

// xcopies(spacing, n) makes n copies of the child along X, centered on the
// origin, emitted as a unioned for-comprehension with a per-copy Move.
func TestBOSL2_Xcopies(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\nxcopies(10, 3) cuboid(2);\n", "part.scad")
	if err != nil {
		t.Fatalf("xcopies should transpile, got: %v", err)
	}
	for _, want := range []string{"Union(", "[0:3 - 1]", ".Move(x:", "(3 - 1) / 2) * 10 mm"} {
		if !strings.Contains(res.Facet, want) {
			t.Fatalf("expected %q in:\n%s", want, res.Facet)
		}
	}
	assertTypeChecks(t, res.Facet)
}

// ycopies/zcopies distribute along Y/Z; the count defaults to 2 when n is
// omitted, and a total length `l` is an alternative to spacing.
func TestBOSL2_CopiesAxisAndDefaults(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\nzcopies(5) cuboid(2);\n", "part.scad")
	if err != nil {
		t.Fatalf("zcopies should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, ".Move(z:") || !strings.Contains(res.Facet, "[0:2 - 1]") {
		t.Fatalf("expected z-axis copies with default n=2 in:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)

	res, err = Transpile("include <BOSL2/std.scad>\nycopies(n=4, l=30) cuboid(2);\n", "part.scad")
	if err != nil {
		t.Fatalf("ycopies(n,l) should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, ".Move(y:") || !strings.Contains(res.Facet, "30 mm") {
		t.Fatalf("expected y-axis copies spaced over l=30 in:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// tube is a hollow cylinder: an outer cylinder minus an inner one, both
// centered. od/id give the outer/inner diameters (or/ir give radii).
func TestBOSL2_TubeDiameters(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\ntube(h=10, od=8, id=4);\n", "part.scad")
	if err != nil {
		t.Fatalf("tube should transpile, got: %v", err)
	}
	// Fragments checked individually — the formatter may wrap each Cylinder call.
	for _, want := range []string{"Cylinder(", "r: 8 mm / 2", "r: 4 mm / 2", " - "} {
		if !strings.Contains(res.Facet, want) {
			t.Fatalf("expected %q in:\n%s", want, res.Facet)
		}
	}
	assertTypeChecks(t, res.Facet)
}

// tube with or/ir radii subtracts inner from outer at those radii.
func TestBOSL2_TubeRadii(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\ntube(h=6, or=5, ir=3);\n", "part.scad")
	if err != nil {
		t.Fatalf("tube(or,ir) should transpile, got: %v", err)
	}
	for _, want := range []string{"r: 5 mm", "r: 3 mm", "h: 6 mm", " - "} {
		if !strings.Contains(res.Facet, want) {
			t.Fatalf("expected %q in:\n%s", want, res.Facet)
		}
	}
	assertTypeChecks(t, res.Facet)
}

// rot(a) is BOSL2's general rotation: a scalar spins about Z, a vector is an
// euler rotation (same mapping as OpenSCAD rotate).
func TestBOSL2_Rot(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\nrot(45) cuboid(2);\n", "part.scad")
	if err != nil {
		t.Fatalf("rot(scalar) should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, ".Rotate(z: 45 deg)") {
		t.Fatalf("expected scalar rot about Z in:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)

	res, err = Transpile("include <BOSL2/std.scad>\nrot([90, 0, 45]) cuboid(2);\n", "part.scad")
	if err != nil {
		t.Fatalf("rot(vector) should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "x: 90 deg") || !strings.Contains(res.Facet, "z: 45 deg") {
		t.Fatalf("expected euler rot in:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// zrot_copies(n) makes n copies evenly rotated about Z; with r, the copies sit
// on a circle of that radius (moved out, then rotated).
func TestBOSL2_ZrotCopies(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\nzrot_copies(n=4) cuboid([2, 6, 2]);\n", "part.scad")
	if err != nil {
		t.Fatalf("zrot_copies(n) should transpile, got: %v", err)
	}
	for _, want := range []string{"Union(", "[0:4 - 1]", ".Rotate(z:", "* 360 / 4"} {
		if !strings.Contains(res.Facet, want) {
			t.Fatalf("expected %q in:\n%s", want, res.Facet)
		}
	}
	assertTypeChecks(t, res.Facet)

	res, err = Transpile("include <BOSL2/std.scad>\nzrot_copies(n=6, r=20) cyl(h=5, r=1);\n", "part.scad")
	if err != nil {
		t.Fatalf("zrot_copies(n,r) should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, ".Move(x: 20 mm)") || !strings.Contains(res.Facet, ".Rotate(z:") {
		t.Fatalf("expected radial copies (Move then Rotate) in:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// rect_tube is a hollow rectangular tube: an outer box minus an inner one, both
// centered. The inner footprint comes from isize, or from size minus 2*wall.
func TestBOSL2_RectTube(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\nrect_tube(h=10, size=[20, 16], isize=[14, 10]);\n", "part.scad")
	if err != nil {
		t.Fatalf("rect_tube(isize) should transpile, got: %v", err)
	}
	for _, want := range []string{"Cube(", "x: 20 mm", "y: 16 mm", "x: 14 mm", "y: 10 mm", " - "} {
		if !strings.Contains(res.Facet, want) {
			t.Fatalf("expected %q in:\n%s", want, res.Facet)
		}
	}
	assertTypeChecks(t, res.Facet)

	res, err = Transpile("include <BOSL2/std.scad>\nrect_tube(h=8, size=16, wall=2);\n", "part.scad")
	if err != nil {
		t.Fatalf("rect_tube(wall) should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "16 mm - 2 *") {
		t.Fatalf("expected inner = size - 2*wall in:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// xflip_copy/yflip_copy/zflip_copy keep the child and add a copy mirrored
// across the corresponding plane (normal X/Y/Z).
func TestBOSL2_FlipCopies(t *testing.T) {
	cases := []struct{ name, mirror string }{
		{"xflip_copy", ".Mirror(x: 1)"},
		{"yflip_copy", ".Mirror(y: 1)"},
		{"zflip_copy", ".Mirror(z: 1)"},
	}
	for _, c := range cases {
		res, err := Transpile("include <BOSL2/std.scad>\n"+c.name+"() cuboid([2, 4, 6]);\n", "part.scad")
		if err != nil {
			t.Fatalf("%s should transpile, got: %v", c.name, err)
		}
		if !strings.Contains(res.Facet, c.mirror) || !strings.Contains(res.Facet, " + ") {
			t.Fatalf("%s: expected child + %s in:\n%s", c.name, c.mirror, res.Facet)
		}
		assertTypeChecks(t, res.Facet)
	}
}

// mirror_copy(v) keeps the child and adds a copy mirrored across the plane with
// normal v.
func TestBOSL2_MirrorCopy(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\nmirror_copy([1, 1, 0]) cuboid([2, 4, 6]);\n", "part.scad")
	if err != nil {
		t.Fatalf("mirror_copy should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, ".Mirror(x: 1, y: 1)") || !strings.Contains(res.Facet, " + ") {
		t.Fatalf("expected mirror across [1,1,0] in:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// grid_copies(spacing, n) tiles the child in an NxM grid on XY, centered, as a
// nested for-comprehension. Scalar spacing/n apply to both axes; vectors give
// per-axis values.
func TestBOSL2_GridCopies(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\ngrid_copies(spacing=10, n=3) cuboid(2);\n", "part.scad")
	if err != nil {
		t.Fatalf("grid_copies should transpile, got: %v", err)
	}
	for _, want := range []string{"Union(", ".Move(", "y: (scad_i1", "(3 - 1) / 2) * 10 mm"} {
		if !strings.Contains(res.Facet, want) {
			t.Fatalf("expected %q in:\n%s", want, res.Facet)
		}
	}
	// Two distinct loop ranges for the 3x3 grid.
	if strings.Count(res.Facet, "[0:3 - 1]") < 2 {
		t.Fatalf("expected two grid iterators in:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)

	res, err = Transpile("include <BOSL2/std.scad>\ngrid_copies(spacing=[10, 20], n=[2, 3]) cuboid(2);\n", "part.scad")
	if err != nil {
		t.Fatalf("grid_copies(vectors) should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "[0:2 - 1]") || !strings.Contains(res.Facet, "[0:3 - 1]") {
		t.Fatalf("expected per-axis counts 2 and 3 in:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// zcyl/xcyl/ycyl are cylinders oriented along Z/X/Y, centered. zcyl is the
// default (no rotation); xcyl spins the Z-axis cylinder onto X, ycyl onto Y.
func TestBOSL2_OrientedCylinders(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\nzcyl(h=10, r=2);\n", "part.scad")
	if err != nil {
		t.Fatalf("zcyl should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "Cylinder(r: 2 mm, h: 10 mm") || strings.Contains(res.Facet, ".Rotate(") {
		t.Fatalf("zcyl should be an unrotated centered cylinder:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)

	res, err = Transpile("include <BOSL2/std.scad>\nxcyl(h=10, r=2);\n", "part.scad")
	if err != nil {
		t.Fatalf("xcyl should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, ".Rotate(y: 90 deg)") {
		t.Fatalf("xcyl should rotate the cylinder onto X:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)

	res, err = Transpile("include <BOSL2/std.scad>\nycyl(h=10, d=4);\n", "part.scad")
	if err != nil {
		t.Fatalf("ycyl should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, ".Rotate(x: -90 deg)") {
		t.Fatalf("ycyl should rotate the cylinder onto Y:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// mainBody returns the `fn Main` portion of transpiled output, excluding the
// injected runtime preamble (whose definitions name the *Remove methods).
func mainBody(t *testing.T, facet string) string {
	t.Helper()
	i := strings.Index(facet, "fn Main(")
	if i < 0 {
		t.Fatalf("no Main in output:\n%s", facet)
	}
	return facet[i:]
}

// diff() with a tag("remove") plain child subtracts that child from the parent
// — a through-hole drilled at the center.
func TestBOSL2_DiffRemovePlain(t *testing.T) {
	src := "include <BOSL2/std.scad>\ndiff() cuboid([20, 20, 20]) tag(\"remove\") cyl(h=30, d=6);\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("diff+tag should transpile, got: %v", err)
	}
	if !strings.Contains(mainBody(t, res.Facet), ".positionRemove(") {
		t.Fatalf("expected a subtractive positionRemove in:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// diff() with a tag("remove") attach child subtracts the attached shape (a
// pocket on a face) — emitted via attachReorientRemove for a 1-anchor attach.
func TestBOSL2_DiffRemoveAttach(t *testing.T) {
	src := "include <BOSL2/std.scad>\ndiff() cuboid([20, 20, 20]) attach(TOP) tag(\"remove\") cyl(h=10, d=6);\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("diff+attach+tag should transpile, got: %v", err)
	}
	if !strings.Contains(mainBody(t, res.Facet), ".attachReorientRemove(") {
		t.Fatalf("expected a subtractive attachReorientRemove in:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// Without diff(), a tag("remove") is inert: the child is unioned normally (the
// Main body uses .position, not the subtractive form).
func TestBOSL2_TagWithoutDiffIsInert(t *testing.T) {
	src := "include <BOSL2/std.scad>\ncuboid([20, 20, 20]) tag(\"remove\") cyl(h=30, d=6);\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("tag without diff should transpile, got: %v", err)
	}
	body := mainBody(t, res.Facet)
	if strings.Contains(body, "Remove(") {
		t.Fatalf("tag outside diff() must not subtract:\n%s", res.Facet)
	}
	if !strings.Contains(body, ".position(") {
		t.Fatalf("expected a normal union .position in:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// xscale/yscale/zscale scale the child along a single axis (the others by 1).
func TestBOSL2_AxisScales(t *testing.T) {
	cases := []struct{ name, want string }{
		{"xscale", ".Scale(x: 2, y: 1, z: 1)"},
		{"yscale", ".Scale(x: 1, y: 2, z: 1)"},
		{"zscale", ".Scale(x: 1, y: 1, z: 2)"},
	}
	for _, c := range cases {
		res, err := Transpile("include <BOSL2/std.scad>\n"+c.name+"(2) cuboid(10);\n", "part.scad")
		if err != nil {
			t.Fatalf("%s should transpile, got: %v", c.name, err)
		}
		if !strings.Contains(res.Facet, c.want) {
			t.Fatalf("%s: expected %q in:\n%s", c.name, c.want, res.Facet)
		}
		assertTypeChecks(t, res.Facet)
	}
}

// rect([x,y]) is BOSL2's 2D rectangle, centered on the origin (a Sketch). The
// result type must be Sketch — assertTypeChecks confirms the dimensionality.
func TestBOSL2_Rect(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\nrect([20, 10]);\n", "part.scad")
	if err != nil {
		t.Fatalf("rect should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "fn Main() Sketch") {
		t.Fatalf("rect should yield a Sketch:\n%s", res.Facet)
	}
	if !strings.Contains(res.Facet, "Square(x: 20 mm, y: 10 mm)") {
		t.Fatalf("expected centered square in:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// rect extruded becomes a centered solid, exercising the 2D-under-extrude path.
func TestBOSL2_RectExtruded(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\nlinear_extrude(3) rect(8);\n", "part.scad")
	if err != nil {
		t.Fatalf("extruded rect should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "fn Main() Solid") {
		t.Fatalf("extruded rect should yield a Solid:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// regular_ngon(n, r) is a centered regular polygon (a Sketch), vertices at
// a = 360 - i*360/n from +X (BOSL2's default orientation).
func TestBOSL2_RegularNgon(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\nregular_ngon(n=6, r=10);\n", "part.scad")
	if err != nil {
		t.Fatalf("regular_ngon should transpile, got: %v", err)
	}
	for _, want := range []string{"fn Main() Sketch", "Polygon(", "points: for scad_i", "Cos(a:", "Sin(a:", "360 / 6"} {
		if !strings.Contains(res.Facet, want) {
			t.Fatalf("expected %q in:\n%s", want, res.Facet)
		}
	}
	assertTypeChecks(t, res.Facet)
}

// hexagon/pentagon/octagon are regular_ngon with a fixed side count.
func TestBOSL2_NamedNgons(t *testing.T) {
	cases := []struct{ name, sides string }{
		{"hexagon", "360 / 6"},
		{"pentagon", "360 / 5"},
		{"octagon", "360 / 8"},
	}
	for _, c := range cases {
		res, err := Transpile("include <BOSL2/std.scad>\n"+c.name+"(r=8);\n", "part.scad")
		if err != nil {
			t.Fatalf("%s should transpile, got: %v", c.name, err)
		}
		if !strings.Contains(res.Facet, c.sides) || !strings.Contains(res.Facet, "fn Main() Sketch") {
			t.Fatalf("%s: expected %q (2D ngon) in:\n%s", c.name, c.sides, res.Facet)
		}
		assertTypeChecks(t, res.Facet)
	}
}

// torus revolves a minor-radius circle, offset by the major radius, around Z.
func TestBOSL2_Torus(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\ntorus(r_maj=20, r_min=3);\n", "part.scad")
	if err != nil {
		t.Fatalf("torus should transpile, got: %v", err)
	}
	for _, want := range []string{"Circle(r: 3 mm)", ".Revolve()", "20 mm - 3 mm"} {
		if !strings.Contains(res.Facet, want) {
			t.Fatalf("expected %q in:\n%s", want, res.Facet)
		}
	}
	assertTypeChecks(t, res.Facet)
}

// torus also accepts outer/inner radii: r_maj=(or+ir)/2, r_min=(or-ir)/2.
func TestBOSL2_TorusOuterInner(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\ntorus(or=23, ir=17);\n", "part.scad")
	if err != nil {
		t.Fatalf("torus(or,ir) should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "(23 mm - 17 mm) / 2") || !strings.Contains(res.Facet, ".Revolve()") {
		t.Fatalf("expected or/ir-derived radii in:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// prismoid lofts between a bottom and top rectangle, centered, giving a tapered
// box.
func TestBOSL2_Prismoid(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\nprismoid(size1=[20, 20], size2=[10, 10], h=15);\n", "part.scad")
	if err != nil {
		t.Fatalf("prismoid should transpile, got: %v", err)
	}
	for _, want := range []string{"Loft(", "Square(x: 20 mm", "Square(x: 10 mm", "heights: [0 mm, 15 mm]", ".AlignCenter(pos: Vec3{})"} {
		if !strings.Contains(res.Facet, want) {
			t.Fatalf("expected %q in:\n%s", want, res.Facet)
		}
	}
	assertTypeChecks(t, res.Facet)
}

// star(n, r, ir) is a centered 2n-point star (Sketch): outer radius r at even
// indices, inner radius ir at odd ones, at angle 180*i/n.
func TestBOSL2_Star(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\nstar(n=5, r=10, ir=4);\n", "part.scad")
	if err != nil {
		t.Fatalf("star should transpile, got: %v", err)
	}
	for _, want := range []string{"fn Main() Sketch", "Polygon(", "[1:2 * 5]", "% 2 == 1 ? 4 mm : 10 mm", "180 * ", "Cos("} {
		if !strings.Contains(res.Facet, want) {
			t.Fatalf("expected %q in:\n%s", want, res.Facet)
		}
	}
	assertTypeChecks(t, res.Facet)
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
