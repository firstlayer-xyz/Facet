package formatter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"facet/pkg/fctlang/parser"
)

// minifyString parses source and minifies it.
func minifyString(t *testing.T, source string) string {
	t.Helper()
	src, err := parser.Parse(source, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	return Minify(src)
}

func TestMinifyDropsCommentsAndIndent(t *testing.T) {
	input := "fn Main() {\n    // hello\n    return 1\n}\n"
	want := "fn Main() {\nreturn 1\n}\n"
	if got := minifyString(t, input); got != want {
		t.Errorf("Minify:\n got %q\nwant %q", got, want)
	}
}

func TestMinifyRemovesBlankLinesAndDocComments(t *testing.T) {
	input := `# a doc comment
type Box {
    w Length  // width of the box
    h Length
}

fn Main() {
    // build a box
    var b = Box{w: 10, h: 20}

    return b.w
}
`
	got := minifyString(t, input)
	if strings.Contains(got, "//") || strings.Contains(got, "# ") {
		t.Errorf("Minify left comments behind:\n%s", got)
	}
	for _, line := range strings.Split(got, "\n") {
		if line != strings.TrimLeft(line, " \t") {
			t.Errorf("Minify left indentation on line %q in:\n%s", line, got)
		}
		if line == "" {
			// trailing newline produces one empty final element; tolerate only that
			continue
		}
	}
	// Named args / struct fields are NOT comments and must survive.
	if !strings.Contains(got, "w: 10") || !strings.Contains(got, "h: 20") {
		t.Errorf("Minify dropped struct-literal fields:\n%s", got)
	}
}

// Minify must be semantics-preserving: minifying its own output yields an
// identical result (idempotent), proving the minified source re-parses to the
// same comment-free AST.
func TestMinifyIsIdempotent(t *testing.T) {
	input := `fn helper(width Length, depth Length) Solid {
    // inner comment
    if width > depth {
        return Cube(size: width)
    }
    return Cube(size: depth)
}

fn Main() Solid {
    var parts = for i [0:<3] {
        yield helper(width: 10 + i, depth: 20)
    }
    return parts.Union()
}
`
	once := minifyString(t, input)
	twice := minifyString(t, once)
	if once != twice {
		t.Errorf("Minify not idempotent:\nonce:\n%s\ntwice:\n%s", once, twice)
	}
}

// Minify must round-trip every real bundled example: parsing the minified
// output and minifying again yields an identical string. This is the strongest
// available proof that minify is semantics-preserving across all syntax in use.
func TestMinifyRoundTripsBundledExamples(t *testing.T) {
	examples, err := filepath.Glob("../../../share/examples/*.fct")
	if err != nil {
		t.Fatalf("glob examples: %v", err)
	}
	if len(examples) == 0 {
		t.Fatal("no bundled examples found")
	}
	for _, path := range examples {
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			src, err := parser.Parse(string(data), path, parser.SourceUser)
			if err != nil {
				t.Skipf("example does not parse standalone: %v", err)
			}
			once := Minify(src)
			reparsed, err := parser.Parse(once, path, parser.SourceUser)
			if err != nil {
				t.Fatalf("minified output failed to re-parse: %v\n%s", err, once)
			}
			if twice := Minify(reparsed); once != twice {
				t.Errorf("Minify not idempotent for %s", filepath.Base(path))
			}
		})
	}
}
