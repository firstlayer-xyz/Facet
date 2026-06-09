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
		// #124 native shapes — sanity that the harness agrees with the merged fix.
		{"cyl_diag", "cyl(h=20, r=10, $fn=64, anchor=RIGHT+BACK);", 0.3},
		{"spheroid_diag", "spheroid(r=10, $fn=64, anchor=RIGHT+FWD);", 0.3},
		{"prismoid_right", "prismoid(size1=[20,20], size2=[10,10], h=10, anchor=RIGHT);", 0.02},
		{"ellipse_diag", "linear_extrude(2) ellipse([10,5], $fn=64, anchor=RIGHT+BACK);", 0.3},
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
