package parser

// Tests for automatic semicolon insertion (ASI).
//
// Semicolons are now optional — the lexer inserts synthetic semicolons on
// newlines after line-terminating tokens (identifiers, numbers, strings,
// ), ], true, false, yield).  ASI is suppressed when the next non-whitespace
// character is a continuation character: ) ] . & | + - * / % ^

import (
	"testing"
)

// lexTokenTypes returns the sequence of TokenTypes produced by the lexer for src.
func lexTokenTypes(src string) ([]TokenType, error) {
	l := newLexer(src)
	var types []TokenType
	for {
		tok, err := l.Next()
		if err != nil {
			return nil, err
		}
		types = append(types, tok.Type)
		if tok.Type == TokenEOF {
			break
		}
	}
	return types, nil
}

// mustParse is a parse-only helper used by table-driven tests.
func mustParse(t *testing.T, name, src string) *Source {
	t.Helper()
	prog, err := Parse(src, "", SourceUser)
	if err != nil {
		t.Fatalf("%s: unexpected parse error: %v", name, err)
	}
	return prog
}

func mustParseError(t *testing.T, name, src string) error {
	t.Helper()
	_, err := Parse(src, "", SourceUser)
	if err == nil {
		t.Fatalf("%s: expected a parse error, got none", name)
	}
	return err
}

// ---------------------------------------------------------------------------
// Lexer: synthetic semicolons
// ---------------------------------------------------------------------------

func TestASILexer_AfterIdent(t *testing.T) {
	// "x\n" should produce: ident, semi, EOF
	types, err := lexTokenTypes("x\n")
	if err != nil {
		t.Fatal(err)
	}
	want := []TokenType{TokenIdent, TokenSemicolon, TokenEOF}
	if len(types) != len(want) {
		t.Fatalf("got %v, want %v", types, want)
	}
	for i, w := range want {
		if types[i] != w {
			t.Errorf("token[%d]: got %v, want %v", i, types[i], w)
		}
	}
}

func TestASILexer_AfterNumber(t *testing.T) {
	types, err := lexTokenTypes("42\n")
	if err != nil {
		t.Fatal(err)
	}
	if types[1] != TokenSemicolon {
		t.Errorf("expected synthetic semicolon after number, got %v", types[1])
	}
}

func TestASILexer_AfterRParen(t *testing.T) {
	types, err := lexTokenTypes("f()\n")
	if err != nil {
		t.Fatal(err)
	}
	// ident ( ) SEMI EOF
	if types[3] != TokenSemicolon {
		t.Errorf("expected synthetic semicolon after ), got %v", types[3])
	}
}

func TestASILexer_AfterRBracket(t *testing.T) {
	types, err := lexTokenTypes("[1]\n")
	if err != nil {
		t.Fatal(err)
	}
	// [ 1 ] SEMI EOF
	if types[3] != TokenSemicolon {
		t.Errorf("expected synthetic semicolon after ], got %v", types[3])
	}
}

func TestASILexer_AfterTrue(t *testing.T) {
	types, err := lexTokenTypes("true\n")
	if err != nil {
		t.Fatal(err)
	}
	if types[1] != TokenSemicolon {
		t.Errorf("expected synthetic semicolon after true, got %v", types[1])
	}
}

func TestASILexer_AfterFalse(t *testing.T) {
	types, err := lexTokenTypes("false\n")
	if err != nil {
		t.Fatal(err)
	}
	if types[1] != TokenSemicolon {
		t.Errorf("expected synthetic semicolon after false, got %v", types[1])
	}
}

func TestASILexer_AfterYield(t *testing.T) {
	types, err := lexTokenTypes("yield\n")
	if err != nil {
		t.Fatal(err)
	}
	if types[1] != TokenSemicolon {
		t.Errorf("expected synthetic semicolon after yield, got %v", types[1])
	}
}

func TestASILexer_NoSemiAfterRBrace(t *testing.T) {
	// '}' must NOT insert a semicolon — "} else {" would otherwise break.
	types, err := lexTokenTypes("}\nelse")
	if err != nil {
		t.Fatal(err)
	}
	// rbrace else EOF — no semicolon between } and else
	if types[1] == TokenSemicolon {
		t.Error("unexpected synthetic semicolon after }: would break '} else {' on separate lines")
	}
}

func TestASILexer_NoSemiAfterComma(t *testing.T) {
	types, err := lexTokenTypes("a,\nb")
	if err != nil {
		t.Fatal(err)
	}
	// ident , ident SEMI EOF — no semicolon immediately after the comma
	for i := 1; i < len(types); i++ {
		if types[i] == TokenSemicolon && types[i-1] == TokenComma {
			t.Errorf("unexpected synthetic semicolon immediately after comma: %v", types)
		}
	}
}

func TestASILexer_EOFWithoutTrailingNewline(t *testing.T) {
	// A file that ends with no newline should still get a final semicolon.
	types, err := lexTokenTypes("x")
	if err != nil {
		t.Fatal(err)
	}
	// ident SEMI EOF
	if len(types) < 2 || types[1] != TokenSemicolon {
		t.Errorf("expected synthetic semicolon at EOF without trailing newline, got %v", types)
	}
}

func TestASILexer_BlankLinesNoDuplicate(t *testing.T) {
	// Multiple blank lines between statements must not produce multiple semicolons.
	types, err := lexTokenTypes("x\n\n\ny")
	if err != nil {
		t.Fatal(err)
	}
	semi := 0
	for _, tt := range types {
		if tt == TokenSemicolon {
			semi++
		}
	}
	if semi != 2 {
		t.Errorf("expected exactly 2 semicolons (one after x, one at EOF), got %d in %v", semi, types)
	}
}

// ---------------------------------------------------------------------------
// Lexer: ASI suppression (continuation characters)
// ---------------------------------------------------------------------------

func TestASILexer_SuppressBeforeRParen(t *testing.T) {
	// Multi-line function call: no semicolon before closing ')'
	types, err := lexTokenTypes("f(\n  x\n)")
	if err != nil {
		t.Fatal(err)
	}
	for i, tt := range types {
		if tt == TokenSemicolon && i < len(types)-2 {
			// A semicolon before ) is wrong; one at the very end is fine.
			if i < len(types)-3 {
				t.Errorf("unexpected semicolon at token[%d] before closing ): %v", i, types)
			}
		}
	}
	// Specifically: no semi immediately before the TokenRParen
	for i := 1; i < len(types); i++ {
		if types[i] == TokenRParen && types[i-1] == TokenSemicolon {
			t.Errorf("semicolon inserted immediately before ')': %v", types)
		}
	}
}

func TestASILexer_SuppressBeforeDot(t *testing.T) {
	// Method chain continuation: no semicolon before '.'
	types, err := lexTokenTypes("f()\n.g()")
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(types); i++ {
		if types[i] == TokenDot && types[i-1] == TokenSemicolon {
			t.Errorf("semicolon inserted immediately before '.': %v", types)
		}
	}
}

func TestASILexer_SuppressBeforeAmpAmp(t *testing.T) {
	types, err := lexTokenTypes("x\n&& y")
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(types); i++ {
		if types[i] == TokenAmpAmp && types[i-1] == TokenSemicolon {
			t.Errorf("semicolon inserted before &&: %v", types)
		}
	}
}

func TestASILexer_SuppressBeforePipePipe(t *testing.T) {
	types, err := lexTokenTypes("x\n|| y")
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(types); i++ {
		if types[i] == TokenPipePipe && types[i-1] == TokenSemicolon {
			t.Errorf("semicolon inserted before ||: %v", types)
		}
	}
}

func TestASILexer_SuppressBeforePlus(t *testing.T) {
	types, err := lexTokenTypes("x\n+ y")
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(types); i++ {
		if types[i] == TokenPlus && types[i-1] == TokenSemicolon {
			t.Errorf("semicolon inserted before +: %v", types)
		}
	}
}

func TestASILexer_SuppressBeforeMinus(t *testing.T) {
	types, err := lexTokenTypes("x\n- y")
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(types); i++ {
		if types[i] == TokenMinus && types[i-1] == TokenSemicolon {
			t.Errorf("semicolon inserted before -: %v", types)
		}
	}
}

func TestASILexer_SuppressBeforeStar(t *testing.T) {
	types, err := lexTokenTypes("x\n* y")
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(types); i++ {
		if types[i] == TokenStar && types[i-1] == TokenSemicolon {
			t.Errorf("semicolon inserted before *: %v", types)
		}
	}
}

// ---------------------------------------------------------------------------
// Parser: basic optional semicolons
// ---------------------------------------------------------------------------

func TestASIParse_ReturnNoSemi(t *testing.T) {
	prog := mustParse(t, "return no semi", `
fn Main() {
    return Cube(size: Vec3{x: 10, y: 10, z: 10})
}
`)
	fn := prog.Functions()[0]
	if len(fn.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(fn.Body))
	}
	if _, ok := fn.Body[0].(*ReturnStmt); !ok {
		t.Fatalf("expected ast.ReturnStmt, got %T", fn.Body[0])
	}
}

func TestASIParse_VarNoSemi(t *testing.T) {
	prog := mustParse(t, "var no semi", `
fn Main() {
    var x = 10
    return Cube(size: Vec3{x: x, y: x, z: x})
}
`)
	fn := prog.Functions()[0]
	if len(fn.Body) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(fn.Body))
	}
	if _, ok := fn.Body[0].(*VarStmt); !ok {
		t.Fatalf("stmt[0]: expected ast.VarStmt, got %T", fn.Body[0])
	}
}

func TestASIParse_ImplicitReturn(t *testing.T) {
	// Bare expression at end of body is now ExprStmt (no auto-return).
	prog := mustParse(t, "bare expr no semi", `
fn Main() {
    Cube(size: Vec3{x: 10, y: 10, z: 10})
}
`)
	fn := prog.Functions()[0]
	if len(fn.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(fn.Body))
	}
	if _, ok := fn.Body[0].(*ExprStmt); !ok {
		t.Fatalf("expected ast.ExprStmt, got %T", fn.Body[0])
	}
}

func TestASIParse_ImplicitReturnWithExplicitSemi(t *testing.T) {
	// 'expr;' at the end of a body is ExprStmt (no auto-return).
	prog := mustParse(t, "bare expr explicit semi", `
fn Main() {
    Cube(size: Vec3{x: 10, y: 10, z: 10});
}
`)
	fn := prog.Functions()[0]
	if _, ok := fn.Body[0].(*ExprStmt); !ok {
		t.Fatalf("expected ast.ExprStmt, got %T", fn.Body[0])
	}
}

func TestASIParse_MultiStatement(t *testing.T) {
	prog := mustParse(t, "multi statement no semis", `
fn Main() {
    var a = 10
    var b = 20
    var c = 30
    Cube(size: Vec3{x: a, y: b, z: c})
}
`)
	fn := prog.Functions()[0]
	if len(fn.Body) != 4 {
		t.Fatalf("expected 4 statements, got %d", len(fn.Body))
	}
}

func TestASIParse_GlobalVarNoSemi(t *testing.T) {
	prog := mustParse(t, "global var no semi", `
var size = 10

fn Main() {
    Cube(size: Vec3{x: size, y: size, z: size})
}
`)
	if len(prog.Globals()) != 1 {
		t.Fatalf("expected 1 global, got %d", len(prog.Globals()))
	}
	if prog.Globals()[0].Name != "size" {
		t.Errorf("global name = %q, want size", prog.Globals()[0].Name)
	}
}

func TestASIParse_AssertNoSemi(t *testing.T) {
	mustParse(t, "assert no semi", `
fn Main() {
    var x = 5
    assert x > 0
    Cube(size: Vec3{x: x, y: x, z: x})
}
`)
}

func TestASIParse_AssignmentNoSemi(t *testing.T) {
	mustParse(t, "assignment no semi", `
fn Main() {
    var x = 5
    x = 10
    Cube(size: Vec3{x: x, y: x, z: x})
}
`)
}

func TestASIParse_IfElseNoSemi(t *testing.T) {
	// 'else' on a new line after '}' must still parse — } is not a line-terminator.
	prog := mustParse(t, "if-else on separate lines", `
fn Main() {
    if true {
        return Cube(size: Vec3{x: 10, y: 10, z: 10});
    } else {
        return Sphere(radius: 5);
    }
}
`)
	fn := prog.Functions()[0]
	if len(fn.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(fn.Body))
	}
	if _, ok := fn.Body[0].(*IfStmt); !ok {
		t.Fatalf("expected ast.IfStmt, got %T", fn.Body[0])
	}
}

func TestASIParse_IfElseIfNoSemi(t *testing.T) {
	mustParse(t, "if-elseif-else on separate lines", `
fn Main() {
    if false {
        return Cube(size: Vec3{x: 1, y: 1, z: 1})
    } else if true {
        return Sphere(radius: 5)
    } else {
        return Cube(size: Vec3{x: 2, y: 2, z: 2})
    }
}
`)
}

func TestASIParse_ForYieldNoSemi(t *testing.T) {
	mustParse(t, "for-yield no semis", `
fn Main() {
    var arr = for i [0:3] {
        yield i
    }
    Cube(size: Vec3{x: 10, y: 10, z: 10})
}
`)
}

func TestASIParse_BareYieldASI(t *testing.T) {
	// "yield\n" (no value) should become "yield ;" via ASI.
	mustParse(t, "bare yield via ASI", `
fn Main() {
    var arr = for i [0:3] {
        if i == 1 {
            yield
        }
        yield i
    }
    Cube(size: Vec3{x: 10, y: 10, z: 10})
}
`)
}

func TestASIParse_FoldNoSemi(t *testing.T) {
	mustParse(t, "fold no semis", `
fn Main() {
    var arr = []Number[1, 2, 3]
    var sum = fold a, b arr { yield a + b }
    Cube(size: Vec3{x: sum, y: sum, z: sum})
}
`)
}

func TestASIParse_StructDeclNoSemi(t *testing.T) {
	prog := mustParse(t, "struct decl no semis in fields", `
type Pair {
    first Number
    second Number
}

fn Main() {
    Cube(size: Vec3{x: 1, y: 1, z: 1})
}
`)
	if len(prog.StructDecls()) != 1 {
		t.Fatalf("expected 1 struct decl, got %d", len(prog.StructDecls()))
	}
	sd := prog.StructDecls()[0]
	if len(sd.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(sd.Fields))
	}
}

// ---------------------------------------------------------------------------
// Parser: multi-line expressions (continuation suppression)
// ---------------------------------------------------------------------------

func TestASIParse_MethodChainNewLine(t *testing.T) {
	// Method chain with '.' on the next line must not be split.
	prog := mustParse(t, "method chain new line", `
fn Main() {
    Cube(size: Vec3{x: 10, y: 10, z: 10})
        .Translate(v: Vec3{x: 0, y: 0, z: 5})
}
`)
	fn := prog.Functions()[0]
	es := fn.Body[0].(*ExprStmt)
	if _, ok := es.Expr.(*MethodCallExpr); !ok {
		t.Fatalf("expected ast.MethodCallExpr (chain), got %T", es.Expr)
	}
}

func TestASIParse_MultiLineFunctionCall(t *testing.T) {
	// Arguments split across lines: no semicolon before closing ')'.
	mustParse(t, "multi-line call", `
fn Main() {
    Cube(
        size: Vec3{x: 10, y: 20, z: 30}
    )
}
`)
}

func TestASIParse_MultiLineAndChain(t *testing.T) {
	// '&&' on the next line must continue the expression.
	mustParse(t, "multi-line &&", `
fn Main() {
    var ok = true
        && false
    Cube(size: Vec3{x: 1, y: 1, z: 1})
}
`)
}

func TestASIParse_MultiLineOrChain(t *testing.T) {
	mustParse(t, "multi-line ||", `
fn Main() {
    var ok = false
        || true
    Cube(size: Vec3{x: 1, y: 1, z: 1})
}
`)
}

func TestASIParse_MultiLineAddition(t *testing.T) {
	mustParse(t, "multi-line +", `
fn Main() {
    var x = 5
        + 5
    Cube(size: Vec3{x: x, y: x, z: x})
}
`)
}

func TestASIParse_MultiLineSubtraction(t *testing.T) {
	mustParse(t, "multi-line -", `
fn Main() {
    var x = 20
        - 5
    Cube(size: Vec3{x: x, y: x, z: x})
}
`)
}

func TestASIParse_MultiLineStructLit(t *testing.T) {
	// Struct literal fields on separate lines must not break on the closing '}'.
	mustParse(t, "multi-line struct literal", `
type Point {
    x Number
    y Number
}

fn MakePoint(a, b Number) Point {
    Point{
        x: a,
        y: b
    }
}

fn Main() {
    Cube(size: Vec3{x: 1, y: 1, z: 1})
}
`)
}

// ---------------------------------------------------------------------------
// Parser: backward compatibility — explicit semicolons still work
// ---------------------------------------------------------------------------

func TestASIParse_BackwardCompatExplicitSemis(t *testing.T) {
	mustParse(t, "explicit semicolons still work", `
fn Main() {
    var x = 10;
    var y = 20;
    return Cube(size: Vec3{x: x, y: y, z: x});
}
`)
}

func TestASIParse_BackwardCompatMixed(t *testing.T) {
	// Mix of explicit semicolons and no semicolons.
	mustParse(t, "mixed explicit and no semis", `
fn Main() {
    var x = 10;
    var y = 20
    Cube(size: Vec3{x: x, y: y, z: x})
}
`)
}

func TestASIParse_BackwardCompatOneLiner(t *testing.T) {
	mustParse(t, "one-liner with semis", `
fn Main() { var x = 5; return Cube(size: Vec3{x: x, y: x, z: x}); }
`)
}

// ---------------------------------------------------------------------------
// Parser: ASI interaction with comments
// ---------------------------------------------------------------------------

func TestASIParse_CommentAtLineEnd(t *testing.T) {
	// A trailing comment must not suppress the ASI that follows the token before it.
	mustParse(t, "trailing comment preserves ASI", `
fn Main() {
    var x = 10 # this is x
    return Cube(size: Vec3{x: x, y: x, z: x})
}
`)
}

func TestASIParse_CommentOnlyLines(t *testing.T) {
	// Comment-only lines between statements must not insert spurious semicolons.
	mustParse(t, "comment-only lines", `
fn Main() {
    # first
    var x = 10
    # second
    return Cube(size: Vec3{x: x, y: x, z: x})
}
`)
}

// ---------------------------------------------------------------------------
// Error cases: these must still produce errors
// ---------------------------------------------------------------------------

func TestASIParse_BareExpressionMidBody(t *testing.T) {
	// Bare expressions mid-body are valid (ExprStmt).
	prog := mustParse(t, "bare expression mid-body", `
fn Main() {
    Cube(size: Vec3{x: 10, y: 10, z: 10})
    return Cube(size: Vec3{x: 1, y: 1, z: 1})
}
`)
	fn := prog.Functions()[0]
	if len(fn.Body) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(fn.Body))
	}
	if _, ok := fn.Body[0].(*ExprStmt); !ok {
		t.Fatalf("expected ast.ExprStmt, got %T", fn.Body[0])
	}
	if _, ok := fn.Body[1].(*ReturnStmt); !ok {
		t.Fatalf("expected ast.ReturnStmt, got %T", fn.Body[1])
	}
}
