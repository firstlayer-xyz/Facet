package parser

import (
	"strings"
	"testing"
)

// A lambda body is a fresh function scope: yield inside it is a parse error
// even when the lambda literal sits inside a for-yield body. Without the
// reset, the yield parsed and then escaped at runtime into whatever
// comprehension happened to be active when the lambda was called.
func TestParseYieldInsideLambdaRejected(t *testing.T) {
	src := `fn Main() Solid {
    var fs = for i [1, 2] {
        yield fn() Number {
            if true {
                yield 9
            }
            return i
        }
    }
    return Cube(s: 10 mm)
}
`
	_, err := Parse(src, "main.fct", SourceUser)
	if err == nil {
		t.Fatal("expected parse error for yield inside a lambda body")
	}
	if !strings.Contains(err.Error(), "yield") {
		t.Fatalf("expected the error to mention yield, got: %v", err)
	}
}
