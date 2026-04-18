package checker

import (
	"fmt"

	"facet/app/pkg/fctlang/parser"
)

// entryReturnTypes is the set of return types an entry-point function may
// declare. An entry point must ultimately produce a renderable geometry —
// a single solid, an array of solids, or a PolyMesh. Any other return type
// is rejected statically so the user gets the error at check time rather
// than after a full evaluation.
var entryReturnTypes = map[string]bool{
	"Solid":   true,
	"[]Solid": true,
	"PolyMesh": true,
}

// ValidateEntryPoint checks that the function named entryName in the given
// source declares (or infers) a return type acceptable for an entry point.
//
// entryName is treated as an ordinary function name — no identifier is
// special-cased here. Which function serves as the entry point is a runtime
// decision (the user picks it in the UI, or the caller passes it). This
// function only enforces the type constraint on whichever name is supplied.
//
// Returns nil when the return type is acceptable. Returns nil as well when
// the function cannot be found: that's the evaluator's "no such entry" case,
// reported separately with its own message.
func (r *Result) ValidateEntryPoint(srcKey, entryName string) *parser.SourceError {
	if entryName == "" {
		return nil
	}
	src := r.Prog.Sources[srcKey]
	if src == nil {
		return nil
	}
	var fn *parser.Function
	for _, f := range src.Functions() {
		if f.ReceiverType == "" && f.Name == entryName {
			fn = f
			break
		}
	}
	if fn == nil {
		return nil
	}

	ret := fn.ReturnType
	if ret == "" {
		ret = r.InferredReturnTypes[entryName]
	}
	if entryReturnTypes[ret] {
		return nil
	}

	msg := fmt.Sprintf("%s() must return Solid, []Solid, or PolyMesh", entryName)
	if ret != "" {
		msg = fmt.Sprintf("%s() must return Solid, []Solid, or PolyMesh, got %s", entryName, ret)
	}
	return &parser.SourceError{
		File:    srcKey,
		Line:    fn.Pos.Line,
		Col:     fn.Pos.Col,
		Message: msg,
	}
}
