package evaluator

import (
	"facet/app/pkg/fctlang/parser"
	"errors"
	"fmt"
)

// errAt creates a SourceError at the given source location using e.file.
func (e *evaluator) errAt(pos parser.Pos, format string, a ...any) *parser.SourceError {
	se := &parser.SourceError{File: e.file, Line: pos.Line, Col: pos.Col, Message: fmt.Sprintf(format, a...)}
	if e.file != "" && e.libSources != nil {
		se.Source = e.libSources[e.file]
	}
	return se
}

// wrapErr attaches source location to an error if it doesn't already have one.
// If err is already a *SourceError, returns it as-is (prevents double-wrapping).
func (e *evaluator) wrapErr(pos parser.Pos, err error) error {
	if err == nil {
		return nil
	}
	var se *parser.SourceError
	if errors.As(err, &se) {
		return err
	}
	se = &parser.SourceError{File: e.file, Line: pos.Line, Col: pos.Col, Message: err.Error()}
	if e.file != "" && e.libSources != nil {
		se.Source = e.libSources[e.file]
	}
	return se
}
