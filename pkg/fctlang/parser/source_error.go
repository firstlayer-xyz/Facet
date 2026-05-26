package parser

import "fmt"

// SourceError is a source-level error with location.
// Used by the parser, type checker, and evaluator.
type SourceError struct {
	File    string `json:"file"`              // "" = main file, "facet/std" or lib path for libraries
	Line    int    `json:"line"`
	Col     int    `json:"col"`
	EndCol  int    `json:"endCol"`            // 0 = highlight to end of line (used by checker)
	Message string `json:"message"`
	Source  string `json:"source,omitempty"`  // library source text (for error navigation)
}

func (e *SourceError) Error() string {
	if e.File != "" {
		return fmt.Sprintf("[%s] %d:%d: %s", e.File, e.Line, e.Col, e.Message)
	}
	if e.Line > 0 {
		return fmt.Sprintf("%d:%d: %s", e.Line, e.Col, e.Message)
	}
	return e.Message
}
