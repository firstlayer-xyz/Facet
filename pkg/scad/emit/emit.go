// Package emit translates an OpenSCAD AST into Facet source text.
package emit

import (
	"fmt"
	"strings"

	"facet/pkg/scad/ast"
)

// Emitter walks an AST and produces Facet source.
type Emitter struct {
	errs []TranspileError
	// syms holds collected user module/function definitions (see symbols.go),
	// so calls resolve regardless of source order.
	syms symtab
	// vecParams classifies each definition's parameters as vector ([]Number) or
	// scalar (Number), including types propagated across call sites (see
	// classifyVectorParams).
	vecParams vectorParamSet
	// nested marks parameters that are nested arrays (a list of lists), which
	// exceed the binary scalar/vector model and are typed `Any` (see
	// classifyNestedParams).
	nested vectorParamSet
	// Global resolution captured from top-level $fn/$fa/$fs (see resolution.go).
	// globalFn is a rendered $fn expression ("" if unset); globalFa/globalFs are
	// positive literals guarded by their has* flags.
	globalFn    string
	globalFa    float64
	globalFs    float64
	hasGlobalFa bool
	hasGlobalFs bool
	// usesAnimTime is set when the program references $t (OpenSCAD's animation
	// clock); File then emits `const scad_t = 0`, the non-animating default.
	usesAnimTime bool
	// usesV2/usesV3/usesFaces/usesV2Path are set when the corresponding emitted
	// helper is referenced (scad_v2 for polygon points, scad_v3 + scad_faces
	// for polyhedron, scad_v2_path for polygon-with-paths whose points are
	// computed at runtime); File then emits only those helpers (see
	// helperPreamble).
	usesV2     bool
	usesV3     bool
	usesFaces  bool
	usesV2Path bool
	// childUse records, per module, whether it consumes children() and whether
	// those children are 2D (see analyzeChildren). A module that uses children
	// gains a `children []Solid`/`[]Sketch` parameter.
	childUse map[string]childUse
	// curChild2D is the dimensionality of the children array of the module
	// currently being emitted; it classifies `children(...)` geometry nodes.
	curChild2D bool
	// localFn is the module-local $fn segment count (a rendered expression, ""
	// if none) in effect for the module currently being emitted — set when a
	// module takes a $fn parameter. It sits between a per-call $fn and the
	// global $fn in resolutionFn, mirroring OpenSCAD's dynamic scope.
	localFn string
	// paramRenames maps a parameter that the current module body reassigns
	// (OpenSCAD shadowing, e.g. `radius = radius/4`) to a fresh name for the
	// parameter; the reassignment becomes a const of the original name.
	paramRenames map[string]string
	// renamingParamRHS gates paramRenames in expr: it is true only while
	// emitting the right-hand side of such a reassignment, where the name still
	// refers to the (renamed) parameter rather than the new const.
	renamingParamRHS bool
	// scope maps the current definition's in-scope names (parameters + local
	// bindings) to their Facet type, so cond() can apply OpenSCAD truthiness to
	// a bare identifier (see buildScope / inferType).
	scope map[string]string
}

// animTimeVar is the Facet name $t is translated to. It is `scad_*`-prefixed to
// avoid colliding with a user variable named `t` (distinct from $t in OpenSCAD).
const animTimeVar = "scad_t"

// File emits a whole program as `fn Main() Solid { return <expr> }` plus any
// module/function definitions, and returns the source text + any errors.
func File(f *ast.File) (string, []TranspileError) {
	e := &Emitter{}
	e.collectResolution(f.Stmts)
	e.syms = collectSymbols(f)
	e.vecParams = classifyVectorParams(e.syms)
	e.classifyNestedParams()
	e.childUse = e.analyzeChildren(f)
	var defs []string
	var consts []string
	varIdx := map[string]int{} // last top-level assignment of a name wins
	var top []ast.Stmt
	for _, s := range f.Stmts {
		switch n := s.(type) {
		case *ast.ModuleDef:
			defs = append(defs, e.emitModuleDef(n))
		case *ast.FunctionDef:
			defs = append(defs, e.emitFunctionDef(n))
		case *ast.Assign:
			if isResolutionVar(n.Name) {
				continue // captured as a global resolution by collectResolution
			}
			if strings.HasPrefix(n.Name, "$") {
				e.errf(n.Pos(), "special variable %q is not supported", n.Name)
				continue
			}
			decl := "const " + n.Name + " = " + e.expr(n.Value, kNumber)
			if i, ok := varIdx[n.Name]; ok {
				consts[i] = decl
			} else {
				varIdx[n.Name] = len(consts)
				consts = append(consts, decl)
			}
		default:
			top = append(top, s)
		}
	}
	if len(top) == 0 {
		e.errf(f.Pos(), "no top-level geometry to render")
	}
	body := e.unionStmts(top)
	var w writer
	if e.usesAnimTime {
		w.writef("const %s = 0\n", animTimeVar)
	}
	for _, c := range consts {
		w.write(c)
		w.write("\n")
	}
	w.write(e.helperPreamble())
	for _, d := range defs {
		w.write(d)
		w.write("\n")
	}
	w.writef("fn Main() %s { return %s }\n", e.topReturnType(top), body)
	return w.str(), e.errs
}

// topReturnType classifies Main's return type from the top-level geometry
// statements. Transforms are unwrapped to their underlying primitive, so a
// `translate(...) circle(...)` still classifies as 2D. If every top-level
// statement yields a Sketch the result is a Sketch; otherwise (including the
// empty case) it is a Solid.
func (e *Emitter) topReturnType(stmts []ast.Stmt) string {
	saw2D := false
	for _, s := range stmts {
		if !e.stmtIs2D(s, map[string]bool{}) {
			return "Solid"
		}
		saw2D = true
	}
	if saw2D {
		return "Sketch"
	}
	return "Solid"
}

// unionStmts emits a sequence of child statements as a Facet expression,
// unioning multiple geometry-producing children with `+`. An empty sequence
// yields "" (the caller is responsible for ensuring this doesn't happen at the
// top level; File checks for an empty program before calling unionStmts).
func (e *Emitter) unionStmts(stmts []ast.Stmt) string {
	parts := e.childParts(stmts)
	if len(parts) == 0 {
		return ""
	}
	return unionParts(parts)
}

// unionParts folds rendered geometry parts with the union operator ` + `. A
// single part is returned bare. Any part that itself contains a top-level
// boolean operator is parenthesized so it combines as a single operand.
func unionParts(parts []string) string {
	out := parenthesizeIfOperator(parts[0])
	for _, p := range parts[1:] {
		out += " + " + parenthesizeIfOperator(p)
	}
	return out
}

// stmt emits a single statement as a Facet geometry expression ("" if none).
func (e *Emitter) stmt(s ast.Stmt) string {
	switch n := s.(type) {
	case *ast.ModuleCall:
		return e.moduleCall(n)
	case *ast.For:
		return e.forStmt(n)
	}
	return e.errf(s.Pos(), "statement %T", s)
}

// errf records an untranslatable construct and returns an empty fragment.
// Emission continues so one pass collects every error; the transpile then fails
// with no output (scad.Transpile discards the text when any error is recorded).
func (e *Emitter) errf(p ast.Pos, format string, args ...any) string {
	e.errs = append(e.errs, TranspileError{
		Feature: fmt.Sprintf(format, args...),
		Line:    p.Line,
		Col:     p.Col,
	})
	return ""
}
