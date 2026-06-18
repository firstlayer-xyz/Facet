package formatter

import "testing"

// A leading comment that immediately follows the previous statement (no blank
// line between them) must not gain a spurious blank line. The blank-line gap is
// measured from the comment's line, not the statement it precedes — otherwise
// the comment's source line reads as an empty gap and a blank is inserted.
func TestFormatNoSpuriousBlankBeforeLeadingComment(t *testing.T) {
	src := "fn Main() {\n    var a = 1\n    # comment\n    var b = 2\n    return b\n}\n"
	if got := formatString(src); got != src {
		t.Errorf("formatter inserted a spurious blank before the leading comment:\ninput:\n%q\ngot:\n%q", src, got)
	}
}

// An intentional blank line before a leading comment is still preserved.
func TestFormatPreservesBlankBeforeLeadingComment(t *testing.T) {
	src := "fn Main() {\n    var a = 1\n\n    # comment\n    var b = 2\n    return b\n}\n"
	if got := formatString(src); got != src {
		t.Errorf("formatter changed an intentional blank-before-comment:\ninput:\n%q\ngot:\n%q", src, got)
	}
}
