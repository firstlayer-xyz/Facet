package doc

import (
	"facet/app/pkg/fctlang/parser"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"facet/app/stdlib"
)

// stdlibDocSource returns a parsed stdlib Source.
// Used by BuildDocIndex when no pre-parsed stdlib is provided.
func stdlibDocSource() *parser.Source {
	src, err := parser.Parse(stdlib.StdlibSource, "", parser.SourceUser)
	if err != nil {
		return &parser.Source{}
	}
	return src
}

// DocEntry is a JSON-serializable documentation entry for a function, method, type, or keyword.
type DocEntry struct {
	Name      string `json:"name"`      // "Cube" or "Solid.Translate"
	Signature string `json:"signature"` // "Solid Cube(Length x, Length y, Length z)"
	Doc       string `json:"doc"`       // "Creates an axis-aligned box..."
	Kind      string `json:"kind"`      // "function", "method", "type", "keyword"
	Library   string `json:"library"`   // "facet/gears" or "" for stdlib/builtins
}

// formatStructSignature reconstructs a human-readable signature from a StructDecl AST node.
func formatStructSignature(sd *parser.StructDecl) string {
	var b strings.Builder
	b.WriteString("type ")
	b.WriteString(sd.Name)
	b.WriteString(" { ")
	for i, f := range sd.Fields {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(f.Name)
		b.WriteByte(' ')
		b.WriteString(f.Type)
		b.WriteByte(';')
	}
	b.WriteString(" }")
	return b.String()
}

// FormatSignature reconstructs a human-readable signature from a Function AST node.
// Uses the fn keyword with trailing return type and groups consecutive same-type params.
func FormatSignature(fn *parser.Function) string {
	var b strings.Builder
	b.WriteString("fn ")
	if fn.ReceiverType != "" {
		b.WriteString(fn.ReceiverType)
		b.WriteByte('.')
	}
	b.WriteString(fn.Name)
	b.WriteByte('(')
	// Group consecutive same-type params: "x, y Length, z Angle"
	type paramGroup struct {
		names []string
		typ   string
	}
	var groups []paramGroup
	for _, p := range fn.Params {
		if len(groups) == 0 || groups[len(groups)-1].typ != p.Type {
			groups = append(groups, paramGroup{typ: p.Type})
		}
		groups[len(groups)-1].names = append(groups[len(groups)-1].names, p.Name)
	}
	first := true
	for _, g := range groups {
		for _, name := range g.names {
			if !first {
				b.WriteString(", ")
			}
			first = false
			b.WriteString(name)
		}
		if g.typ != "" {
			b.WriteByte(' ')
			b.WriteString(g.typ)
		}
	}
	b.WriteByte(')')
	if fn.ReturnType != "" {
		b.WriteByte(' ')
		b.WriteString(fn.ReturnType)
	}
	return b.String()
}

// BuildLibDocEntries extracts doc entries from .fct files in a filesystem library directory.
// It walks libDir looking for <name>/<name>.fct library files and parses their doc comments.
func BuildLibDocEntries(libDir string) []DocEntry {
	var entries []DocEntry
	_ = filepath.WalkDir(libDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		base := strings.TrimSuffix(filepath.Base(path), ".fct")
		if base == filepath.Base(path) || base != filepath.Base(filepath.Dir(path)) {
			return nil
		}
		// Skip std — handled separately
		if strings.Contains(path, "facet/std/") || strings.Contains(path, "facet"+string(filepath.Separator)+"std"+string(filepath.Separator)) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		src := string(data)
		libProg, err := parser.Parse(src, "", parser.SourceUser)
		if err != nil {
			return nil
		}
		// Derive namespace from path relative to libDir
		dir := filepath.Dir(path)
		ns, _ := filepath.Rel(libDir, dir)
		for _, fn := range libProg.Functions {
			if strings.HasPrefix(fn.Name, "_") {
				continue
			}
			kind := "function"
			name := fn.Name
			if fn.ReceiverType != "" {
				kind = "method"
				name = fmt.Sprintf("%s.%s", fn.ReceiverType, fn.Name)
			}
			entries = append(entries, DocEntry{
				Name:      name,
				Signature: FormatSignature(fn),
				Doc:       parser.DocComment(fn.Comments),
				Kind:      kind,
				Library:   ns,
			})
		}
		for _, sd := range libProg.StructDecls {
			entries = append(entries, DocEntry{
				Name:      sd.Name,
				Signature: formatStructSignature(sd),
				Doc:       parser.DocComment(sd.Comments),
				Kind:      "type",
				Library:   ns,
			})
			for _, f := range sd.Fields {
				entries = append(entries, DocEntry{
					Name:      fmt.Sprintf("%s.%s", sd.Name, f.Name),
					Signature: f.Type,
					Doc:       fmt.Sprintf("Field of %s (type %s)", sd.Name, f.Type),
					Kind:      "field",
					Library:   ns,
				})
			}
		}
		return nil
	})
	return entries
}

// BuildDocIndex builds a complete documentation index from stdlib, user source, and built-in types/keywords.
// stdlibSrc may be nil if stdlib is not available.
func BuildDocIndex(source string, stdlibSrc *parser.Source) []DocEntry {
	var entries []DocEntry

	// 1. Stdlib entries
	if stdlibSrc == nil {
		stdlibSrc = stdlibDocSource()
	}
	for _, fn := range stdlibSrc.Functions {
		// Skip _-prefixed internal builtins that might sneak through
		if strings.HasPrefix(fn.Name, "_") {
			continue
		}
		kind := "function"
		name := fn.Name
		if fn.ReceiverType != "" {
			kind = "method"
			name = fmt.Sprintf("%s.%s", fn.ReceiverType, fn.Name)
		}
		entries = append(entries, DocEntry{
			Name:      name,
			Signature: FormatSignature(fn),
			Doc:       parser.DocComment(fn.Comments),
			Kind:      kind,
		})
	}

	// 1b. Stdlib struct fields
	for _, sd := range stdlibSrc.StructDecls {
		for _, f := range sd.Fields {
			entries = append(entries, DocEntry{
				Name:      fmt.Sprintf("%s.%s", sd.Name, f.Name),
				Signature: f.Type,
				Doc:       fmt.Sprintf("Field of %s (type %s)", sd.Name, f.Type),
				Kind:      "field",
			})
		}
	}

	// 2. Built-in library entries (facet/gears, etc. — excluding facet/std which is already covered)
	_ = fs.WalkDir(stdlib.Libraries, "libraries", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		base := strings.TrimSuffix(filepath.Base(path), ".fct")
		if base == filepath.Base(path) || base != filepath.Base(filepath.Dir(path)) {
			return nil
		}
		// Skip std — already indexed above
		if strings.Contains(path, "facet/std/") {
			return nil
		}
		data, err := stdlib.Libraries.ReadFile(path)
		if err != nil {
			return nil
		}
		src := string(data)
		libProg, err := parser.Parse(src, "", parser.SourceUser)
		if err != nil {
			return nil
		}
		// Derive library namespace from path: "libraries/facet/gears" → "facet/gears"
		dir := filepath.Dir(path)
		ns, _ := filepath.Rel("libraries", dir)
		for _, fn := range libProg.Functions {
			if strings.HasPrefix(fn.Name, "_") {
				continue
			}
			kind := "function"
			name := fn.Name
			if fn.ReceiverType != "" {
				kind = "method"
				name = fmt.Sprintf("%s.%s", fn.ReceiverType, fn.Name)
			}
			entries = append(entries, DocEntry{
				Name:      name,
				Signature: FormatSignature(fn),
				Doc:       parser.DocComment(fn.Comments),
				Kind:      kind,
				Library:   ns,
			})
		}
		for _, sd := range libProg.StructDecls {
			entries = append(entries, DocEntry{
				Name:      sd.Name,
				Signature: formatStructSignature(sd),
				Doc:       parser.DocComment(sd.Comments),
				Kind:      "type",
				Library:   ns,
			})
			for _, f := range sd.Fields {
				entries = append(entries, DocEntry{
					Name:      fmt.Sprintf("%s.%s", sd.Name, f.Name),
					Signature: f.Type,
					Doc:       fmt.Sprintf("Field of %s (type %s)", sd.Name, f.Type),
					Kind:      "field",
					Library:   ns,
				})
			}
		}
		return nil
	})

	// 3. User-defined functions/methods and structs
	prog, err := parser.Parse(source, "", parser.SourceUser)
	if err == nil && prog != nil {
		for _, fn := range prog.Functions {
			if strings.HasPrefix(fn.Name, "_") {
				continue
			}
			kind := "function"
			name := fn.Name
			if fn.ReceiverType != "" {
				kind = "method"
				name = fmt.Sprintf("%s.%s", fn.ReceiverType, fn.Name)
			}
			entries = append(entries, DocEntry{
				Name:      name,
				Signature: FormatSignature(fn),
				Doc:       parser.DocComment(fn.Comments),
				Kind:      kind,
			})
		}
		for _, sd := range prog.StructDecls {
			entries = append(entries, DocEntry{
				Name:      sd.Name,
				Signature: formatStructSignature(sd),
				Doc:       parser.DocComment(sd.Comments),
				Kind:      "type",
			})
			for _, f := range sd.Fields {
				entries = append(entries, DocEntry{
					Name:      fmt.Sprintf("%s.%s", sd.Name, f.Name),
					Signature: f.Type,
					Doc:       fmt.Sprintf("Field of %s (type %s)", sd.Name, f.Type),
					Kind:      "field",
				})
			}
		}
	}

	// 4. Built-in types
	builtinTypes := []DocEntry{
		{Name: "Solid", Doc: "3D manifold geometry. Created by constructors like Cube, Sphere, Cylinder, or by extruding/revolving a Sketch.", Kind: "type"},
		{Name: "Sketch", Doc: "2D cross-section. Created by Square, Circle, Polygon, or by slicing/projecting a Solid.", Kind: "type"},
		{Name: "Length", Doc: "Dimensional value with a unit, stored internally as millimeters. Literals: 10 mm, 2.5 cm, 1 ft.", Kind: "type"},
		{Name: "Angle", Doc: "Angular value, stored internally as degrees. Literals: 45 deg, 3.14 rad.", Kind: "type"},
		{Name: "Number", Doc: "Plain numeric value without units. Literals: 10, 3.14, 1/2.", Kind: "type"},
		{Name: "Vec2", Doc: "2D vector/coordinate created by Vec2(x, y).", Kind: "type"},
		{Name: "Vec3", Doc: "3D vector/coordinate created by Vec3(x, y, z).", Kind: "type"},
		{Name: "[]Type", Doc: "Typed array. Literals: [a, b, c]. Created by array literals or for-yield loops. All elements must be the same type.", Kind: "type"},
		{Name: "Bool", Doc: "Boolean value: true or false.", Kind: "type"},
		{Name: "String", Doc: "String value. Used for library paths, text, and pattern matching.", Kind: "type"},
	}
	entries = append(entries, builtinTypes...)

	// 5. Keywords
	keywords := []DocEntry{
		{Name: "fn", Doc: "Declares a function or method: fn Name(params) ReturnType { ... }", Kind: "keyword"},
		{Name: "type", Doc: "Declares a named struct type: type Name { field Type ... }", Kind: "keyword"},
		{Name: "var", Doc: "Declares a local or global variable: var x = 10 mm;", Kind: "keyword"},
		{Name: "for", Doc: "Loop construct. for-yield collects values into an Array.", Kind: "keyword"},
		{Name: "fold", Doc: "Reduces an array with an accumulator: fold acc, elem array { acc + elem }.", Kind: "keyword"},
		{Name: "if", Doc: "Conditional expression: if cond { ... } else if cond { ... } else { ... }", Kind: "keyword"},
		{Name: "return", Doc: "Returns a value from the current function or block.", Kind: "keyword"},
		{Name: "yield", Doc: "Yields a value inside a for-yield loop body, collecting results into an Array.", Kind: "keyword"},
		{Name: "lib", Doc: "Loads a library: var T = lib \"facet/gears\"; then T.Func(...).", Kind: "keyword"},
		{Name: "assert", Doc: "Asserts a condition at runtime: assert cond; or assert cond, \"message\";", Kind: "keyword"},
		{Name: "self", Doc: "Implicit receiver inside a method body. Refers to the object the method was called on.", Kind: "keyword"},
		{Name: "const", Doc: "Declares an immutable binding: const x = 10 mm;", Kind: "keyword"},
		{Name: "where", Doc: "Attaches a constraint to a variable or parameter: var x = 10 where [0:100]; Constraints are re-validated on reassignment.", Kind: "keyword"},
	}
	entries = append(entries, keywords...)

	return entries
}
