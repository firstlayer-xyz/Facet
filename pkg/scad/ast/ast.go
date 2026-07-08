// Package ast defines the OpenSCAD abstract syntax tree.
package ast

// Pos is a 1-based source position.
type Pos struct{ Line, Col int }

// Node is any AST node; every node exposes its source position.
type Node interface{ Pos() Pos }

// Stmt is a statement node.
type Stmt interface {
	Node
	stmtNode()
}

// Expr is an expression node.
type Expr interface {
	Node
	exprNode()
}

// File is a whole .scad program: a sequence of top-level statements.
type File struct {
	Stmts []Stmt
	P     Pos
}

func (f *File) Pos() Pos { return f.P }

// Arg is a call/instantiation argument; Name is "" for positional args.
type Arg struct {
	Name  string
	Value Expr
}

// ---- statements ----

// ModuleCall is an instantiation: `name(args) <children>` or `name(args);`.
// Children is non-nil when the call had a `{ ... }` or single-child body.
type ModuleCall struct {
	Name     string
	Args     []Arg
	Children []Stmt
	P        Pos
}

// ModuleDef is `module name(params) { body }`.
type ModuleDef struct {
	Name   string
	Params []Param
	Body   []Stmt
	P      Pos
}

// FunctionDef is `function name(params) = expr;`.
type FunctionDef struct {
	Name   string
	Params []Param
	Body   Expr
	P      Pos
}

// Param is a module/function parameter with an optional default.
type Param struct {
	Name    string
	Default Expr // nil if required
}

// Assign is a top-level or let `name = value;`.
type Assign struct {
	Name  string
	Value Expr
	P     Pos
}

// For is `for (var = range) <child>`. Multiple iterators allowed.
type For struct {
	Iters    []ForIter
	Children []Stmt
	P        Pos
}

// ForIter is a single `var = range` clause of a for statement.
type ForIter struct {
	Var   string
	Range Expr // *Range or a vector expression
}

// If is `if (cond) <then> [else <else>]` in statement position.
type If struct {
	Cond Expr
	Then []Stmt
	Else []Stmt
	P    Pos
}

// Use is a `use <path>` library reference (parsed, not yet resolved).
type Use struct {
	Path string
	P    Pos
}

// Include is an `include <path>` library reference (parsed, not yet resolved).
type Include struct {
	Path string
	P    Pos
}

func (*ModuleCall) stmtNode()  {}
func (*ModuleDef) stmtNode()   {}
func (*FunctionDef) stmtNode() {}
func (*Assign) stmtNode()      {}
func (*For) stmtNode()         {}
func (*If) stmtNode()          {}
func (*Use) stmtNode()         {}
func (*Include) stmtNode()     {}

func (n *ModuleCall) Pos() Pos  { return n.P }
func (n *ModuleDef) Pos() Pos   { return n.P }
func (n *FunctionDef) Pos() Pos { return n.P }
func (n *Assign) Pos() Pos      { return n.P }
func (n *For) Pos() Pos         { return n.P }
func (n *If) Pos() Pos          { return n.P }
func (n *Use) Pos() Pos         { return n.P }
func (n *Include) Pos() Pos     { return n.P }

// ---- expressions ----

// Num is a numeric literal (kept as text to preserve the original spelling).
type Num struct {
	Text string
	P    Pos
}

// Str is a string literal.
type Str struct {
	Value string
	P     Pos
}

// Ident is an identifier reference; SpecialVar marks $-prefixed names.
type Ident struct {
	Name       string
	SpecialVar bool
	P          Pos
}

// Bool is a boolean literal.
type Bool struct {
	Val bool
	P   Pos
}

// Undef is the OpenSCAD `undef` literal.
type Undef struct{ P Pos }

// Vector is `[a, b, c]`.
type Vector struct {
	Elems []Expr
	P     Pos
}

// ListComp is a list `[...]` holding at least one comprehension clause
// (for/if/let/each); a bracket of only values is a Vector instead. The elements
// are concatenated to build the list.
type ListComp struct {
	Elems []CompElem
	P     Pos
}

// CompElem is one element of a list comprehension: a plain value, or a
// for/if/let/each clause wrapping a nested element. `each` flattens its list into
// the surrounding list.
type CompElem interface{ compElem() }

// ValueElem is a plain value element `expr`.
type ValueElem struct{ X Expr }

// ForElem is `for (var = range, …) body` — body is produced once per iteration
// (multiple clauses form a Cartesian product, matching OpenSCAD).
type ForElem struct {
	Iters []ForIter
	Body  CompElem
}

// IfElem is `if (cond) then [else else]` as a list element.
type IfElem struct {
	Cond       Expr
	Then, Else CompElem // Else is nil when absent
}

// LetElem is `let (binds) body` as a list element.
type LetElem struct {
	Binds []Assign
	Body  CompElem
}

// EachElem is `each expr` — it flattens expr's elements into the list.
type EachElem struct{ X Expr }

func (*ValueElem) compElem() {}
func (*ForElem) compElem()   {}
func (*IfElem) compElem()    {}
func (*LetElem) compElem()   {}
func (*EachElem) compElem()  {}

// Range is `[start:end]` or `[start:step:end]` (OpenSCAD step-MIDDLE order).
type Range struct {
	Start, End Expr
	Step       Expr // nil if absent
	P          Pos
}

// Binary is a binary operation `L op R`.
type Binary struct {
	Op   string
	L, R Expr
	P    Pos
}

// Unary is a prefix operation: `-x` or `!x`.
type Unary struct {
	Op string // "-", "!"
	X  Expr
	P  Pos
}

// Ternary is `cond ? then : else`.
type Ternary struct {
	Cond, Then, Else Expr
	P                Pos
}

// Call is a function call expression: `name(args)`.
type Call struct {
	Name string
	Args []Arg
	P    Pos
}

// Index is `x[index]`.
type Index struct {
	X, Index Expr
	P        Pos
}

// Member is `x.name`.
type Member struct {
	X    Expr
	Name string
	P    Pos
}

// Let is the expression form `let(x = e) body`.
type Let struct {
	Binds []Assign
	Body  Expr
	P     Pos
}

func (*Num) exprNode()      {}
func (*Str) exprNode()      {}
func (*Ident) exprNode()    {}
func (*Bool) exprNode()     {}
func (*Undef) exprNode()    {}
func (*Vector) exprNode()   {}
func (*Range) exprNode()    {}
func (*ListComp) exprNode() {}
func (*Binary) exprNode()   {}
func (*Unary) exprNode()    {}
func (*Ternary) exprNode()  {}
func (*Call) exprNode()     {}
func (*Index) exprNode()    {}
func (*Member) exprNode()   {}
func (*Let) exprNode()      {}

func (n *Num) Pos() Pos      { return n.P }
func (n *Str) Pos() Pos      { return n.P }
func (n *Ident) Pos() Pos    { return n.P }
func (n *Bool) Pos() Pos     { return n.P }
func (n *Undef) Pos() Pos    { return n.P }
func (n *Vector) Pos() Pos   { return n.P }
func (n *Range) Pos() Pos    { return n.P }
func (n *ListComp) Pos() Pos { return n.P }
func (n *Binary) Pos() Pos   { return n.P }
func (n *Unary) Pos() Pos    { return n.P }
func (n *Ternary) Pos() Pos  { return n.P }
func (n *Call) Pos() Pos     { return n.P }
func (n *Index) Pos() Pos    { return n.P }
func (n *Member) Pos() Pos   { return n.P }
func (n *Let) Pos() Pos      { return n.P }
