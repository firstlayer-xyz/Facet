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

// A two-anchor attach with NON-opposite anchors (here RIGHT, BOTTOM) rotates the
// child so its named face mates with the parent face. It used to be unsupported;
// the runtime now does it via move-to-origin → rotate → move-to-anchor, so it
// transpiles and type-checks.
func TestBOSL2_AttachNonOpposite(t *testing.T) {
	src := "include <BOSL2/std.scad>\ncuboid([20, 20, 10]) attach(RIGHT, BOTTOM) cyl(h=8, r=3);\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("non-opposite attach should transpile, got: %v", err)
	}
	assertTypeChecks(t, res.Facet)
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

// BOSL2 rot(a, v=axis) is an axis-angle rotation — maps to Rotate(axis, angle),
// now that those exist in the stdlib.
func TestBOSL2_RotAxisAngle(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\nrot(a=30, v=[1, 1, 0]) cube(2);\n", "part.scad")
	if err != nil {
		t.Fatalf("rot(a, v) should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "axis: Vec3{") || !strings.Contains(res.Facet, "angle: 30 deg") {
		t.Fatalf("expected an axis-angle rotate:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// BOSL2 rot(from=, to=) aligns one direction onto another — maps to
// Rotate(from, to).
func TestBOSL2_RotFromTo(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\nrot(from=[0, 0, 1], to=[1, 0, 0]) cube(2);\n", "part.scad")
	if err != nil {
		t.Fatalf("rot(from, to) should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "from: Vec3{") || !strings.Contains(res.Facet, "to: Vec3{") {
		t.Fatalf("expected a from/to rotate:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// BOSL2 orient= points a primitive's +Z (UP) axis along the given anchor — maps
// to Rotate(from: UP, to: orient) on the centered shape.
func TestBOSL2_CuboidOrient(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\ncuboid([10, 4, 2], orient=RIGHT);\n", "part.scad")
	if err != nil {
		t.Fatalf("cuboid(orient=) should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "from: Vec3{") || !strings.Contains(res.Facet, "to: Vec3{") {
		t.Fatalf("expected orient -> Rotate(from, to):\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

func TestBOSL2_CylOrient(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\ncyl(h=10, r=2, orient=FWD);\n", "part.scad")
	if err != nil {
		t.Fatalf("cyl(orient=) should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "from: Vec3{") || !strings.Contains(res.Facet, "to: Vec3{") {
		t.Fatalf("expected orient -> Rotate(from, to):\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// BOSL2 cuboid(rounding=R) rounds every edge — maps to Facet Cube(fillet: R),
// now that fillet primitives exist.
func TestBOSL2_CuboidRounding(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\ncuboid([10, 10, 10], rounding=2);\n", "part.scad")
	if err != nil {
		t.Fatalf("cuboid(rounding=) should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "fillet: 2 mm") {
		t.Fatalf("expected rounding -> fillet:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// BOSL2 cyl(rounding=R) rounds both rims — maps to Facet Cylinder(fillet: R).
func TestBOSL2_CylRounding(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\ncyl(h=10, r=5, rounding=1);\n", "part.scad")
	if err != nil {
		t.Fatalf("cyl(rounding=) should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "fillet: 1 mm") {
		t.Fatalf("expected rounding -> fillet:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// A single-anchor attach to a COMBINED anchor (an edge/corner like TOP+RIGHT)
// reorients the child out along that diagonal — now expressible via the from/to
// rotation, so it no longer errors.
func TestBOSL2_AttachCombinedAnchor(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\ncuboid([20, 20, 20]) attach(TOP+RIGHT) cyl(h=10, r=2);\n", "part.scad")
	if err != nil {
		t.Fatalf("combined-anchor attach should transpile, got: %v", err)
	}
	assertTypeChecks(t, res.Facet)
}

// cuboid(chamfer=C) / cyl(chamfer=C) bevel every edge — mapped to the Facet
// Cube/Cylinder chamfer primitives (added in #106).
func TestBOSL2_Chamfer(t *testing.T) {
	for _, tc := range []struct{ src, want string }{
		{"cuboid([10, 10, 10], chamfer=2)", "chamfer: 2 mm"},
		{"cyl(h=10, r=5, chamfer=1)", "chamfer: 1 mm"},
	} {
		res, err := Transpile("include <BOSL2/std.scad>\n"+tc.src+";\n", "part.scad")
		if err != nil {
			t.Fatalf("%q should transpile, got: %v", tc.src, err)
		}
		if !strings.Contains(res.Facet, tc.want) {
			t.Fatalf("%q: expected %q in:\n%s", tc.src, tc.want, res.Facet)
		}
		assertTypeChecks(t, res.Facet)
	}
}

// A BARE child (no position/attach) on a shape that isn't an attachment parent
// must error, not be silently dropped by the leaf emitter. In BOSL2 a child of a
// shape is an attachment, and only cuboid/cyl carry one.
func TestBOSL2_BareChildOnUnsupportedParentErrors(t *testing.T) {
	for _, src := range []string{
		"include <BOSL2/std.scad>\nsphere(5) cuboid(2);\n",
		"include <BOSL2/std.scad>\nprismoid(size1=[10, 10], size2=[6, 6], h=8) cuboid(2);\n",
	} {
		res, err := Transpile(src, "part.scad")
		if err == nil {
			t.Fatalf("a bare child on an unsupported parent should error, got:\n%s", res.Facet)
		}
		if res.Facet != "" {
			t.Fatalf("expected no output on error, got:\n%s", res.Facet)
		}
		if !strings.Contains(err.Error(), "only supported on cuboid and cyl") {
			t.Fatalf("error should explain the unsupported parent, got: %v", err)
		}
	}
}

// An attached child cannot itself carry attachments — nested attachment chains
// aren't supported, so the inner attachment errors rather than being dropped.
func TestBOSL2_NestedAttachmentErrors(t *testing.T) {
	src := "include <BOSL2/std.scad>\ncuboid(20) attach(TOP) cuboid(4) attach(TOP) cyl(h=3, r=1);\n"
	res, err := Transpile(src, "part.scad")
	if err == nil {
		t.Fatalf("nested attachment should error, got:\n%s", res.Facet)
	}
	if !strings.Contains(err.Error(), "nested attachments") {
		t.Fatalf("error should explain nested attachments, got: %v", err)
	}
}

// arc_copies with a full/wrapping end angle (ea >= 360) would overlap the first
// and last copies under the (n-1)-gap formula; it errors rather than emitting
// overlapping geometry. Omitting ea gives a correct full circle.
func TestBOSL2_ArcCopiesFullArcErrors(t *testing.T) {
	src := "include <BOSL2/std.scad>\narc_copies(n=6, r=20, ea=360) cube(2);\n"
	_, err := Transpile(src, "part.scad")
	if err == nil {
		t.Fatal("arc_copies with ea=360 should error")
	}
	if !strings.Contains(err.Error(), "full or wrapping arc") {
		t.Fatalf("error should explain the wrapping arc, got: %v", err)
	}
}

// Distributor arguments that are recognized but not implemented (line_copies l,
// grid_copies size) must error via rejectExtraArgs, never be silently ignored.
func TestBOSL2_UnimplementedDistributorArgsError(t *testing.T) {
	for _, src := range []string{
		"include <BOSL2/std.scad>\nline_copies(l=50, n=3) cube(2);\n",
		"include <BOSL2/std.scad>\ngrid_copies(size=[40, 40], n=3) cube(2);\n",
	} {
		_, err := Transpile(src, "part.scad")
		if err == nil {
			t.Fatalf("an unimplemented distributor arg should error:\n%s", src)
		}
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

// line_copies(spacing=[..], n) spaces n copies along a direction vector,
// centered on the origin (the vector generalization of xcopies).
func TestBOSL2_LineCopies(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\nline_copies(spacing=[10, 5, 0], n=3) cuboid(2);\n", "part.scad")
	if err != nil {
		t.Fatalf("line_copies should transpile, got: %v", err)
	}
	for _, want := range []string{"Union(", "[0:3 - 1]", ".Move(", "(3 - 1) / 2) * 10 mm", "(3 - 1) / 2) * 5 mm"} {
		if !strings.Contains(res.Facet, want) {
			t.Fatalf("expected %q in:\n%s", want, res.Facet)
		}
	}
	// A zero spacing component is dropped (no z offset here).
	if strings.Contains(res.Facet, "z: (") {
		t.Fatalf("zero spacing axis should be omitted:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// arc_copies(n, r) with no sa/ea spaces n copies evenly around a full circle;
// each is moved out to r and rotated to face outward.
func TestBOSL2_ArcCopiesFull(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\narc_copies(n=6, r=20) cuboid(2);\n", "part.scad")
	if err != nil {
		t.Fatalf("arc_copies should transpile, got: %v", err)
	}
	for _, want := range []string{"Union(", ".Move(x: 20 mm)", ".Rotate(z:", "* 360 / 6"} {
		if !strings.Contains(res.Facet, want) {
			t.Fatalf("expected %q in:\n%s", want, res.Facet)
		}
	}
	assertTypeChecks(t, res.Facet)
}

// A partial arc (sa/ea) spans the angle range inclusively: angle = sa +
// i*(ea-sa)/(n-1).
func TestBOSL2_ArcCopiesPartial(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\narc_copies(n=3, r=20, sa=0, ea=90) cuboid(2);\n", "part.scad")
	if err != nil {
		t.Fatalf("partial arc_copies should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "(90 - 0) / (3 - 1)") {
		t.Fatalf("expected inclusive arc stepping in:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// wedge emits BOSL2's triangular ramp as its exact VNF (a Mesh). By default the
// min corner sits at the origin (a trailing Move by size/2).
func TestBOSL2_Wedge(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\nwedge([10, 8, 6]);\n", "part.scad")
	if err != nil {
		t.Fatalf("wedge should transpile, got: %v", err)
	}
	for _, want := range []string{"Mesh{vertices:", "[]Face[", ".Solid()", ".Move(x: 10 mm / 2"} {
		if !strings.Contains(res.Facet, want) {
			t.Fatalf("expected %q in:\n%s", want, res.Facet)
		}
	}
	assertTypeChecks(t, res.Facet)
}

// wedge(center=true) keeps the VNF centered (no trailing Move).
func TestBOSL2_WedgeCentered(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\nwedge([10, 8, 6], center=true);\n", "part.scad")
	if err != nil {
		t.Fatalf("centered wedge should transpile, got: %v", err)
	}
	if strings.Contains(res.Facet, ".Solid().Move(") {
		t.Fatalf("centered wedge should not be shifted:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// cyl with r1/r2 (or d1/d2) is a cone/frustum, centered, via Facet's Frustum.
func TestBOSL2_CylCone(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\ncyl(h=10, r1=5, r2=2);\n", "part.scad")
	if err != nil {
		t.Fatalf("cyl cone should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "Frustum(r1: 5 mm, r2: 2 mm, h: 10 mm") || !strings.Contains(res.Facet, ".AlignCenter(pos: Vec3{})") {
		t.Fatalf("expected centered Frustum in:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// spheroid is BOSL2's preferred sphere; r/d map to a centered Sphere (the
// circum/style tessellation options are not supported and error).
func TestBOSL2_Spheroid(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\nspheroid(r=5);\n", "part.scad")
	if err != nil {
		t.Fatalf("spheroid should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "Sphere(r: 5 mm)") || !strings.Contains(res.Facet, ".AlignCenter(pos: Vec3{})") {
		t.Fatalf("expected centered Sphere in:\n%s", res.Facet)
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

// BOSL2 trapezoid is a centered 2D isosceles trapezoid — a four-point Polygon.
func TestBOSL2_Trapezoid(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\ntrapezoid(h=10, w1=20, w2=10);\n", "part.scad")
	if err != nil {
		t.Fatalf("trapezoid should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "Polygon(") {
		t.Fatalf("expected a Polygon:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// BOSL2 half-space cuts (top_half/back_half/…) and the general half_of(v) keep
// one side of the plane through the origin — mapped to Solid.Trim.
func TestBOSL2_HalfCuts(t *testing.T) {
	cases := []struct{ src, want string }{
		{"top_half() cuboid([10, 10, 10])", ".Trim(z: 1)"},
		{"bottom_half() cuboid([10, 10, 10])", ".Trim(z: -1)"},
		{"back_half() cuboid([10, 10, 10])", ".Trim(y: 1)"},
		{"left_half() cuboid([10, 10, 10])", ".Trim(x: -1)"},
		{"half_of([0, 0, 1]) cuboid([10, 10, 10])", ".Trim(x: 0, y: 0, z: 1)"},
		{"half_of(UP) cuboid([10, 10, 10])", ".Trim(x: 0, y: 0, z: 1)"},
	}
	for _, c := range cases {
		res, err := Transpile("include <BOSL2/std.scad>\n"+c.src+";\n", "part.scad")
		if err != nil {
			t.Fatalf("%q should transpile, got: %v", c.src, err)
		}
		if !strings.Contains(res.Facet, c.want) {
			t.Fatalf("%q: expected %q in:\n%s", c.src, c.want, res.Facet)
		}
		assertTypeChecks(t, res.Facet)
	}
}

// BOSL2 ellipse is a centered 2D ellipse — a circle of radius rx scaled in y by
// ry/rx (built at rx so the facet count suits the final size).
func TestBOSL2_Ellipse(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\nellipse(r=[10, 5]);\n", "part.scad")
	if err != nil {
		t.Fatalf("ellipse should transpile, got: %v", err)
	}
	for _, want := range []string{"Circle(r: 10 mm", ".Scale(", "Number(from: 5 mm) / Number(from: 10 mm)"} {
		if !strings.Contains(res.Facet, want) {
			t.Fatalf("expected %q in:\n%s", want, res.Facet)
		}
	}
	assertTypeChecks(t, res.Facet)
}

// xflip/yflip/zflip are single-axis mirrors. With no offset they mirror about the
// plane through the origin (a plain Mirror); an offset shifts the mirror plane.
func TestBOSL2_Flip(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\nxflip() cuboid([10, 10, 10]);\n", "part.scad")
	if err != nil {
		t.Fatalf("xflip should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, ".Mirror(x: 1)") {
		t.Fatalf("expected .Mirror(x: 1) in:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)

	res, err = Transpile("include <BOSL2/std.scad>\nzflip(z=5) cuboid([10, 10, 10]);\n", "part.scad")
	if err != nil {
		t.Fatalf("zflip(z=5) should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, ".Move(z: -5 mm).Mirror(z: 1).Move(z: 5 mm)") {
		t.Fatalf("expected offset mirror sandwich in:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// recolor(c) paints the child the given color — Facet's Solid.Color via the
// shared OpenSCAD color mapping (CSS names and r,g,b vectors).
func TestBOSL2_Recolor(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\nrecolor(\"red\") cuboid([10, 10, 10]);\n", "part.scad")
	if err != nil {
		t.Fatalf("recolor name should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, `.Color(hex: "#FF0000")`) {
		t.Fatalf("expected red Color(hex:) in:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)

	res, err = Transpile("include <BOSL2/std.scad>\nrecolor([0, 0, 1]) cuboid([10, 10, 10]);\n", "part.scad")
	if err != nil {
		t.Fatalf("recolor vector should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, `.Color(hex: "#0000FF")`) {
		t.Fatalf("expected blue Color(hex:) from vector in:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// xdistribute/ydistribute/zdistribute spread the distinct children evenly along
// an axis, centered: child i of n sits at (i-(n-1)/2)*spacing. Three children at
// spacing 10 land at -10, 0, +10 (the middle one keeps its place).
func TestBOSL2_Distribute(t *testing.T) {
	src := "include <BOSL2/std.scad>\nxdistribute(10) { cuboid(2); cuboid(2); cuboid(2); }\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("xdistribute should transpile, got: %v", err)
	}
	for _, want := range []string{".Move(x: -1 * 10 mm)", ".Move(x: 1 * 10 mm)"} {
		if !strings.Contains(res.Facet, want) {
			t.Fatalf("expected %q in:\n%s", want, res.Facet)
		}
	}
	assertTypeChecks(t, res.Facet)
}

// A total length l spreads the children across the gaps: l=20 over three children
// gives spacing 10.
func TestBOSL2_DistributeLength(t *testing.T) {
	src := "include <BOSL2/std.scad>\nydistribute(l=20) { cuboid(2); cuboid(2); cuboid(2); }\n"
	res, err := Transpile(src, "part.scad")
	if err != nil {
		t.Fatalf("ydistribute(l=) should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "20 mm / 2") {
		t.Fatalf("expected l spread across gaps in:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// cuboid(anchor=) shifts the centered box so the anchor point lands on the
// origin: the box moves by -anchor*size/2 per axis. BOTTOM sits it on the plate.
func TestBOSL2_CuboidAnchor(t *testing.T) {
	cases := []struct{ src, want string }{
		// BOTTOM -> +z/2: a 30-tall box rises to z:0..30.
		{"cuboid([10, 20, 30], anchor=BOTTOM)", ".Move(z: 0.5 * 30 mm)"},
		// TOP -> -z/2.
		{"cuboid(10, anchor=TOP)", ".Move(z: -0.5 * 10 mm)"},
		// Combined RIGHT+TOP shifts two axes at once.
		{"cuboid([10, 10, 10], anchor=RIGHT+TOP)", ".Move(x: -0.5 * 10 mm, z: -0.5 * 10 mm)"},
	}
	for _, c := range cases {
		res, err := Transpile("include <BOSL2/std.scad>\n"+c.src+";\n", "part.scad")
		if err != nil {
			t.Fatalf("%q should transpile, got: %v", c.src, err)
		}
		if !strings.Contains(res.Facet, c.want) {
			t.Fatalf("%q: expected %q in:\n%s", c.src, c.want, res.Facet)
		}
		assertTypeChecks(t, res.Facet)
	}
}

// cyl(anchor=) shifts the centered cylinder by its anchor point. BOTTOM uses the
// height (z); an off-axis anchor (RIGHT) uses the diameter (2r). A tapered cyl
// with an x/y anchor is unsupported (its bounding diameter is the larger end).
func TestBOSL2_CylAnchor(t *testing.T) {
	cases := []struct{ src, want string }{
		{"cyl(h=10, r=4, anchor=BOTTOM)", ".Move(z: 0.5 * 10 mm)"},
		{"cyl(h=10, r=4, anchor=TOP)", ".Move(z: -0.5 * 10 mm)"},
		{"cyl(h=10, r=4, anchor=RIGHT)", ".Move(x: -0.5 * (2 * 4 mm))"},
		{"cyl(h=10, d=8, anchor=RIGHT)", ".Move(x: -0.5 * 8 mm)"},
	}
	for _, c := range cases {
		res, err := Transpile("include <BOSL2/std.scad>\n"+c.src+";\n", "part.scad")
		if err != nil {
			t.Fatalf("%q should transpile, got: %v", c.src, err)
		}
		if !strings.Contains(res.Facet, c.want) {
			t.Fatalf("%q: expected %q in:\n%s", c.src, c.want, res.Facet)
		}
		assertTypeChecks(t, res.Facet)
	}
}

// An x/y anchor on a tapered cyl is a located error, not silently-wrong geometry.
func TestBOSL2_CylAnchorTaperedXYErrors(t *testing.T) {
	_, err := Transpile("include <BOSL2/std.scad>\ncyl(h=10, r1=4, r2=2, anchor=RIGHT);\n", "part.scad")
	if err == nil {
		t.Fatal("x/y anchor on a tapered cyl should error")
	}
	if !strings.Contains(err.Error(), "tapered") {
		t.Fatalf("expected a tapered-cyl anchor error, got: %v", err)
	}
}

// spheroid(anchor=) shifts the centered sphere by its anchor point over its
// [2r,2r,2r] box. BOTTOM lifts it onto the plate by r; the d form halves first.
func TestBOSL2_SpheroidAnchor(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\nspheroid(r=5, anchor=BOTTOM);\n", "part.scad")
	if err != nil {
		t.Fatalf("spheroid anchor should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, ".Move(z: 0.5 * (2 * 5 mm))") {
		t.Fatalf("expected BOTTOM lift by r in:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)

	// Plain spheroid (no anchor) still emits a bare centered sphere.
	res, err = Transpile("include <BOSL2/std.scad>\nspheroid(d=8);\n", "part.scad")
	if err != nil {
		t.Fatalf("spheroid should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "Sphere(d: 8 mm") || !strings.Contains(res.Facet, ".AlignCenter(pos: Vec3{})") {
		t.Fatalf("expected centered sphere:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// spin= rotates a shape about its Z axis (BOSL2's attachable spin), applied after
// anchor placement. It maps to a trailing Rotate(z: spin deg) on cuboid/cyl/
// spheroid; with anchor= the spin follows the anchor Move.
func TestBOSL2_Spin(t *testing.T) {
	cases := []struct{ src, want string }{
		{"cuboid([10, 20, 30], spin=45)", ".Rotate(z: 45 deg)"},
		{"cyl(h=10, r=4, spin=30)", ".Rotate(z: 30 deg)"},
		{"spheroid(r=5, spin=90)", ".Rotate(z: 90 deg)"},
	}
	for _, c := range cases {
		res, err := Transpile("include <BOSL2/std.scad>\n"+c.src+";\n", "part.scad")
		if err != nil {
			t.Fatalf("%q should transpile, got: %v", c.src, err)
		}
		if !strings.Contains(res.Facet, c.want) {
			t.Fatalf("%q: expected %q in:\n%s", c.src, c.want, res.Facet)
		}
		assertTypeChecks(t, res.Facet)
	}

	// anchor + spin: both apply, spin after the anchor Move.
	res, err := Transpile("include <BOSL2/std.scad>\ncuboid([10, 20, 30], anchor=BOTTOM, spin=90);\n", "part.scad")
	if err != nil {
		t.Fatalf("anchor+spin should transpile, got: %v", err)
	}
	for _, want := range []string{".Move(z: 0.5 * 30 mm)", ".Rotate(z: 90 deg)"} {
		if !strings.Contains(res.Facet, want) {
			t.Fatalf("expected %q in:\n%s", want, res.Facet)
		}
	}
	assertTypeChecks(t, res.Facet)
}

// align(anchor) seats the child flush against the parent's anchor face (aligned
// by bounding box, not reoriented): the child is placed at the anchor point and
// pushed out by half its own size (dir 1). inside=true pushes it in (dir -1), and
// under diff() that becomes a subtractive alignRemove.
func TestBOSL2_Align(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\ncuboid([20, 20, 10]) align(TOP) cuboid([4, 4, 8]);\n", "part.scad")
	if err != nil {
		t.Fatalf("align should transpile, got: %v", err)
	}
	for _, want := range []string{".align(", "a: B2Anchor{x: 0, y: 0, z: 1}", "child: b2_cuboid(size: Vec3{x: 4 mm", "dir: 1"} {
		if !strings.Contains(res.Facet, want) {
			t.Fatalf("expected %q in:\n%s", want, res.Facet)
		}
	}
	assertTypeChecks(t, res.Facet)

	// inside=true flips the push direction.
	res, err = Transpile("include <BOSL2/std.scad>\ncuboid([20, 20, 10]) align(TOP, inside=true) cuboid([4, 4, 4]);\n", "part.scad")
	if err != nil {
		t.Fatalf("align inside should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "dir: -1") {
		t.Fatalf("expected inside push dir -1 in:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)

	// diff() + inside makes it subtractive (alignRemove).
	res, err = Transpile("include <BOSL2/std.scad>\ndiff() cuboid([20, 20, 10]) { align(TOP, inside=true) cuboid([4, 4, 4]); }\n", "part.scad")
	if err != nil {
		t.Fatalf("diff align should transpile, got: %v", err)
	}
	if !strings.Contains(mainBody(t, res.Facet), ".alignRemove(") {
		t.Fatalf("expected subtractive alignRemove in Main:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// align= second positional (sub-alignment) and inset/shiftout are not translated;
// they must error rather than be silently dropped.
func TestBOSL2_AlignRejectsExtras(t *testing.T) {
	_, err := Transpile("include <BOSL2/std.scad>\ncuboid([20, 20, 10]) align(TOP, inset=2) cuboid([4, 4, 8]);\n", "part.scad")
	if err == nil {
		t.Fatal("align(inset=) should error (not translated)")
	}
}

// rect(anchor=) shifts the centered 2D rectangle by its anchor point (in-plane
// only). RIGHT puts the right edge on the origin. A z-bearing anchor (TOP) is a
// located error — there is no out-of-plane axis on a 2D shape.
func TestBOSL2_RectAnchor(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\nrect([20, 10], anchor=RIGHT);\n", "part.scad")
	if err != nil {
		t.Fatalf("rect anchor should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, ".Move(x: -0.5 * 20 mm)") {
		t.Fatalf("expected RIGHT shift by w/2 in:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)

	_, err = Transpile("include <BOSL2/std.scad>\nrect([20, 10], anchor=TOP);\n", "part.scad")
	if err == nil {
		t.Fatal("rect(anchor=TOP) should error (out-of-plane anchor on a 2D shape)")
	}
}

// tube(anchor=) shifts the centered tube by its anchor point over its
// [outer-dia, outer-dia, h] bounding box. BOTTOM sits it on the plate (z).
func TestBOSL2_TubeAnchor(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\ntube(h=10, or=8, ir=4, anchor=BOTTOM);\n", "part.scad")
	if err != nil {
		t.Fatalf("tube anchor should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, ".Move(z: 0.5 * 10 mm)") {
		t.Fatalf("expected BOTTOM lift by h/2 in:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}

// prismoid(anchor=) shifts the tapered box by its anchor point over the bounding
// box [max(size1.x,size2.x), max(...y), h]. BOTTOM sits it on the plate (z);
// RIGHT uses the wider end on x via Max.
func TestBOSL2_PrismoidAnchor(t *testing.T) {
	res, err := Transpile("include <BOSL2/std.scad>\nprismoid(size1=[20, 20], size2=[10, 10], h=15, anchor=BOTTOM);\n", "part.scad")
	if err != nil {
		t.Fatalf("prismoid anchor should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, ".Move(z: 0.5 * 15 mm)") {
		t.Fatalf("expected BOTTOM lift by h/2 in:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)

	res, err = Transpile("include <BOSL2/std.scad>\nprismoid(size1=[20, 20], size2=[10, 10], h=15, anchor=RIGHT);\n", "part.scad")
	if err != nil {
		t.Fatalf("prismoid RIGHT anchor should transpile, got: %v", err)
	}
	if !strings.Contains(res.Facet, "Max(a: 20 mm, b: 10 mm)") {
		t.Fatalf("expected x bound via Max of the two ends in:\n%s", res.Facet)
	}
	assertTypeChecks(t, res.Facet)
}
