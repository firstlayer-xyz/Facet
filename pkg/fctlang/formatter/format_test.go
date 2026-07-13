package formatter

import (
	"facet/pkg/fctlang/parser"
	"strings"
	"testing"
)

// formatString is a test helper that parses source and formats it.
func formatString(source string) string {
	src, err := parser.Parse(source, "", parser.SourceUser)
	if err != nil {
		return source
	}
	return Format(src)
}

func TestFormat(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty source",
			input: "",
			want:  "\n",
		},
		{
			name:  "already correct simple function",
			input: "fn Main() {\n    return 1\n}\n",
			want:  "fn Main() { return 1 }\n",
		},
		{
			name:  "re-indent over-indented body",
			input: "fn Main() {\n        var x = 1\n        return x\n}\n",
			want:  "fn Main() {\n    var x = 1\n    return x\n}\n",
		},
		{
			name:  "re-indent under-indented body",
			input: "fn Main() {\nvar x = 1\nreturn x\n}\n",
			want:  "fn Main() {\n    var x = 1\n    return x\n}\n",
		},
		{
			name: "if else",
			input: `fn Main() {
if true {
return 1
} else {
return 2
}
}
`,
			want: `fn Main() {
    if true {
        return 1
    } else {
        return 2
    }
}
`,
		},
		{
			name: "for-yield body collapses when short",
			input: `fn Main() {
var arr = for i [0:<3] {
yield i * 2
}
return arr
}
`,
			want: `fn Main() {
    var arr = for i [0:<3] { yield i * 2 }
    return arr
}
`,
		},
		{
			name: "for-yield body stays multi-line when too long",
			input: `fn Main() {
var arr = for i [0:<3] {
yield Cube(x: 10 mm, y: 20 mm, z: 30 mm).Move(v: Vec3{x: i * 5 mm, y: 0 mm, z: 0 mm})
}
return arr
}
`,
			want: `fn Main() {
    var arr = for i [0:<3] {
        yield Cube(x: 10 mm, y: 20 mm, z: 30 mm)
            .Move(v: Vec3{x: i * 5 mm, y: 0 mm, z: 0 mm})
    }
    return arr
}
`,
		},
		{
			name: "blank lines between var groups preserved",
			input: `var x = 10 mm

var y = 20 mm

fn Main() {
return x + y
}
`,
			want: `var x = 10 mm

var y = 20 mm

fn Main() { return x + y }
`,
		},
		{
			name: "blank line inserted before function",
			input: `var x = 10 mm
var y = 20 mm
fn Main() {
return x + y
}
`,
			want: `var x = 10 mm
var y = 20 mm

fn Main() { return x + y }
`,
		},
		{
			name: "doc comments preserved",
			input: `# Top comment
fn Main() {
return 1
}
`,
			want: `# Top comment
fn Main() { return 1 }
`,
		},
		{
			name: "user comments preserved",
			input: `// User comment
fn Main() {
return 1
}
`,
			want: `// User comment
fn Main() { return 1 }
`,
		},
		{
			name: "end-of-line comment on var",
			input: `var x = 10 mm // width
var y = 20 mm
fn Main() {
return x + y
}
`,
			want: `var x = 10 mm // width
var y = 20 mm

fn Main() { return x + y }
`,
		},
		{
			name: "end-of-line comment on return",
			input: `fn Main() {
return 1 // the answer
}
`,
			want: `fn Main() {
    return 1 // the answer
}
`,
		},
		{
			name: "method definition",
			input: `fn Solid.Scale(f Number) Solid {
return self
}
`,
			want: "fn Solid.Scale(f Number) Solid { return self }\n",
		},
		{
			name: "deeply nested",
			input: `fn Main() {
if side_length_is_a_fairly_long_name > 10 mm {
if another_descriptive_condition_name {
return ComputeSomethingReasonablyVerbose()
}
}
return 0
}
`,
			want: `fn Main() {
    if side_length_is_a_fairly_long_name > 10 mm {
        if another_descriptive_condition_name {
            return ComputeSomethingReasonablyVerbose()
        }
    }
    return 0
}
`,
		},
		{
			name: "short if written multi-line stays multi-line",
			input: `fn Main() {
var x = 1 mm
if x > 0 mm {
x = 2 mm
}
return x
}
`,
			want: `fn Main() {
    var x = 1 mm
    if x > 0 mm {
        x = 2 mm
    }
    return x
}
`,
		},
		{
			name: "short if written inline stays inline",
			input: `fn Main() {
var x = 1 mm
if x > 0 mm { x = 2 mm }
return x
}
`,
			want: `fn Main() {
    var x = 1 mm
    if x > 0 mm { x = 2 mm }
    return x
}
`,
		},
		{
			name: "if with else stays multi-line",
			input: `fn Main() {
if true { return 1 } else { return 2 }
}
`,
			want: `fn Main() {
    if true {
        return 1
    } else {
        return 2
    }
}
`,
		},
		{
			name: "struct declaration",
			input: `type Point {
x Length
y Length
}
`,
			want: `type Point {
    x Length
    y Length
}
`,
		},
		{
			name: "struct literal",
			input: `fn Main() {
return Point{x: 1 mm, y: 2 mm}
}
`,
			want: "fn Main() { return Point{x: 1 mm, y: 2 mm} }\n",
		},
		{
			name: "struct literal preserves explicit zeros",
			input: `fn Main() {
return Vec3{x: 1 mm, y: 0 mm, z: 0 mm}
}
`,
			want: "fn Main() { return Vec3{x: 1 mm, y: 0 mm, z: 0 mm} }\n",
		},
		{
			name: "method chain splits on dot",
			input: `fn Main() {
return Cube(x: 10 mm, y: 10 mm, z: 10 mm).Move(v: Vec3{x: 5 mm, y: 0 mm, z: 0 mm})
}
`,
			want: `fn Main() {
    return Cube(x: 10 mm, y: 10 mm, z: 10 mm)
        .Move(v: Vec3{x: 5 mm, y: 0 mm, z: 0 mm})
}
`,
		},
		{
			name: "binary expression precedence",
			input: `fn Main() {
return 1 + 2 * 3
}
`,
			want: "fn Main() { return 1 + 2 * 3 }\n",
		},
		{
			name: "assert statement",
			input: `fn Main() {
assert true
assert 1 + 1 == 2, "math is broken"
}
`,
			want: `fn Main() {
    assert true
    assert 1 + 1 == 2, "math is broken"
}
`,
		},
		{
			name: "const declaration",
			input: `const PI = 3.14159
`,
			want: `const PI = 3.14159
`,
		},
		{
			name: "var with constraint",
			input: `fn Main() {
var x = 10 where [0:100]
return x
}
`,
			want: `fn Main() {
    var x = 10 where [0:100]
    return x
}
`,
		},
		{
			name: "lib expression",
			input: `var G = lib "facet/gears"
`,
			want: `var G = lib "facet/gears"
`,
		},
		{
			name: "array literal",
			input: `fn Main() {
return []Number[1, 2, 3]
}
`,
			want: "fn Main() { return []Number[1, 2, 3] }\n",
		},
		{
			name: "typed array strips struct type",
			input: `fn Main() {
return []Vec3[Vec3{x: 1, y: 2, z: 3}, Vec3{x: 4, y: 5, z: 6}]
}
`,
			want: "fn Main() { return []Vec3[{x: 1, y: 2, z: 3}, {x: 4, y: 5, z: 6}] }\n",
		},
		{
			name: "long array wraps to multi-line",
			input: `fn Main() {
return []Vec3[Vec3{x: 100 mm, y: 200 mm, z: 300 mm}, Vec3{x: 400 mm, y: 500 mm, z: 600 mm}]
}
`,
			want: `fn Main() {
    return []Vec3[
        {x: 100 mm, y: 200 mm, z: 300 mm}, {x: 400 mm, y: 500 mm, z: 600 mm}
    ]
}
`,
		},
		{
			name: "array of structs splits between structs",
			input: `fn Main() {
return []Vec3[Vec3{x: 1 mm, y: 2 mm, z: 3 mm}, Vec3{x: 4 mm, y: 5 mm, z: 6 mm}, Vec3{x: 7 mm, y: 8 mm, z: 9 mm}, Vec3{x: 10 mm, y: 11 mm, z: 12 mm}]
}
`,
			want: `fn Main() {
    return []Vec3[
        {x: 1 mm, y: 2 mm, z: 3 mm}, {x: 4 mm, y: 5 mm, z: 6 mm},
        {x: 7 mm, y: 8 mm, z: 9 mm}, {x: 10 mm, y: 11 mm, z: 12 mm}
    ]
}
`,
		},
		{
			name: "array of numbers packs per line",
			input: `fn Main() {
return [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25]
}
`,
			want: `fn Main() {
    return [
        1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20,
        21, 22, 23, 24, 25
    ]
}
`,
		},
		{
			name: "long struct literal wraps fields",
			input: `fn Main() {
return SomeType{alpha: 100 mm, beta: 200 mm, gamma: 300 mm, delta: 400 mm, epsilon: 500 mm}
}
`,
			want: `fn Main() {
    return SomeType{alpha: 100 mm, beta: 200 mm, gamma: 300 mm, delta: 400 mm,
        epsilon: 500 mm}
}
`,
		},
		{
			name: "long call wraps args",
			input: `fn Main() {
return SomeFunction(alpha: 100 mm, beta: 200 mm, gamma: 300 mm, delta: 400 mm)
}
`,
			want: `fn Main() {
    return SomeFunction(
        alpha: 100 mm,
        beta: 200 mm,
        gamma: 300 mm,
        delta: 400 mm
    )
}
`,
		},
		{
			name: "fold expression collapses when short",
			input: `fn Main() {
return fold acc, elem [1, 2, 3] {
yield acc + elem
}
}
`,
			want: "fn Main() { return fold acc, elem [1, 2, 3] { yield acc + elem } }\n",
		},
		{
			name: "field assignment",
			input: `fn Solid.Foo() Solid {
var p = Point{x: 0 mm, y: 0 mm}
p.x = 10 mm
return self
}
`,
			want: `fn Solid.Foo() Solid {
    var p = Point{x: 0 mm, y: 0 mm}
    p.x = 10 mm
    return self
}
`,
		},
		{
			name: "param with default",
			input: `fn Foo(x Length = 10) Length {
var y = x
return y
}
`,
			want: `fn Foo(
    x Length = 10,
) Length {
    var y = x
    return y
}
`},
		{
			name: "multi-line params group consecutive same-type without defaults",
			input: `fn Frustum(r1 Length, r2 Length, h Length, segments Number = 0) Solid {
var out = r1
return out
}
`,
			want: `fn Frustum(
    r1, r2, h Length,
    segments Number = 0,
) Solid {
    var out = r1
    return out
}
`},
		{
			name: "multi-line params keep distinct types on separate group lines",
			input: `fn Foo(a Length, b Length, c Number, d Number, e Angle = 0 deg) Solid {
var out = a
return out
}
`,
			want: `fn Foo(
    a, b Length,
    c, d Number,
    e Angle = 0 deg,
) Solid {
    var out = a
    return out
}
`},
		{
			// Grouping Any params shares a type slot (the checker forces them to
			// one concrete type), so independently-declared Any params must NOT
			// be merged — that would change the program's meaning.
			name: "independently-declared generic Any params are not merged single-line",
			input: `fn Foo(a Any, b Any) Any {
return a
}
`,
			want: "fn Foo(a Any, b Any) Any { return a }\n",
		},
		{
			name: "independently-declared generic Any params are not merged multi-line",
			input: `fn Foo(a Any, b Any, c Number = 0) Any {
var out = a
return out
}
`,
			want: `fn Foo(
    a Any,
    b Any,
    c Number = 0,
) Any {
    var out = a
    return out
}
`},
		{
			// Author-declared generic groups DO share a type slot, so they must
			// be preserved exactly as grouped.
			name: "author-grouped generic Any params are preserved",
			input: `fn Foo(a, b Any, c Number = 0) Any {
var out = a
return out
}
`,
			want: `fn Foo(
    a, b Any,
    c Number = 0,
) Any {
    var out = a
    return out
}
`},
		{
			name: "fraction preserved",
			input: `fn Main() {
return 1/2 mm
}
`,
			want: "fn Main() { return 1/2 mm }\n",
		},
		{
			name: "fraction without unit preserved",
			input: `fn Main() {
return 1/2
}
`,
			want: "fn Main() { return 1/2 }\n",
		},
		{
			name: "parenthesized division with unit",
			input: `fn Main() {
return (1 / 2) mm
}
`,
			want: "fn Main() { return (1 / 2) mm }\n",
		},
		{
			name: "parenthesized division without unit",
			input: `fn Main() {
return (1 / 2)
}
`,
			want: "fn Main() { return 1 / 2 }\n",
		},
		{
			name: "ternary",
			input: `fn Main() {
return a > 0 ? 1 : 2
}
`,
			want: "fn Main() { return a > 0 ? 1 : 2 }\n",
		},
		{
			name: "nested ternary right-associative",
			input: `fn Main() {
return a ? 1 : b ? 2 : 3
}
`,
			want: "fn Main() { return a ? 1 : b ? 2 : 3 }\n",
		},
		{
			name: "ternary inside binary needs parens",
			input: `fn Main() {
return 1 + (a ? 2 : 3)
}
`,
			want: "fn Main() { return 1 + (a ? 2 : 3) }\n",
		},
		{
			name: "nil literal",
			input: `fn Lookup() Number? {
return nil
}
`,
			want: "fn Lookup() Number? { return nil }\n",
		},
		{
			name: "null-coalesce binds tighter than or",
			input: `fn Main() {
return a || x ?? 0
}
`,
			want: "fn Main() { return a || x ?? 0 }\n",
		},
		{
			name: "optional field access",
			input: `fn Main() {
return p?.x
}
`,
			want: "fn Main() { return p?.x }\n",
		},
		{
			name: "optional method call",
			input: `fn Main() {
return p?.Norm()
}
`,
			want: "fn Main() { return p?.Norm() }\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := formatString(tc.input)
			if got != tc.want {
				t.Errorf("Format() mismatch\ninput:\n%s\ngot:\n%s\nwant:\n%s",
					annotate(tc.input), annotate(got), annotate(tc.want))
			}
		})
	}
}

// TestFormatIdempotent verifies that running Format twice produces the same
// result as running it once.
func TestFormatIdempotent(t *testing.T) {
	sources := []string{
		"fn Main() {\n    return 1\n}\n",
		"var x = 10 mm\n\nfn Main() {\n    return x\n}\n",
		"# Doc comment\nfn Main() {\n    return 1\n}\n",
		"fn Main() {\n    var arr = for i [0:<3] {\n        yield i * 2\n    }\n    return arr\n}\n",
	}
	for _, src := range sources {
		first := formatString(src)
		second := formatString(first)
		if first != second {
			t.Errorf("Format not idempotent for:\n%s\nfirst:\n%s\nsecond:\n%s",
				src, annotate(first), annotate(second))
		}
	}
}

// TestFormatRangeBoundModifier pins that the formatter reproduces the exact
// range end-bound modifier the user wrote. Deriving it from a bool rewrote `>`
// to `<` and dropped explicit `<=`/`>=`.
func TestFormatRangeBoundModifier(t *testing.T) {
	for _, mod := range []string{"", "<", ">", "<=", ">="} {
		src := "fn Main() {\n    var arr = for i [0:" + mod + "3] {\n        yield i\n    }\n    return arr\n}\n"
		got := formatString(src)
		want := "[0:" + mod + "3]"
		if !strings.Contains(got, want) {
			t.Errorf("range modifier %q not preserved: want %q in:\n%s", mod, want, got)
		}
	}
}

func TestFormatSliceExpr(t *testing.T) {
	input := "fn Main() {\n    var x = [1, 2, 3]\n    var a = x[1:3]\n    var b = x[:2]\n    var c = x[1:]\n    return Cube(s: 10 mm)\n}\n"
	got := formatString(input)
	if !strings.Contains(got, "x[1:3]") {
		t.Errorf("expected x[1:3] in output, got:\n%s", got)
	}
	if !strings.Contains(got, "x[:2]") {
		t.Errorf("expected x[:2] in output, got:\n%s", got)
	}
	if !strings.Contains(got, "x[1:]") {
		t.Errorf("expected x[1:] in output, got:\n%s", got)
	}
}

func TestFormatIfVarBinding(t *testing.T) {
	// The `if var NAME = Cond` optional-narrowing binding must survive
	// formatting — dropping it silently rewrites valid code to broken (NAME
	// undefined). Parse explicitly so a parse failure can't mask the bug.
	src := "fn Main() {\n    if var x = Maybe() {\n        return x\n    }\n    return 0\n}\n"
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	got := Format(prog)
	if !strings.Contains(got, "if var x = ") {
		t.Errorf("if-var binding dropped on format; got:\n%s", got)
	}
	prog2, err := parser.Parse(got, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("reparse of formatted output failed: %v", err)
	}
	if Format(prog2) != got {
		t.Errorf("format not idempotent for if-var:\n%s", got)
	}
}

func TestFormatStringEscapes(t *testing.T) {
	input := "fn Main() {\n    var x = \"hello\\nworld\"\n    return Cube(s: 10 mm)\n}\n"
	got := formatString(input)
	if !strings.Contains(got, `"hello\nworld"`) {
		t.Errorf("expected escaped string in output, got:\n%s", got)
	}
}

func TestFormatNegativeIndex(t *testing.T) {
	input := "fn Main() {\n    var x = [1, 2, 3]\n    var a = x[-1]\n    return Cube(s: a * 1 mm)\n}\n"
	got := formatString(input)
	if !strings.Contains(got, "x[-1]") {
		t.Errorf("expected x[-1] in output, got:\n%s", got)
	}
}

// TestFormatPreservesPrecedenceParens guards two semantics-changing paren drops:
// a non-associative comparison chain that would no longer parse, and a prefix
// unary receiver that would reparse with a flipped meaning.
func TestFormatPreservesPrecedenceParens(t *testing.T) {
	cmp := formatString("fn Main() Bool {\n    var a = true\n    var b = false\n    var c = true\n    return (a == b) == c\n}\n")
	if !strings.Contains(cmp, "(a == b) == c") {
		t.Errorf("comparison chain lost its parens:\n%s", cmp)
	}
	if _, err := parser.Parse(cmp, "", parser.SourceUser); err != nil {
		t.Errorf("formatted comparison no longer parses (%v):\n%s", err, cmp)
	}

	un := formatString("fn Number.Sq() Number {\n    return self * self\n}\nfn Main() Number {\n    var x = 3\n    return (-x).Sq()\n}\n")
	if !strings.Contains(un, "(-x).Sq()") {
		t.Errorf("unary receiver lost its parens (would flip sign on reparse):\n%s", un)
	}
}

// annotate replaces newlines with ↵\n so diff lines are visible in test output.
func annotate(s string) string {
	return strings.ReplaceAll(s, "\n", "↵\n")
}

// A comment between array-literal elements must stay inside the array (next to
// its element), not leak forward onto the following statement.
func TestFormatArrayInteriorComment(t *testing.T) {
	src := "var xs = [\n    1,\n    // keep me\n    2,\n]\nvar y = 3\n"
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out := Format(prog)
	if !strings.Contains(out, "// keep me") {
		t.Fatalf("comment dropped:\n%s", out)
	}
	// The comment must appear inside the array (before ']') and before `var y`.
	if strings.Index(out, "// keep me") > strings.Index(out, "]") {
		t.Fatalf("comment leaked out of the array:\n%s", out)
	}
	// `var y = 3` must not carry the comment as its own leading comment.
	if strings.Contains(out, "// keep me\nvar y") {
		t.Fatalf("comment leaked onto the next statement:\n%s", out)
	}
	// Reparses and is idempotent.
	prog2, err := parser.Parse(out, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("reparse formatted output: %v\n%s", err, out)
	}
	if got := Format(prog2); got != out {
		t.Fatalf("not idempotent:\nfirst:\n%s\nsecond:\n%s", out, got)
	}
}

// A comment between struct-literal fields must stay inside the literal.
func TestFormatStructInteriorComment(t *testing.T) {
	src := "type P {\n    a Number\n    b Number\n}\nvar p = P{\n    a: 1,\n    // keep me\n    b: 2,\n}\nvar y = 3\n"
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out := Format(prog)
	if !strings.Contains(out, "// keep me") {
		t.Fatalf("comment dropped:\n%s", out)
	}
	if strings.Index(out, "// keep me") > strings.LastIndex(out, "}") {
		t.Fatalf("comment leaked out of the struct literal:\n%s", out)
	}
	if strings.Contains(out, "// keep me\nvar y") {
		t.Fatalf("comment leaked onto the next statement:\n%s", out)
	}
	prog2, err := parser.Parse(out, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("reparse formatted output: %v\n%s", err, out)
	}
	if got := Format(prog2); got != out {
		t.Fatalf("not idempotent:\nfirst:\n%s\nsecond:\n%s", out, got)
	}
}

// A comment between typed-array-literal elements is preserved too.
func TestFormatTypedArrayInteriorComment(t *testing.T) {
	src := "var xs = []Vec3[\n    {x: 0 mm, y: 0 mm, z: 0 mm},\n    // keep me\n    {x: 1 mm, y: 1 mm, z: 1 mm},\n]\nvar y = 3\n"
	prog, err := parser.Parse(src, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out := Format(prog)
	if !strings.Contains(out, "// keep me") {
		t.Fatalf("comment dropped:\n%s", out)
	}
	if strings.Index(out, "// keep me") > strings.LastIndex(out, "]") || strings.Contains(out, "// keep me\nvar y") {
		t.Fatalf("comment leaked out of the typed array:\n%s", out)
	}
	prog2, err := parser.Parse(out, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("reparse: %v\n%s", err, out)
	}
	if got := Format(prog2); got != out {
		t.Fatalf("not idempotent:\nfirst:\n%s\nsecond:\n%s", out, got)
	}
}

// TestCollapseSingleStatementFunction pins the rule: a function with a
// single-statement body collapses onto one line — params and all — when the
// whole line fits in 80 cols; extra statements, body comments, or an over-long
// line keep it expanded.
func TestCollapseSingleStatementFunction(t *testing.T) {
	cases := []struct{ name, input, want string }{
		{"no-param single return collapses",
			"fn Foo() Solid {\n    return Cube(s: 10 mm)\n}\n",
			"fn Foo() Solid { return Cube(s: 10 mm) }\n"},
		{"method with receiver collapses",
			"fn Solid.Twice() Solid {\n    return self + self\n}\n",
			"fn Solid.Twice() Solid { return self + self }\n"},
		{"single non-return statement collapses",
			"fn Foo() {\n    assert true, \"ok\"\n}\n",
			"fn Foo() { assert true, \"ok\" }\n"},
		{"params that fit collapse",
			"fn Add(a, b Number) Number {\n    return a + b\n}\n",
			"fn Add(a, b Number) Number { return a + b }\n"},
		{"params that overflow keep it expanded",
			"fn Combine(alpha Number, beta Number, gamma Number, delta Number) Number {\n    return alpha + beta + gamma + delta * epsilon * zeta\n}\n",
			"fn Combine(alpha, beta, gamma, delta Number) Number {\n    return alpha + beta + gamma + delta * epsilon * zeta\n}\n"},
		{"multiple statements keep it expanded",
			"fn Foo() Number {\n    var x = 1\n    return x\n}\n",
			"fn Foo() Number {\n    var x = 1\n    return x\n}\n"},
		{"body comment keeps it expanded",
			"fn Foo() Number {\n    // keep me\n    return 1\n}\n",
			"fn Foo() Number {\n    // keep me\n    return 1\n}\n"},
		{"over-long collapse stays expanded",
			"fn Compute() Length {\n    return alpha + beta + gamma + delta + epsilon + zeta + eta + theta + iota\n}\n",
			"fn Compute() Length {\n    return alpha + beta + gamma + delta + epsilon + zeta + eta + theta + iota\n}\n"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatString(tt.input); got != tt.want {
				t.Errorf("Format() = %q, want %q", got, tt.want)
			}
		})
	}
}
