package evaluator

import (
	"context"
	"strings"
	"testing"
)

func TestEvalStringComparison(t *testing.T) {
	tests := []struct {
		name string
		op   string
		want bool
	}{
		{"eq true", `"abc" == "abc"`, true},
		{"eq false", `"abc" == "def"`, false},
		{"ne true", `"abc" != "def"`, true},
		{"ne false", `"abc" != "abc"`, false},
		{"lt true", `"abc" < "def"`, true},
		{"lt false", `"def" < "abc"`, false},
		{"gt true", `"def" > "abc"`, true},
		{"gt false", `"abc" > "def"`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			branch := "10 mm"
			other := "5 mm"
			if !tt.want {
				branch, other = other, branch
			}
			src := `fn Main() { if ` + tt.op + ` { return Cube(s: Vec3{x: ` + branch + `, y: ` + branch + `, z: ` + branch + `}); } else { return Cube(s: Vec3{x: ` + other + `, y: ` + other + `, z: ` + other + `}); } }`
			prog := parseTestProg(t, src)
			mesh, err := evalMerged(context.Background(), prog, nil)
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}
			if mesh == nil {
				t.Fatal("expected non-nil mesh")
			}
		})
	}
}

func TestEvalStringConcatenation(t *testing.T) {
	src := `
fn Main() {
    var a = "hello";
    var b = " world";
    var c = a + b;
    if c == "hello world" {
        return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    } else {
        return Cube(s: Vec3{x: 5 mm, y: 5 mm, z: 5 mm});
    }
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
}

func TestEvalStringLength(t *testing.T) {
	src := `
fn Main() {
    var s = "hello";
    if Size(of: s) == 5 {
        return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    } else {
        return Cube(s: Vec3{x: 5 mm, y: 5 mm, z: 5 mm});
    }
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
}

func TestEvalStringSubStr(t *testing.T) {
	src := `
fn Main() {
    var s = "hello world";
    if s.SubStr(start: 0, length: 5) == "hello" {
        return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    } else {
        return Cube(s: Vec3{x: 5 mm, y: 5 mm, z: 5 mm});
    }
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
}

func TestEvalStringHasPrefixSuffix(t *testing.T) {
	src := `
fn Main() {
    var s = "hello world";
    if s.HasPrefix(prefix: "hello") && s.HasSuffix(suffix: "world") {
        return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    } else {
        return Cube(s: Vec3{x: 5 mm, y: 5 mm, z: 5 mm});
    }
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
}

func TestEvalStringMatch(t *testing.T) {
	// Match with captures: "m30x2".Match(`m(\d+)x([\d.]+)`) → ["m30x2", "30", "2"]
	src := "fn Main() {\n" +
		"    var s = \"m30x2\";\n" +
		"    var m = s.Match(pattern: `m(\\d+)x([\\d.]+)`);\n" +
		"    if Size(of: m) == 3 && m[0] == \"m30x2\" && m[1] == \"30\" && m[2] == \"2\" {\n" +
		"        return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});\n" +
		"    } else {\n" +
		"        return Cube(s: Vec3{x: 5 mm, y: 5 mm, z: 5 mm});\n" +
		"    }\n" +
		"}\n"
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
}

func TestEvalStringMatchNoMatch(t *testing.T) {
	// No match returns empty array
	src := `
fn Main() {
    var s = "hello";
    var m = s.Match(pattern: "xyz");
    if Size(of: m) == 0 {
        return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    } else {
        return Cube(s: Vec3{x: 5 mm, y: 5 mm, z: 5 mm});
    }
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
}

func TestEvalStringMatchTruthiness(t *testing.T) {
	// Non-empty match result == true, empty == false
	src := "fn Main() {\n" +
		"    var s = \"m30x2\";\n" +
		"    var matched = s.Match(pattern: `m\\d+`);\n" +
		"    var noMatch = s.Match(pattern: \"xyz\");\n" +
		"    if matched == true && noMatch == false {\n" +
		"        return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});\n" +
		"    } else {\n" +
		"        return Cube(s: Vec3{x: 5 mm, y: 5 mm, z: 5 mm});\n" +
		"    }\n" +
		"}\n"
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
}

func TestEvalStringMatchInvalidRegex(t *testing.T) {
	src := `
fn Main() {
    var s = "hello";
    var m = s.Match(pattern: "[invalid");
    return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
}
`
	prog := parseTestProg(t, src)
	_, err := evalMerged(context.Background(), prog, nil)
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
	if !strings.Contains(err.Error(), "invalid regex") {
		t.Fatalf("expected 'invalid regex' in error, got: %v", err)
	}
}

// stringPredicateTest runs `if cond { 10mm cube } else { 5mm cube }` and
// asserts the 10mm branch was taken — i.e. that the predicate written into
// the if condition holds. Used by the rest of the string-method tests in
// this file to keep each test small.
func stringPredicateTest(t *testing.T, condition string) {
	t.Helper()
	src := `
fn Main() {
    if ` + condition + ` {
        return Cube(s: Vec3{x: 10 mm, y: 10 mm, z: 10 mm});
    } else {
        return Cube(s: Vec3{x: 5 mm, y: 5 mm, z: 5 mm});
    }
}
`
	prog := parseTestProg(t, src)
	mesh, err := evalMerged(context.Background(), prog, nil)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if mesh == nil {
		t.Fatal("expected non-nil mesh")
	}
	assertMeshSize(t, mesh, 10, 10, 10, 0.1)
}

func TestEvalStringContains(t *testing.T) {
	stringPredicateTest(t, `"hello world".Contains(substr: "lo wor")`)
	stringPredicateTest(t, `!"hello world".Contains(substr: "xyz")`)
}

func TestEvalStringIndexOf(t *testing.T) {
	stringPredicateTest(t, `"hello world".IndexOf(substr: "world") == 6`)
	stringPredicateTest(t, `"hello world".IndexOf(substr: "missing") == -1`)
	stringPredicateTest(t, `"abc".IndexOf(substr: "a") == 0`)
}

func TestEvalStringReplace(t *testing.T) {
	stringPredicateTest(t, `"hello world".Replace(old: "world", new: "facet") == "hello facet"`)
	// All occurrences replaced, not just the first.
	stringPredicateTest(t, `"a-b-c".Replace(old: "-", new: "_") == "a_b_c"`)
	// Replacing a missing substring is a no-op.
	stringPredicateTest(t, `"abc".Replace(old: "x", new: "y") == "abc"`)
}

func TestEvalStringSplit(t *testing.T) {
	stringPredicateTest(t, `Size(of: "a,b,c".Split(delimiter: ",")) == 3`)
	stringPredicateTest(t, `"a,b,c".Split(delimiter: ",")[1] == "b"`)
	// Splitting on a missing delimiter yields a single-element slice.
	stringPredicateTest(t, `Size(of: "abc".Split(delimiter: ",")) == 1`)
	stringPredicateTest(t, `"abc".Split(delimiter: ",")[0] == "abc"`)
}

func TestEvalStringToUpper(t *testing.T) {
	stringPredicateTest(t, `"Hello".ToUpper() == "HELLO"`)
	stringPredicateTest(t, `"".ToUpper() == ""`)
	stringPredicateTest(t, `"123abc".ToUpper() == "123ABC"`)
}

func TestEvalStringToLower(t *testing.T) {
	stringPredicateTest(t, `"HELLO".ToLower() == "hello"`)
	stringPredicateTest(t, `"".ToLower() == ""`)
	stringPredicateTest(t, `"123ABC".ToLower() == "123abc"`)
}

func TestEvalStringTrim(t *testing.T) {
	stringPredicateTest(t, `"  hello  ".Trim() == "hello"`)
	stringPredicateTest(t, `"\thello\n".Trim() == "hello"`)
	// No leading/trailing whitespace is a no-op.
	stringPredicateTest(t, `"hello".Trim() == "hello"`)
}
