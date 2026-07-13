package parser

// markTrailing flags each comment as an end-of-line (trailing) comment.
func markTrailing(cs []Comment) {
	for i := range cs {
		cs[i].IsTrailing = true
	}
}

// elemComments accumulates the interior comments of an array literal, keyed by
// element index, so the formatter can keep each comment beside its element.
type elemComments struct {
	byIndex map[int][]Comment
}

func newElemComments() *elemComments {
	return &elemComments{byIndex: map[int][]Comment{}}
}

// attach records comments for element idx. Trailing (end-of-line) comments are
// marked as such before being stored.
func (e *elemComments) attach(idx int, cs []Comment, trailing bool) {
	if len(cs) == 0 {
		return
	}
	if trailing {
		markTrailing(cs)
	}
	e.byIndex[idx] = append(e.byIndex[idx], cs...)
}

// list materializes the parallel per-element side-list for n elements, or nil
// when no interior comments were recorded.
func (e *elemComments) list(n int) [][]Comment {
	if len(e.byIndex) == 0 {
		return nil
	}
	out := make([][]Comment, n)
	for i := 0; i < n; i++ {
		out[i] = e.byIndex[i]
	}
	return out
}
