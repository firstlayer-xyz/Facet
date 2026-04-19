package checker

import (
	"fmt"

	"facet/app/pkg/fctlang/parser"
)

// References maps a source position ("file:line:col") to the declaration it
// refers to. File is "" for the main file; set for library files. The map is
// built incrementally during type checking and used by the editor for
// go-to-definition and hover.
//
// Keys are opaque to consumers — always construct via refKey, never parse.
type References map[string]DeclLocation

// refKey formats a reference key from file + position. File is the empty
// string for the main source; library sources carry their resolved path.
func refKey(file string, pos parser.Pos) string {
	return fmt.Sprintf("%s:%d:%d", file, pos.Line, pos.Col)
}

// normalizeFile returns the DeclLocation.File value for a source key: the
// empty string for the main source, the key itself otherwise. Single-sources
// the "main file is empty string" convention shared by addRef,
// fileForFunction, and fileForStruct.
func (c *checker) normalizeFile(srcKey string) string {
	if srcKey == c.mainSrcKey {
		return ""
	}
	return srcKey
}

// addRef records a reference at `pos` in the current source pointing at `target`.
// Skips calls with a zero position — many synthesized subexpressions carry
// Line == 0 and have nothing to link to.
func (c *checker) addRef(pos parser.Pos, target DeclLocation) {
	if pos.Line == 0 {
		return
	}
	c.references[refKey(c.normalizeFile(c.currentSrcKey), pos)] = target
}

// fileForFunction returns the DeclLocation.File value for fn — empty for the
// main source, the source key (stdlib/library path) otherwise.
func (c *checker) fileForFunction(fn *parser.Function) string {
	return c.normalizeFile(c.funcSrcKey[fn])
}

// fileForStruct returns the DeclLocation.File value for sd — empty for the
// main source, the source key (stdlib/library path) otherwise.
func (c *checker) fileForStruct(sd *parser.StructDecl) string {
	return c.normalizeFile(c.structSrcKey[sd])
}

// globalDecl looks up a top-level declaration by name from the precomputed
// DeclResult built during initChecker. Used to resolve references to stdlib
// and other globals whose binding sites aren't tracked in typeEnv.declPos.
func (c *checker) globalDecl(name string) (DeclLocation, bool) {
	if c.declarations == nil {
		return DeclLocation{}, false
	}
	loc, ok := c.declarations.Decls[name]
	return loc, ok
}
