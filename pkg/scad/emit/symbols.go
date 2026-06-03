package emit

import "facet/pkg/scad/ast"

// symbol is a collected user definition (module or function). It lets calls be
// translated before the definition is emitted, since OpenSCAD allows forward
// and mutual references, and carries the body for inter-procedural parameter
// classification (see classifyVectorParams).
type symbol struct {
	params []ast.Param
	isFunc bool // function (value) vs module (geometry)
	// Exactly one of moduleBody/funcBody is set, per isFunc.
	moduleBody []ast.Stmt
	funcBody   ast.Expr
}

// symtab maps a user-defined module/function name to its signature.
type symtab map[string]*symbol

// collectSymbols records every top-level module/function definition so calls
// resolve regardless of source order.
func collectSymbols(f *ast.File) symtab {
	syms := symtab{}
	for _, s := range f.Stmts {
		switch d := s.(type) {
		case *ast.ModuleDef:
			syms[d.Name] = &symbol{params: d.Params, moduleBody: d.Body}
		case *ast.FunctionDef:
			syms[d.Name] = &symbol{params: d.Params, isFunc: true, funcBody: d.Body}
		}
	}
	return syms
}
