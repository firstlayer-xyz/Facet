package emit

import (
	"strings"

	"facet/pkg/scad/ast"
)

// reservedPrefix is the identifier prefix the transpiler reserves for the names
// it generates: scad_t / scad_ms (the animation clock and frame, see anim.go),
// scad_fn / scad_fa / scad_fs (resolution parameters), and scad_v2 / scad_v3 /
// scad_faces plus comprehension loop vars (helpers). A user identifier using
// this prefix would silently shadow or clash with generated code, so it is
// rejected up front rather than allowed to miscompile.
const reservedPrefix = "scad_"

// checkReservedNames errors on any user-defined binding — definition names,
// parameters, top-level and local variables, and for/let/comprehension loop
// variables — that uses the reserved scad_ prefix.
func (e *Emitter) checkReservedNames(f *ast.File) {
	check := func(name string, pos ast.Pos) {
		if strings.HasPrefix(name, reservedPrefix) {
			e.errf(pos, "identifier %q uses the reserved %q prefix (reserved for transpiler-generated names)", name, reservedPrefix)
		}
	}

	var walkExpr func(x ast.Expr)
	walkExpr = func(x ast.Expr) {
		switch n := x.(type) {
		case *ast.Let:
			for _, b := range n.Binds {
				check(b.Name, n.Pos())
				walkExpr(b.Value)
			}
			walkExpr(n.Body)
		case *ast.ListComp:
			for _, el := range n.Elems {
				walkCompElem(el, func(nm string) { check(nm, n.Pos()) }, walkExpr)
			}
		case *ast.Vector:
			for _, el := range n.Elems {
				walkExpr(el)
			}
		case *ast.Range:
			walkExpr(n.Start)
			walkExpr(n.End)
			if n.Step != nil {
				walkExpr(n.Step)
			}
		case *ast.Binary:
			walkExpr(n.L)
			walkExpr(n.R)
		case *ast.Unary:
			walkExpr(n.X)
		case *ast.Ternary:
			walkExpr(n.Cond)
			walkExpr(n.Then)
			walkExpr(n.Else)
		case *ast.Call:
			for _, a := range n.Args {
				walkExpr(a.Value)
			}
		case *ast.Index:
			walkExpr(n.X)
			walkExpr(n.Index)
		case *ast.Member:
			walkExpr(n.X)
		}
	}

	var walkStmts func(stmts []ast.Stmt)
	walkStmts = func(stmts []ast.Stmt) {
		for _, s := range stmts {
			switch n := s.(type) {
			case *ast.Assign:
				check(n.Name, n.Pos())
				walkExpr(n.Value)
			case *ast.ModuleCall:
				for _, a := range n.Args {
					walkExpr(a.Value)
				}
				walkStmts(n.Children)
			case *ast.For:
				for _, it := range n.Iters {
					check(it.Var, n.Pos())
					walkExpr(it.Range)
				}
				walkStmts(n.Children)
			case *ast.If:
				walkExpr(n.Cond)
				walkStmts(n.Then)
				walkStmts(n.Else)
			case *ast.ModuleDef:
				check(n.Name, n.Pos())
				for _, p := range n.Params {
					check(p.Name, n.Pos())
					if p.Default != nil {
						walkExpr(p.Default)
					}
				}
				walkStmts(n.Body)
			case *ast.FunctionDef:
				check(n.Name, n.Pos())
				for _, p := range n.Params {
					check(p.Name, n.Pos())
					if p.Default != nil {
						walkExpr(p.Default)
					}
				}
				walkExpr(n.Body)
			}
		}
	}
	walkStmts(f.Stmts)
}
