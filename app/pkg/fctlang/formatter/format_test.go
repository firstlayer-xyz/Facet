package formatter

import (
	"facet/app/pkg/fctlang/parser"
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
			want:  "fn Main() {\n    return 1\n}\n",
		},
		{
			name:  "re-indent over-indented body",
			input: "fn Main() {\n        return 1\n}\n",
			want:  "fn Main() {\n    return 1\n}\n",
		},
		{
			name:  "re-indent under-indented body",
			input: "fn Main() {\nreturn 1\n}\n",
			want:  "fn Main() {\n    return 1\n}\n",
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
			name: "for-yield body",
			input: `fn Main() {
var arr = for i [0:<3] {
yield i * 2
}
return arr
}
`,
			want: `fn Main() {
    var arr = for i [0:<3] {
        yield i * 2
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

fn Main() {
    return x + y
}
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

fn Main() {
    return x + y
}
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
fn Main() {
    return 1
}
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
fn Main() {
    return 1
}
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

fn Main() {
    return x + y
}
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
			want: `fn Solid.Scale(f Number) Solid {
    return self
}
`,
		},
		{
			name: "deeply nested",
			input: `fn Main() {
if true {
if false {
return 1
}
}
return 0
}
`,
			want: `fn Main() {
    if true {
        if false {
            return 1
        }
    }
    return 0
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
			want: `fn Main() {
    return Point{x: 1 mm, y: 2 mm}
}
`,
		},
		{
			name: "struct literal preserves explicit zeros",
			input: `fn Main() {
return Vec3{x: 1 mm, y: 0 mm, z: 0 mm}
}
`,
			want: `fn Main() {
    return Vec3{x: 1 mm, y: 0 mm, z: 0 mm}
}
`,
		},
		{
			name: "method chain splits on dot",
			input: `fn Main() {
return Cube(x: 10 mm, y: 10 mm, z: 10 mm).Translate(v: Vec3{x: 5 mm, y: 0 mm, z: 0 mm})
}
`,
			want: `fn Main() {
    return Cube(x: 10 mm, y: 10 mm, z: 10 mm)
        .Translate(v: Vec3{x: 5 mm, y: 0 mm, z: 0 mm})
}
`,
		},
		{
			name: "binary expression precedence",
			input: `fn Main() {
return 1 + 2 * 3
}
`,
			want: `fn Main() {
    return 1 + 2 * 3
}
`,
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
			want: `fn Main() {
    return []Number[1, 2, 3]
}
`,
		},
		{
			name: "typed array strips struct type",
			input: `fn Main() {
return []Vec3[Vec3{x: 1, y: 2, z: 3}, Vec3{x: 4, y: 5, z: 6}]
}
`,
			want: `fn Main() {
    return []Vec3[{x: 1, y: 2, z: 3}, {x: 4, y: 5, z: 6}]
}
`,
		},
		{
			name: "long array wraps to multi-line",
			input: `fn Main() {
return []Vec3[Vec3{x: 100 mm, y: 200 mm, z: 300 mm}, Vec3{x: 400 mm, y: 500 mm, z: 600 mm}]
}
`,
			want: `fn Main() {
    return []Vec3[
        {x: 100 mm, y: 200 mm, z: 300 mm},
        {x: 400 mm, y: 500 mm, z: 600 mm}
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
			name: "fold expression",
			input: `fn Main() {
return fold acc, elem [1, 2, 3] {
yield acc + elem
}
}
`,
			want: `fn Main() {
    return fold acc, elem [1, 2, 3] {
        yield acc + elem
    }
}
`,
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
return x
}
`,
			want: `fn Foo(
    x Length = 10,
) Length {
    return x
}
`,
		},
		{
			name: "fraction preserved",
			input: `fn Main() {
return 1/2 mm
}
`,
			want: `fn Main() {
    return 1/2 mm
}
`,
		},
		{
			name: "fraction without unit preserved",
			input: `fn Main() {
return 1/2
}
`,
			want: `fn Main() {
    return 1/2
}
`,
		},
		{
			name: "parenthesized division with unit",
			input: `fn Main() {
return (1 / 2) mm
}
`,
			want: `fn Main() {
    return (1 / 2) mm
}
`,
		},
		{
			name: "parenthesized division without unit",
			input: `fn Main() {
return (1 / 2)
}
`,
			want: `fn Main() {
    return 1 / 2
}
`,
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

func TestFormatSliceExpr(t *testing.T) {
	input := "fn Main() {\n    var x = [1, 2, 3]\n    var a = x[1:3]\n    var b = x[:2]\n    var c = x[1:]\n    return Cube(size: 10 mm)\n}\n"
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

func TestFormatStringEscapes(t *testing.T) {
	input := "fn Main() {\n    var x = \"hello\\nworld\"\n    return Cube(size: 10 mm)\n}\n"
	got := formatString(input)
	if !strings.Contains(got, `"hello\nworld"`) {
		t.Errorf("expected escaped string in output, got:\n%s", got)
	}
}

func TestFormatNegativeIndex(t *testing.T) {
	input := "fn Main() {\n    var x = [1, 2, 3]\n    var a = x[-1]\n    return Cube(size: a * 1 mm)\n}\n"
	got := formatString(input)
	if !strings.Contains(got, "x[-1]") {
		t.Errorf("expected x[-1] in output, got:\n%s", got)
	}
}

// annotate replaces newlines with ↵\n so diff lines are visible in test output.
func annotate(s string) string {
	return strings.ReplaceAll(s, "\n", "↵\n")
}
