package parser

import "strings"

// Function represents a function declaration.
// ReceiverType is non-empty for method definitions (e.g. "Solid" for Solid.Translate).
// IsOperator is true for operator functions (e.g. fn +(Vec3 a, b) Vec3 { ... }).
type Function struct {
	ReturnType   string
	Name         string
	ReceiverType string // "" for free functions, "Solid"/"Sketch"/"String" for methods
	IsOperator   bool   // true for operator functions (fn +, fn -, etc.)
	Params       []*Param
	Body         []Stmt
	Pos          Pos       // source position of the function definition
	Comments     []Comment // comments preceding this function
}

// ArgsInRange returns true if nArgs falls within [required, total] for the function,
// where required is the number of parameters without defaults.
func (fn *Function) ArgsInRange(nArgs int) bool {
	required := 0
	for _, p := range fn.Params {
		if p.Default == nil {
			required++
		}
	}
	return nArgs >= required && nArgs <= len(fn.Params)
}

// CollectCandidates filters fns by name (and optionally receiver type),
// splitting into arity-matching candidates and a first-seen fallback for error
// reporting. Pass nArgs < 0 to skip arity filtering (all matches become candidates).
func CollectCandidates(fns []*Function, name string, nArgs int, checkReceiver bool) (candidates []*Function, fallback *Function) {
	for _, fn := range fns {
		if fn.Name != name || (checkReceiver && fn.ReceiverType != "") {
			continue
		}
		if nArgs >= 0 && !fn.ArgsInRange(nArgs) {
			if fallback == nil {
				fallback = fn
			}
			continue
		}
		candidates = append(candidates, fn)
		if fallback == nil {
			fallback = fn
		}
	}
	return
}

// Param represents a typed function parameter.
// Default is non-nil for optional parameters (e.g. Length height = 5 mm).
// Constraint is non-nil for constrained parameters (e.g. radius Length where [0:100]).
type Param struct {
	Type       string
	Name       string
	Default    Expr // nil if required
	Constraint Expr // nil if unconstrained
}

// Stmt is the interface implemented by all statement nodes.
type Stmt interface {
	stmtNode()
}

// Expr is the interface implemented by all expression nodes.
type Expr interface {
	exprNode()
}

// Pos represents a source location (line and column).
type Pos struct {
	Line int
	Col  int
}

// Comment represents a source comment.
// IsDoc is true for # (doc comments used for API documentation),
// false for // (user comments preserved in source but not used for docs).
type Comment struct {
	Text       string // content after # or //, leading space stripped
	Pos        Pos    // position of the # or //
	IsDoc      bool   // true for # comments, false for // comments
	IsTrailing bool   // true if comment was at end of a code line
}

// DocComment returns the doc comment string derived from a slice of Comments,
// joining only IsDoc (#) comments with newlines. Returns "" if none.
func DocComment(comments []Comment) string {
	var lines []string
	for _, c := range comments {
		if c.IsDoc {
			lines = append(lines, c.Text)
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
}

// ReturnStmt represents a return statement.
type ReturnStmt struct {
	Value    Expr
	Pos      Pos
	Comments []Comment
}

func (*ReturnStmt) stmtNode() {}

// VarStmt represents a variable declaration: var/const name = expr;
// Constraint is non-nil for constrained variables (e.g. var x = 10 where [0:100];)
type VarStmt struct {
	Name       string
	Value      Expr
	IsConst    bool      // true for const declarations
	Constraint Expr      // nil if unconstrained; *RangeExpr, *ConstrainedRange, or *ArrayLitExpr
	Pos        Pos       // source position for param extraction + error reporting
	Comments   []Comment // comments preceding this statement
}

func (*VarStmt) stmtNode() {}

// CallExpr represents a function call expression.
type CallExpr struct {
	Name string
	Args []Expr
	Pos  Pos
}

func (*CallExpr) exprNode() {}

// NamedArg wraps a named argument at a call site: name: expr
type NamedArg struct {
	Name  string
	Value Expr
	Pos   Pos
}

func (*NamedArg) exprNode() {}

// MethodCallExpr represents a method call on a receiver: receiver.Method(args).
type MethodCallExpr struct {
	Receiver Expr
	Method   string
	Args     []Expr
	Pos      Pos
}

func (*MethodCallExpr) exprNode() {}

// IdentExpr represents an identifier used as an expression.
type IdentExpr struct {
	Name string
	Pos  Pos
}

func (*IdentExpr) exprNode() {}

// NumberLit represents a numeric literal.
// Raw preserves the original token text (e.g. "1/2", "3.14") for lossless formatting.
type NumberLit struct {
	Value float64
	Raw   string // original source text
}

func (*NumberLit) exprNode() {}


// UnaryExpr represents a unary operation (e.g. -x, !b).
type UnaryExpr struct {
	Op      string // "-" or "!"
	Operand Expr
	Pos     Pos
}

func (*UnaryExpr) exprNode() {}

// BinaryExpr represents a binary operation (e.g. a + b, x * y).
type BinaryExpr struct {
	Op    string
	Left  Expr
	Right Expr
	Pos   Pos
}

func (*BinaryExpr) exprNode() {}

// ArrayLitExpr represents an array literal: [elem, elem, ...]
// When TypeName is non-empty, it's a typed array constructor: TypeName[elem, ...]
// and each element is coerced to TypeName.
type ArrayLitExpr struct {
	TypeName string // "" for untyped, "Foo" for Foo[...]
	Elems    []Expr
	Pos      Pos
}

func (*ArrayLitExpr) exprNode() {}

// RangeExpr represents a range expression: [start:end] or [start:end:step].
// Exclusive is true when < or > modifier is used on the end bound.
type RangeExpr struct {
	Start, End Expr
	Step       Expr // nil when not specified (defaults to 1 or -1)
	Exclusive  bool // true when < or > modifier used
	Pos        Pos
}

func (*RangeExpr) exprNode() {}

// ConstrainedRange wraps a RangeExpr with an optional unit suffix.
// Syntax: [1:100] mm — applies the unit to all range values.
type ConstrainedRange struct {
	Range *RangeExpr
	Unit  string // "mm", "cm", "deg", etc. — "" if no unit suffix
}

func (*ConstrainedRange) exprNode() {}

// ForClause represents a single iteration variable and its iterable.
// Index is non-empty for enumerate syntax: for i, v arr { ... }
type ForClause struct {
	Index string // optional enumerate index variable ("" if not used)
	Var   string
	Iter  Expr
	Pos   Pos
}

// ForYieldExpr represents a for-yield loop that collects values into an array.
// Single: for IDENT expr { ... yield expr; ... }
// Multi:  for IDENT expr, IDENT expr { ... yield expr; ... }  (cartesian product)
type ForYieldExpr struct {
	Clauses []*ForClause
	Body    []Stmt
	Pos     Pos
}

func (*ForYieldExpr) exprNode() {}

// FoldExpr represents a fold expression that reduces an array with an accumulator.
// Syntax: fold accVar, elemVar expr { ... yield expr; ... }
// The first element of the array becomes the initial accumulator value,
// and iteration starts from the second element. Use `yield` to update the accumulator.
type FoldExpr struct {
	AccVar  string
	ElemVar string
	Iter    Expr
	Body    []Stmt
	Pos     Pos
}

func (*FoldExpr) exprNode() {}

// YieldStmt represents a yield statement inside a for-yield body.
type YieldStmt struct {
	Value    Expr
	Pos      Pos
	Comments []Comment
}

func (*YieldStmt) stmtNode() {}

// AssignStmt represents a variable reassignment: name = expr;
type AssignStmt struct {
	Name     string
	Value    Expr
	Pos      Pos
	Comments []Comment
}

func (*AssignStmt) stmtNode() {}

// FieldAssignStmt represents a struct field assignment: receiver.field = expr;
type FieldAssignStmt struct {
	Receiver Expr      // the struct expression (e.g., IdentExpr for "f")
	Field    string    // field name
	Value    Expr      // new value
	Pos      Pos
	Comments []Comment
}

func (*FieldAssignStmt) stmtNode() {}

// ExprStmt wraps an expression used as a statement (result discarded).
// Used for side-effecting expressions like if-guards in for-yield bodies.
type ExprStmt struct {
	Expr     Expr
	Pos      Pos
	Comments []Comment
}

func (*ExprStmt) stmtNode() {}

// AssertStmt represents an assert statement: assert cond[, msg] or assert expr where constraint.
type AssertStmt struct {
	Cond       Expr
	Message    Expr // optional message expression (must evaluate to String), nil if none
	Value      Expr // non-nil for "assert EXPR where CONSTRAINT" form
	Constraint Expr // constraint (RangeExpr, ConstrainedRange, ArrayLitExpr), nil if none
	Pos        Pos
	Comments   []Comment
}

func (*AssertStmt) stmtNode() {}

// BoolLit represents a boolean literal (true or false).
type BoolLit struct {
	Value bool
}

func (*BoolLit) exprNode() {}

// IfStmt represents an if/else-if/else statement.
type IfStmt struct {
	Cond     Expr
	Then     []Stmt
	ElseIfs  []*ElseIfClause
	Else     []Stmt // nil if no else
	Pos      Pos
	Comments []Comment
}

func (*IfStmt) stmtNode() {}

// ElseIfClause represents an "else if" branch.
type ElseIfClause struct {
	Cond     Expr
	Body     []Stmt
	Pos      Pos
	Comments []Comment
}

// StringLit represents a string literal: "hello"
type StringLit struct {
	Value string
}

func (*StringLit) exprNode() {}

// LibExpr represents a library load expression: lib "facet/gears"
type LibExpr struct {
	Path string
	Pos  Pos
}

func (*LibExpr) exprNode() {}

// StructDecl represents a struct type declaration.
type StructDecl struct {
	Name     string
	Fields   []*StructField
	Pos      Pos
	Comments []Comment // comments preceding this struct declaration
}

// StructField represents a single field in a struct declaration.
// Default is non-nil when the field has a default value (e.g. Number count = 40;).
type StructField struct {
	Type       string
	Name       string
	Default    Expr // nil = required (uses zero value when omitted in anonymous literals)
	Constraint Expr // nil if unconstrained; *RangeExpr, *ConstrainedRange, or *ArrayLitExpr
	Comments   []Comment
}

// StructLitExpr represents a struct literal: TypeName { field: value, ... }
type StructLitExpr struct {
	TypeName string
	Fields   []*StructFieldInit
	Pos      Pos
}

func (*StructLitExpr) exprNode() {}

// StructFieldInit represents a field initializer: name: expr
type StructFieldInit struct {
	Name  string
	Value Expr
}

// FieldAccessExpr represents field access: receiver.field
type FieldAccessExpr struct {
	Receiver Expr
	Field    string
	Pos      Pos
}

func (*FieldAccessExpr) exprNode() {}

// IndexExpr represents array index access: receiver[index]
type IndexExpr struct {
	Receiver Expr
	Index    Expr
	Pos      Pos
}

func (*IndexExpr) exprNode() {}

// SliceExpr represents array slicing: receiver[start:end]
// Start and End are optional (nil means from beginning / to end).
type SliceExpr struct {
	Receiver Expr
	Start    Expr // nil = from beginning
	End      Expr // nil = to end
	Pos      Pos
}

func (*SliceExpr) exprNode() {}

// UnitExpr applies a unit suffix to an arbitrary expression: (1/2) mm, Foo() deg
// Factor is the conversion factor (mm for scalars, degrees for angles).
type UnitExpr struct {
	Expr    Expr
	Unit    string
	Factor  float64
	IsAngle bool
	Pos     Pos
}

func (*UnitExpr) exprNode() {}

// LambdaExpr represents a lambda (anonymous function) expression.
// Syntax: fn(params) ReturnType { body }
type LambdaExpr struct {
	Params     []*Param
	ReturnType string // "" = void/inferred
	Body       []Stmt
	Pos        Pos
}

func (*LambdaExpr) exprNode() {}
