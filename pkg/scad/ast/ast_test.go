package ast

import "testing"

func TestAST_Construction(t *testing.T) {
	var _ Stmt = &ModuleCall{Name: "cube"}
	var _ Stmt = &ModuleDef{Name: "ring"}
	var _ Stmt = &FunctionDef{Name: "f"}
	var _ Stmt = &Assign{Name: "x"}
	var _ Stmt = &For{}
	var _ Stmt = &If{}
	var _ Expr = &Num{Text: "1"}
	var _ Expr = &Str{Value: "s"}
	var _ Expr = &Ident{Name: "x"}
	var _ Expr = &Vector{}
	var _ Expr = &Range{}
	var _ Expr = &Binary{Op: "+"}
	var _ Expr = &Unary{Op: "-"}
	var _ Expr = &Ternary{}
	var _ Expr = &Call{Name: "sin"}
	var _ Expr = &Index{}
	var _ Expr = &Member{}
	var _ Expr = &Let{}

	f := &File{Stmts: []Stmt{&ModuleCall{Name: "cube", Args: []Arg{{Value: &Num{Text: "10"}}}}}}
	if len(f.Stmts) != 1 {
		t.Fatal("file should hold one stmt")
	}
	// Position accessor is exported and usable across packages.
	if got := (&Num{P: Pos{Line: 3, Col: 4}}).Pos(); got.Line != 3 || got.Col != 4 {
		t.Fatalf("Pos() = %+v", got)
	}
}
