package parser

import (
	"testing"

	"facet/pkg/scad/ast"
)

func TestStmt_ModuleCallWithChildren(t *testing.T) {
	f, err := Parse("translate([1,0,0]) cube(10);")
	if err != nil {
		t.Fatal(err)
	}
	mc := f.Stmts[0].(*ast.ModuleCall)
	if mc.Name != "translate" || len(mc.Children) != 1 {
		t.Fatalf("got %#v", mc)
	}
	if mc.Children[0].(*ast.ModuleCall).Name != "cube" {
		t.Fatal("child should be cube")
	}
}

func TestStmt_BlockChildren(t *testing.T) {
	f, err := Parse("union(){ cube(1); sphere(2); }")
	if err != nil {
		t.Fatal(err)
	}
	if n := len(f.Stmts[0].(*ast.ModuleCall).Children); n != 2 {
		t.Fatalf("children = %d, want 2", n)
	}
}

func TestStmt_ModuleAndFunctionDef(t *testing.T) {
	f, err := Parse("module ring(d, h=2){ cube(d); } function sq(x)=x*x;")
	if err != nil {
		t.Fatal(err)
	}
	md := f.Stmts[0].(*ast.ModuleDef)
	if md.Name != "ring" || len(md.Params) != 2 || md.Params[1].Default == nil {
		t.Fatalf("module def = %#v", md)
	}
	fd := f.Stmts[1].(*ast.FunctionDef)
	if fd.Name != "sq" || fd.Body == nil {
		t.Fatalf("func def = %#v", fd)
	}
}

func TestStmt_ForAndIf(t *testing.T) {
	f, err := Parse("for(i=[0:3]) cube(i); if(a) cube(1); else sphere(2);")
	if err != nil {
		t.Fatal(err)
	}
	fr := f.Stmts[0].(*ast.For)
	if len(fr.Iters) != 1 || fr.Iters[0].Var != "i" {
		t.Fatalf("for = %#v", fr)
	}
	iff := f.Stmts[1].(*ast.If)
	if len(iff.Then) != 1 || len(iff.Else) != 1 {
		t.Fatalf("if = %#v", iff)
	}
}

func TestStmt_MalformedNeverHangs(t *testing.T) {
	for _, src := range []string{"cube(", "module m(", "for(", "translate([1,0,0])", "if(a)", "} ) ;"} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Parse(%q) panicked: %v", src, r)
				}
			}()
			_, _ = Parse(src)
		}()
	}
}
