//go:build cgo

package scad

import (
	"bufio"
	"bytes"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Ground-truth differential tests: each BOSL2 snippet is rendered BOTH by the real
// OpenSCAD+BOSL2 reference AND by our transpiler, and the two solids' bounding
// boxes must agree. This is the only test that can catch where our understanding
// of BOSL2 is wrong — a self-asserted bounding box just re-encodes our assumption
// (e.g. it let the anchor-by-bounding-box bug for sphere/cyl pass). Anchor errors
// shift the bbox by millimetres; faceting differs by far less, so the tolerance
// separates the two.
//
// Requires `openscad` on PATH (or the macOS app bundle) and a directory holding
// BOSL2 so `include <BOSL2/std.scad>` resolves: $OPENSCAD_BOSL2_PARENT, else
// /tmp/bosl2-ref, else the standard OpenSCAD library dirs. Skips when unavailable
// (so CI without OpenSCAD is unaffected).

func findOpenSCAD(t *testing.T) (bin, libParent string) {
	t.Helper()
	bin, err := exec.LookPath("openscad")
	if err != nil {
		const app = "/Applications/OpenSCAD.app/Contents/MacOS/OpenSCAD"
		if _, e := os.Stat(app); e == nil {
			bin = app
		} else {
			return "", ""
		}
	}
	cands := []string{os.Getenv("OPENSCAD_BOSL2_PARENT"), "/tmp/bosl2-ref"}
	if home, e := os.UserHomeDir(); e == nil {
		cands = append(cands,
			filepath.Join(home, "Documents/OpenSCAD/libraries"),
			filepath.Join(home, ".local/share/OpenSCAD/libraries"))
	}
	for _, c := range cands {
		if c != "" {
			if _, e := os.Stat(filepath.Join(c, "BOSL2", "std.scad")); e == nil {
				return bin, c
			}
		}
	}
	return "", ""
}

// openscadBBox renders scadSrc with the reference OpenSCAD+BOSL2 and returns the
// resulting STL's bounding box (min, max per axis).
func openscadBBox(t *testing.T, bin, libParent, scadSrc string) (min, max [3]float64) {
	t.Helper()
	dir := t.TempDir()
	in := filepath.Join(dir, "in.scad")
	out := filepath.Join(dir, "out.stl")
	if err := os.WriteFile(in, []byte(scadSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "-o", out, in)
	cmd.Env = append(os.Environ(), "OPENSCADPATH="+libParent)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("openscad render failed: %v\n%s", err, b)
	}
	return stlBBox(t, out)
}

// stlBBox parses an ASCII STL (OpenSCAD's default output) and returns its bbox.
func stlBBox(t *testing.T, path string) (min, max [3]float64) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	min = [3]float64{math.Inf(1), math.Inf(1), math.Inf(1)}
	max = [3]float64{math.Inf(-1), math.Inf(-1), math.Inf(-1)}
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	found := false
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) != 4 || f[0] != "vertex" {
			continue
		}
		for i := 0; i < 3; i++ {
			v, err := strconv.ParseFloat(f[i+1], 64)
			if err != nil {
				t.Fatalf("bad vertex %q: %v", sc.Text(), err)
			}
			min[i] = math.Min(min[i], v)
			max[i] = math.Max(max[i], v)
		}
		found = true
	}
	if !found {
		t.Fatalf("no vertices parsed from %s (binary STL not supported)", path)
	}
	return min, max
}

func TestBOSL2GroundTruth(t *testing.T) {
	bin, lib := findOpenSCAD(t)
	if bin == "" {
		t.Skip("openscad + BOSL2 unavailable (set OPENSCAD_BOSL2_PARENT to a dir containing BOSL2/)")
	}
	// $fn pins the facet count so curved-shape bounding boxes line up to well under
	// the tolerance; the tolerance still dwarfs any residual faceting difference but
	// is far below the millimetre-scale shifts an anchor bug produces.
	cases := []struct {
		name string
		body string // a single BOSL2 statement (2D shapes are extruded for an STL)
		tol  float64
	}{
		// core primitives (this branch) — the diagonal anchors are the bug surface.
		{"core_cube_bottom", "cube(10, anchor=BOTTOM);", 0.02},
		{"core_cube_corner", "cube([10,20,30], anchor=RIGHT+BACK+TOP);", 0.02},
		{"core_sphere_center", "sphere(r=10, $fn=64);", 0.3},
		{"core_sphere_bottom", "sphere(r=10, $fn=64, anchor=BOTTOM);", 0.3},
		{"core_sphere_diag", "sphere(r=10, $fn=64, anchor=RIGHT+BACK);", 0.3},
		{"core_cyl_top", "cylinder(h=20, r=10, $fn=64, anchor=TOP);", 0.3},
		{"core_cyl_right", "cylinder(h=20, r=10, $fn=64, anchor=RIGHT);", 0.3},
		{"core_cyl_diag", "cylinder(h=20, r=10, $fn=64, anchor=RIGHT+BACK);", 0.3},
		// regular polygons (this branch) — perimeter anchor, incl. a diagonal.
		{"hexagon_right", "linear_extrude(2) hexagon(r=10, anchor=RIGHT);", 0.02},
		{"hexagon_diag", "linear_extrude(2) hexagon(r=10, anchor=RIGHT+BACK);", 0.02},
		{"octagon_left", "linear_extrude(2) octagon(r=10, anchor=LEFT);", 0.02},
		// Plain CENTER anchor: the Ngon/Star recenter must land on BOSL2's
		// construction origin. Pentagon (odd) and star are NOT bbox-symmetric, so
		// these catch a bounding-center vs construction-center mistake.
		{"hexagon_center", "linear_extrude(2) hexagon(r=10);", 0.02},
		{"pentagon_center", "linear_extrude(2) pentagon(r=8);", 0.02},
		{"star_center", "linear_extrude(2) star(n=5, r=12, ir=5);", 0.02},
		{"star_even_center", "linear_extrude(2) star(n=8, r=10, ir=7);", 0.02},
		// Wedge (this branch) — corner-anchored ramp, centered form, and an
		// OpenSCAD-side slice that makes the bbox sensitive to ramp DIRECTION
		// (a mirrored ramp would change the z extent of the y<=4 slab).
		{"wedge_corner", "wedge([10,8,6]);", 0.02},
		{"wedge_centered", "wedge([16,16,10], center=true);", 0.02},
		{"wedge_ramp_dir", "intersection() { wedge([10,8,6]); cube([10,4,6]); }", 0.02},
		// #124 native shapes — sanity that the harness agrees with the merged fix.
		{"cyl_diag", "cyl(h=20, r=10, $fn=64, anchor=RIGHT+BACK);", 0.3},
		{"spheroid_diag", "spheroid(r=10, $fn=64, anchor=RIGHT+FWD);", 0.3},
		{"prismoid_right", "prismoid(size1=[20,20], size2=[10,10], h=10, anchor=RIGHT);", 0.02},
		{"ellipse_diag", "linear_extrude(2) ellipse([10,5], $fn=64, anchor=RIGHT+BACK);", 0.3},
		// divergences the differential audit caught and fixed (default anchors,
		// attach centering/overlap, flip-copy offset).
		{"prismoid_default", "prismoid(size1=[20,20], size2=[10,10], h=15);", 0.02},
		{"rect_tube_default", "rect_tube(size=[20,30], wall=3, h=10);", 0.02},
		{"attach_center", "cuboid([20,20,10]) attach(TOP) cyl(h=8, r=3, $fn=48);", 0.3},
		{"attach_overlap", "cuboid([20,20,10]) attach(TOP, BOTTOM, overlap=4) cyl(h=8, r=3, $fn=48);", 0.3},
		{"xflip_copy_offset", "xflip_copy(offset=5) right(10) cuboid([2,2,2]);", 0.02},
		// broader vocabulary sweep against the reference.
		{"torus", "torus(r_maj=20, r_min=5, $fn=48);", 0.3},
		{"rect_tube", "rect_tube(size=[20,30], wall=3, h=10);", 0.02},
		{"xcyl", "xcyl(r=6, l=20, $fn=64);", 0.3},
		{"wedge", "wedge([20,12,8]);", 0.02},
		{"move", "move([5,6,7]) cuboid([10,10,10]);", 0.02},
		{"xrot", "xrot(90) cuboid([10,20,30]);", 0.02},
		{"xcopies", "xcopies(spacing=10, n=3) cuboid([2,2,2]);", 0.02},
		{"line_copies", "line_copies(spacing=[5,5,0], n=4) cuboid([2,2,2]);", 0.02},
		{"zrot_copies_ring", "zrot_copies(n=5, r=15) cuboid([3,3,3]);", 0.02},
		{"mirror_copy", "mirror_copy([1,0,0]) right(10) cuboid([2,2,2]);", 0.02},
		{"top_half", "top_half() sphere(r=10, $fn=64);", 0.3},
		// local assignments inside a geometry for-loop body (block-scoped bindings).
		{"forbody_assign", "for(i=[0:2]){ x = i*5; translate([x,0,0]) cube(2); }", 0.02},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src := "include <BOSL2/std.scad>\n" + c.body + "\n"
			rmin, rmax := openscadBBox(t, bin, lib, src)
			s := renderBosl2Solid(t, src)
			fminX, fminY, fminZ, fmaxX, fmaxY, fmaxZ := s.BoundingBox()
			fmin := [3]float64{fminX, fminY, fminZ}
			fmax := [3]float64{fmaxX, fmaxY, fmaxZ}
			ax := [3]string{"x", "y", "z"}
			for i := 0; i < 3; i++ {
				if math.Abs(rmin[i]-fmin[i]) > c.tol || math.Abs(rmax[i]-fmax[i]) > c.tol {
					t.Errorf("%s axis: openscad [%.3f, %.3f] vs facet [%.3f, %.3f] (tol %.2f)",
						ax[i], rmin[i], rmax[i], fmin[i], fmax[i], c.tol)
				}
			}
		})
	}
}

// stlVolume returns the enclosed volume of an ASCII STL (signed tetrahedra). CSG
// results need a volume check: a subtraction and a union share a bounding box but
// not a volume, so the bbox harness above can't tell them apart.
func stlVolume(t *testing.T, path string) float64 {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var v [][3]float64
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) != 4 || f[0] != "vertex" {
			continue
		}
		var p [3]float64
		for i := 0; i < 3; i++ {
			x, err := strconv.ParseFloat(f[i+1], 64)
			if err != nil {
				t.Fatalf("bad vertex %q: %v", sc.Text(), err)
			}
			p[i] = x
		}
		v = append(v, p)
	}
	vol := 0.0
	for i := 0; i+2 < len(v); i += 3 {
		a, b, c := v[i], v[i+1], v[i+2]
		vol += (a[0]*(b[1]*c[2]-b[2]*c[1]) - a[1]*(b[0]*c[2]-b[2]*c[0]) + a[2]*(b[0]*c[1]-b[1]*c[0])) / 6.0
	}
	return math.Abs(vol)
}

func openscadVolume(t *testing.T, bin, libParent, scadSrc string) float64 {
	t.Helper()
	dir := t.TempDir()
	in := filepath.Join(dir, "in.scad")
	out := filepath.Join(dir, "out.stl")
	if err := os.WriteFile(in, []byte(scadSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "-o", out, in)
	cmd.Env = append(os.Environ(), "OPENSCADPATH="+libParent)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("openscad render failed: %v\n%s", err, b)
	}
	return stlVolume(t, out)
}

// TestBOSL2GroundTruth_CSG verifies diff()/intersect()/tag results by volume
// against real OpenSCAD+BOSL2. A sibling tag("remove") must SUBTRACT (today the
// transpiler unions it — silent miscompile); intersect() and the keep tag must
// work too.
func TestBOSL2GroundTruth_CSG(t *testing.T) {
	bin, lib := findOpenSCAD(t)
	if bin == "" {
		t.Skip("openscad + BOSL2 unavailable (set OPENSCAD_BOSL2_PARENT)")
	}
	cases := []struct {
		name   string
		body   string
		relTol float64
	}{
		{"diff_sibling", `diff(){ cuboid([20,20,20]); tag("remove") cuboid([30,6,6]); }`, 0.005},
		{"intersect", `intersect(){ cuboid([20,20,20]); tag("intersect") sphere(d=24, $fn=72); }`, 0.03},
		{"diff_keep", `diff(){ cuboid([20,20,20]); tag("remove") cuboid([30,6,6]); tag("keep") cuboid([2,2,30]); }`, 0.005},
		// custom remove tag name (positional): "hole" replaces the default "remove".
		{"diff_custom_tag", `diff("hole"){ cuboid([20,20,20]); tag("hole") cuboid([30,6,6]); }`, 0.005},
		// attachment-diff (regression): a tag("remove") attach-child of an untagged
		// parent is subtracted via the attachment chain, with the scope keeping the
		// parent as the base.
		{"diff_attach_child", `diff() cuboid([20,20,20]){ tag("remove") attach(TOP) cyl(d=6, h=8, $fn=48); }`, 0.02},
		// nested tag: a keep INSIDE a remove group is unioned back, not subtracted
		// (the inner tag overrides the outer). Was a silent miscompile (6776 vs 7856).
		{"diff_nested_keep", `diff(){ cuboid([20,20,20]); tag("remove"){ cuboid([30,6,6]); tag("keep") cuboid([6,30,6]); } }`, 0.005},
		// cross-parent remove: a tag("remove") attached to one parent (positioned by
		// right(8)) reaches into a SIBLING parent and must cut it too — the remove
		// surfaces to the scope and is subtracted from all untagged geometry.
		{"diff_cross_parent", `diff(){ cuboid([10,10,10]); right(8) cuboid([10,10,10]){ tag("remove") position(LEFT) cuboid([16,3,3]); } }`, 0.01},
		// tag below a transform: the remove rides through up(6) and still cuts the base.
		{"diff_tag_under_transform", `diff(){ cuboid([20,20,20]); up(6) tag("remove") cuboid([30,6,6]); }`, 0.005},
		// hide(tags): the tagged children are dropped from the result; the rest stay.
		{"hide", `hide("bar"){ cuboid([20,20,20]); tag("bar") cuboid([30,6,6]); }`, 0.005},
		// show_only(tags): only the tagged children are kept.
		{"show_only", `show_only("bar"){ cuboid([20,20,20]); tag("bar") cuboid([30,6,6]); }`, 0.005},
		// per-edge rounding/chamfer of one axis group. The box is asymmetric so the
		// volume differs by axis (the rounded edges' length = that axis's extent) —
		// this verifies BOTH the amount removed AND that the correct edges round.
		{"cuboid_round_z", `cuboid([10,20,30], rounding=2, edges="Z", $fn=64);`, 0.01},
		{"cuboid_round_x", `cuboid([10,20,30], rounding=2, edges="X", $fn=64);`, 0.01},
		{"cuboid_round_y", `cuboid([10,20,30], rounding=2, edges="Y", $fn=64);`, 0.01},
		{"cuboid_chamfer_z", `cuboid([10,20,30], chamfer=2, edges="Z");`, 0.005},
		{"cuboid_chamfer_x", `cuboid([10,20,30], chamfer=2, edges="X");`, 0.005},
		{"cuboid_chamfer_all", `cuboid([10,20,30], chamfer=2);`, 0.02},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src := "include <BOSL2/std.scad>\n" + c.body + "\n"
			want := openscadVolume(t, bin, lib, src)
			got := renderBosl2Solid(t, src).Volume()
			if math.Abs(got-want) > c.relTol*want {
				t.Errorf("volume: openscad %.1f vs facet %.1f (rel tol %.1f%%)", want, got, c.relTol*100)
			}
		})
	}
}
