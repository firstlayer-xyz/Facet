package emit

import (
	"math"
	"strconv"

	"facet/pkg/scad/ast"
)

// isResolutionVar reports whether name is an OpenSCAD special resolution
// variable ($fn/$fa/$fs).
func isResolutionVar(name string) bool {
	return name == "$fn" || name == "$fa" || name == "$fs"
}

// resolutionParamName is the Facet parameter name an OpenSCAD resolution
// variable maps to when used as a module parameter ($fn -> scad_fn). The
// scad_ prefix avoids colliding with user names and is a legal identifier.
func resolutionParamName(name string) string {
	return "scad_" + name[1:]
}

// resolutionFn returns the active $fn segment count for a call as a rendered
// expression: a per-call $fn argument, else the module-local $fn, else the
// global $fn. It is "" when no $fn is set anywhere. $fn is the exact segment
// count, so it is used directly (no radius needed).
func (e *Emitter) resolutionFn(n *ast.ModuleCall) string {
	if v, found := arg(n, "$fn", -1); found {
		return e.expr(v, kNumber)
	}
	if e.localFn != "" {
		return e.localFn
	}
	return e.globalFn
}

// collectResolution records top-level $fn/$fa/$fs assignments as the global
// resolution applied to curved primitives that don't set their own. $fn is kept
// as a rendered expression (it is the segment count directly); $fa/$fs must be
// positive literals, since they feed the radius-dependent fragment formula.
func (e *Emitter) collectResolution(stmts []ast.Stmt) {
	for _, s := range stmts {
		a, ok := s.(*ast.Assign)
		if !ok {
			continue
		}
		switch a.Name {
		case "$fn":
			e.globalFn = e.expr(a.Value, kNumber)
		case "$fa":
			if v, ok := numLitValue(a.Value); ok && v > 0 {
				e.globalFa, e.hasGlobalFa = v, true
			} else {
				e.errf(a.Value.Pos(), "$fa must be a positive literal")
			}
		case "$fs":
			if v, ok := numLitValue(a.Value); ok && v > 0 {
				e.globalFs, e.hasGlobalFs = v, true
			} else {
				e.errf(a.Value.Pos(), "$fs must be a positive literal")
			}
		}
	}
}

// localNum reads a positive-literal special-var arg ($fa/$fs) from a call. ok is
// false when the arg is absent; a present-but-non-literal value records an error.
func (e *Emitter) localNum(n *ast.ModuleCall, name string) (float64, bool) {
	v, found := arg(n, name, -1)
	if !found {
		return 0, false
	}
	f, isLit := numLitValue(v)
	if !isLit || f <= 0 {
		e.errf(n.Pos(), "%s must be a positive literal", name)
		return 0, false
	}
	return f, true
}

// openscadFragments is OpenSCAD's get_fragments_from_r for the fa/fs case
// (fn == 0): ceil(max(min(360/fa, 2·π·r/fs), 5)), with r in mm.
func openscadFragments(r, fa, fs float64) int {
	return int(math.Ceil(math.Max(math.Min(360.0/fa, r*2*math.Pi/fs), 5)))
}

// segmentsSuffix renders ", segments: <n>" for a curved primitive of radius rMM
// (a literal in mm; rOK reports whether it was determinable). Returns "" when no
// resolution applies, leaving Facet's own default.
func (e *Emitter) segmentsSuffix(n *ast.ModuleCall, rMM float64, rOK bool) string {
	if s := e.segmentsRadius(n, rMM, rOK); s != "" {
		return ", segments: " + s
	}
	return ""
}

// segmentsRadius returns the segment count for a curved primitive of radius rMM,
// honoring a local $fn/$fa/$fs over the global one via OpenSCAD's fragment
// formula. Returns "" when no resolution is set anywhere (option B: Facet's
// default applies). A non-literal radius under $fa/$fs is an error, not a guess.
func (e *Emitter) segmentsRadius(n *ast.ModuleCall, rMM float64, rOK bool) string {
	// $fn (per-call, then module-local, then global) is the exact segment count.
	if fn := e.resolutionFn(n); fn != "" {
		return fn
	}
	fa, hasFa := e.localNum(n, "$fa")
	if !hasFa && e.hasGlobalFa {
		fa, hasFa = e.globalFa, true
	}
	fs, hasFs := e.localNum(n, "$fs")
	if !hasFs && e.hasGlobalFs {
		fs, hasFs = e.globalFs, true
	}
	if !hasFa && !hasFs {
		return "" // nothing set → Facet default
	}
	if !rOK {
		return e.errf(n.Pos(), "%s: $fa/$fs need a literal radius to compute the fragment count", n.Name)
	}
	if !hasFa {
		fa = 12 // OpenSCAD default
	}
	if !hasFs {
		fs = 2 // OpenSCAD default
	}
	return strconv.Itoa(openscadFragments(rMM, fa, fs))
}

// segmentsAngle returns the segment count for a revolved primitive that has no
// profile radius (rotate_extrude). $fn (local over global) is used directly;
// otherwise $fa gives the angular bound ceil(max(360/$fa, 5)). $fs alone has no
// radius to apply, so it errors rather than being silently dropped.
func (e *Emitter) segmentsAngle(n *ast.ModuleCall) string {
	if fn := e.resolutionFn(n); fn != "" {
		return fn
	}
	fa, hasFa := e.localNum(n, "$fa")
	if !hasFa && e.hasGlobalFa {
		fa, hasFa = e.globalFa, true
	}
	if hasFa {
		return strconv.Itoa(int(math.Ceil(math.Max(360.0/fa, 5))))
	}
	if _, hasFs := e.localNum(n, "$fs"); hasFs || e.hasGlobalFs {
		e.errf(n.Pos(), "rotate_extrude: $fs needs the profile radius, which is not statically known")
	}
	return ""
}

// radiusMM returns a curved primitive's radius in mm as a literal: from a named
// r, a named d (halved), or the positional arg at posIdx (treated as r). ok is
// false when no such argument is a numeric literal.
func radiusMM(n *ast.ModuleCall, posIdx int) (float64, bool) {
	if v, found := arg(n, "r", -1); found {
		f, ok := numLitValue(v)
		return f, ok
	}
	if v, found := arg(n, "d", -1); found {
		f, ok := numLitValue(v)
		return f / 2, ok
	}
	if posIdx >= 0 {
		if v, found := arg(n, "", posIdx); found {
			f, ok := numLitValue(v)
			return f, ok
		}
	}
	return 0, false
}

// cylinderRadiusMM returns the largest radius of a cylinder/frustum in mm (the
// value OpenSCAD uses for fragment counting). ok is false when a present radius
// argument is not a numeric literal, or none is found.
func cylinderRadiusMM(n *ast.ModuleCall) (float64, bool) {
	maxR, any := 0.0, false
	for _, p := range []struct {
		name string
		half bool
	}{{"r1", false}, {"r2", false}, {"d1", true}, {"d2", true}} {
		v, found := arg(n, p.name, -1)
		if !found {
			continue
		}
		f, ok := numLitValue(v)
		if !ok {
			return 0, false
		}
		if p.half {
			f /= 2
		}
		if f > maxR {
			maxR = f
		}
		any = true
	}
	if any {
		return maxR, true
	}
	return radiusMM(n, 1)
}
