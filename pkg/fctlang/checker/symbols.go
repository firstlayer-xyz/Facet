package checker

import (
	"strings"

	"facet/pkg/fctlang/doc"
	"facet/pkg/fctlang/loader"
	"facet/pkg/fctlang/parser"
)

// Symbol is one editor-visible identifier — a function, method, type,
// field, or keyword. Symbols are the editor's symbol table: the
// authoritative answer to "what names exist and where do they live."
// Completion, signature-help, and hover all filter this list by
// (library, receiver) to find what is in scope at the cursor.
//
// Library is the canonical namespace from loader.LibPathToNamespace.
// "" means user/stdlib/builtin/keyword (always in scope; no qualifier
// required). It is the SAME string the checker stamps on a lib-alias
// variable as "Library:<ns>" in varTypes, so qualified completion's
// `s.Library === ns` filter agrees with the checker by construction.
//
// Receiver is the receiver type for methods and fields; empty for
// top-level functions and types. This is what completion's dot path
// filters on after `expr.<dot>` resolves to a concrete type.
type Symbol struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"` // "function" | "method" | "type" | "field" | "keyword"
	Signature string `json:"signature,omitempty"`
	Doc       string `json:"doc,omitempty"`
	Library   string `json:"library,omitempty"`
	Receiver  string `json:"receiver,omitempty"`
}

// BuildSymbols extracts the editor symbol table from a loaded program
// for one active source. entryKey is the user-source key that owns the
// editor's current cursor — symbols from OTHER user sources (open tabs
// that aren't imported by the entry) are excluded so completion doesn't
// suggest names that aren't actually in scope at the call site.
//
// Every library tag comes from loader.LibPathToNamespace applied to the
// raw import path the loader resolved — the one source of truth shared
// with the checker's varTypes. Built-in types and keywords are appended
// so hover and completion find them without a separate code path.
func BuildSymbols(prog loader.Program, entryKey string) []Symbol {
	var out []Symbol
	seen := map[string]bool{}
	add := func(s Symbol) {
		// Receiver-qualified key prevents collisions when a method and
		// a free function share the same name in the same namespace.
		// Overloads (same name + receiver + library) collapse to one
		// entry; signature-help collects all matching symbols at lookup
		// time, so a single dedup key is fine here.
		key := s.Library + "|" + s.Receiver + "|" + s.Name + "|" + s.Kind + "|" + s.Signature
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, s)
	}

	// Stdlib symbols — Library="", in scope unqualified.
	if std := prog.Std(); std != nil {
		for _, sym := range extractSourceSymbols(std, "") {
			add(sym)
		}
	}

	// Library symbols — one tag per resolved import. Iterating
	// prog.Imports (raw → resolved disk path) gives every library the
	// loader actually pulled in for this eval, tagged with its
	// canonical namespace. A library imported by two raw paths (e.g.
	// `lib@main` and `lib@v1`) resolves to the same disk source, so
	// we dedup by resolved path.
	libSeen := map[string]bool{}
	for rawPath, resolvedPath := range prog.Imports {
		if libSeen[resolvedPath] {
			continue
		}
		libSeen[resolvedPath] = true
		src := prog.Sources[resolvedPath]
		if src == nil {
			continue
		}
		ns := loader.LibPathToNamespace(rawPath)
		for _, sym := range extractSourceSymbols(src, ns) {
			add(sym)
		}
	}

	// User source symbols — Library="", in scope unqualified. Only the
	// entry source contributes: other open tabs are separate files
	// whose declarations aren't reachable from the entry unless one
	// imports the other via `lib`, and lib-imported ones are already
	// covered by the prog.Imports walk above.
	if entryKey != "" && entryKey != loader.StdlibPath && !prog.IsLibrarySource(entryKey) {
		if src := prog.Sources[entryKey]; src != nil {
			for _, sym := range extractSourceSymbols(src, "") {
				add(sym)
			}
		}
	}

	// Built-in types + keywords — synthetic, not in any AST.
	out = append(out, builtinSymbols()...)
	return out
}

// extractSourceSymbols walks one source's top-level declarations and
// emits one Symbol per function, method, type, and field. The library
// tag is applied verbatim to every emitted symbol.
func extractSourceSymbols(src *parser.Source, library string) []Symbol {
	var out []Symbol
	for _, fn := range src.Functions() {
		if fn.IsOperator || strings.HasPrefix(fn.Name, "_") {
			continue
		}
		sym := Symbol{
			Name:      fn.Name,
			Signature: doc.FormatSignature(fn),
			Doc:       parser.DocComment(fn.Comments),
			Library:   library,
		}
		if fn.ReceiverType != "" {
			sym.Kind = "method"
			sym.Receiver = fn.ReceiverType
		} else {
			sym.Kind = "function"
		}
		out = append(out, sym)
	}
	for _, sd := range src.StructDecls() {
		out = append(out, Symbol{
			Name:      sd.Name,
			Kind:      "type",
			Signature: doc.FormatStructSignature(sd),
			Doc:       parser.DocComment(sd.Comments),
			Library:   library,
		})
		for _, f := range sd.Fields {
			out = append(out, Symbol{
				Name:      f.Name,
				Kind:      "field",
				Signature: f.Type,
				Library:   library,
				Receiver:  sd.Name,
			})
		}
	}
	return out
}

// builtinSymbols are the primitives and keywords that are not declared
// in any source file but are part of the language. Kept inline rather
// than read from a config file because the set changes only when the
// language grammar does.
func builtinSymbols() []Symbol {
	types := []struct{ name, docStr string }{
		{"Solid", "3D manifold geometry. Created by constructors like Cube, Sphere, Cylinder, or by extruding/revolving a Sketch."},
		{"Sketch", "2D cross-section. Created by Square, Circle, Polygon, or by slicing/projecting a Solid."},
		{"Length", "Dimensional value with a unit, stored internally as millimeters. Literals: 10 mm, 2.5 cm, 1 ft."},
		{"Angle", "Angular value, stored internally as degrees. Literals: 45 deg, 3.14 rad."},
		{"Number", "Plain numeric value without units. Literals: 10, 3.14, 1/2."},
		{"Vec2", "2D vector/coordinate created by Vec2(x, y)."},
		{"Vec3", "3D vector/coordinate created by Vec3(x, y, z)."},
		{"[]Type", "Typed array. Literals: [a, b, c]. Created by array literals or for-yield loops. All elements must be the same type."},
		{"Bool", "Boolean value: true or false."},
		{"String", "String value. Used for library paths, text, and pattern matching."},
	}
	keywords := []struct{ name, docStr string }{
		{"fn", "Declares a function or method: fn Name(params) ReturnType { ... }"},
		{"type", "Declares a named struct type: type Name { field Type ... }"},
		{"var", "Declares a local or global variable: var x = 10 mm;"},
		{"for", "Loop construct. for-yield collects values into an Array."},
		{"fold", "Reduces an array with an accumulator: fold acc, elem array { acc + elem }."},
		{"if", "Conditional expression: if cond { ... } else if cond { ... } else { ... }"},
		{"return", "Returns a value from the current function or block."},
		{"yield", "Yields a value inside a for-yield loop body, collecting results into an Array."},
		{"lib", "Loads a library: var T = lib \"facet/gears\"; then T.Func(...)."},
		{"assert", "Asserts a condition at runtime: assert cond; or assert cond, \"message\";"},
		{"self", "Implicit receiver inside a method body. Refers to the object the method was called on."},
		{"const", "Declares an immutable binding: const x = 10 mm;"},
		{"where", "Attaches a constraint to a variable or parameter: var x = 10 where [0:100]; Constraints are re-validated on reassignment."},
	}
	out := make([]Symbol, 0, len(types)+len(keywords))
	for _, t := range types {
		out = append(out, Symbol{Name: t.name, Kind: "type", Doc: t.docStr})
	}
	for _, k := range keywords {
		out = append(out, Symbol{Name: k.name, Kind: "keyword", Doc: k.docStr})
	}
	return out
}
