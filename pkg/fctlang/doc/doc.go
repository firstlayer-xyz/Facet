package doc

import (
	"context"
	"facet/pkg/fctlang/loader"
	"facet/pkg/fctlang/parser"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"facet/share/stdlib"
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

// UserLibrary is the library tag assigned to doc entries extracted from the user's own source file.
const UserLibrary = "__user__"

// DocEntry is a JSON-serializable documentation entry for a function, method, type, or keyword.
type DocEntry struct {
	Name      string `json:"name"`              // "Cube" or "Solid.Move"
	Signature string `json:"signature"`          // "Solid Cube(Length x, Length y, Length z)"
	Doc       string `json:"doc"`               // "Creates an axis-aligned box..."
	Kind      string `json:"kind"`              // "function", "method", "type", "keyword"
	Library   string `json:"library"`           // "facet/gears" or "" for stdlib/builtins
	Section   string `json:"section,omitempty"` // source-level section, e.g. "3D Constructors"
}

// FormatStructSignature reconstructs a human-readable signature from a StructDecl AST node.
func FormatStructSignature(sd *parser.StructDecl) string {
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
// Uses the fn keyword with trailing return type. Each parameter is emitted with
// its name and type, plus any default literal, so signature-help tools that split
// on commas produce one labeled parameter per position.
func FormatSignature(fn *parser.Function) string {
	var b strings.Builder
	b.WriteString("fn ")
	if fn.ReceiverType != "" {
		b.WriteString(fn.ReceiverType)
		b.WriteByte('.')
	}
	b.WriteString(fn.Name)
	b.WriteByte('(')
	for i, p := range fn.Params {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(p.Name)
		if p.Type != "" {
			b.WriteByte(' ')
			b.WriteString(p.Type)
		}
		if p.Default != nil {
			if lit, ok := formatDefaultExpr(p.Default); ok {
				b.WriteString(" = ")
				b.WriteString(lit)
			}
		}
	}
	b.WriteByte(')')
	if fn.ReturnType != "" {
		b.WriteByte(' ')
		b.WriteString(fn.ReturnType)
	}
	return b.String()
}

// formatDefaultExpr renders a default-value expression for display in a
// function signature. Handles the literal forms actually used as stdlib/library
// defaults (numbers, units, strings, bools, empty struct literals). Returns
// ok=false for expressions that don't have a short display form — the caller
// omits the "= ..." clause rather than spilling a full expression into the
// signature.
func formatDefaultExpr(e parser.Expr) (string, bool) {
	switch v := e.(type) {
	case *parser.NumberLit:
		return strconv.FormatFloat(v.Value, 'f', -1, 64), true
	case *parser.UnitExpr:
		inner, ok := formatDefaultExpr(v.Expr)
		if !ok {
			return "", false
		}
		if v.Unit == "" {
			return inner, true
		}
		return inner + " " + v.Unit, true
	case *parser.StringLit:
		return strconv.Quote(v.Value), true
	case *parser.BoolLit:
		if v.Value {
			return "true", true
		}
		return "false", true
	case *parser.StructLitExpr:
		if len(v.Fields) == 0 {
			return v.TypeName + "{}", true
		}
		return "", false
	}
	return "", false
}

// extractDocEntries extracts doc entries (functions, methods, types, fields) from
// a parsed Source. The library string is set on each entry (empty for stdlib/user code).
// Declarations are processed in source order so that section-divider comments in the
// source propagate correctly to all entries within each section.
func extractDocEntries(src *parser.Source, library string) []DocEntry {
	var entries []DocEntry
	currentSection := ""

	for _, decl := range src.Declarations {
		// Advance the section tracker whenever a declaration opens a new section.
		if sec := parser.SectionName(decl.DeclComments()); sec != "" {
			currentSection = sec
		}

		switch d := decl.(type) {
		case *parser.Function:
			if strings.HasPrefix(d.Name, "_") {
				continue
			}
			kind := "function"
			name := d.Name
			if d.ReceiverType != "" {
				kind = "method"
				name = fmt.Sprintf("%s.%s", d.ReceiverType, d.Name)
			}
			entries = append(entries, DocEntry{
				Name:      name,
				Signature: FormatSignature(d),
				Doc:       parser.DocComment(d.Comments),
				Kind:      kind,
				Library:   library,
				Section:   currentSection,
			})
		case *parser.StructDecl:
			entries = append(entries, DocEntry{
				Name:      d.Name,
				Signature: FormatStructSignature(d),
				Doc:       parser.DocComment(d.Comments),
				Kind:      "type",
				Library:   library,
				Section:   currentSection,
			})
			for _, f := range d.Fields {
				entries = append(entries, DocEntry{
					Name:      fmt.Sprintf("%s.%s", d.Name, f.Name),
					Signature: f.Type,
					Doc:       fmt.Sprintf("Field of %s (type %s)", d.Name, f.Type),
					Kind:      "field",
					Library:   library,
					Section:   currentSection,
				})
			}
		}
	}
	return entries
}

// BuildLibDocEntries extracts doc entries from .fct files in a filesystem
// library directory (user-local libs). It walks libDir looking for
// <name>/<name>.fct files and parses their doc comments. For virtualized
// remote libs inside the git cache, use BuildCachedLibDocEntries instead.
func BuildLibDocEntries(libDir string) []DocEntry {
	var entries []DocEntry
	_ = filepath.WalkDir(libDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		base := strings.TrimSuffix(filepath.Base(p), ".fct")
		if base == filepath.Base(p) || base != filepath.Base(filepath.Dir(p)) {
			return nil
		}
		// Skip std — handled separately
		if strings.Contains(p, "facet/std/") || strings.Contains(p, "facet"+string(filepath.Separator)+"std"+string(filepath.Separator)) {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		libProg, err := parser.Parse(string(data), "", parser.SourceUser)
		if err != nil {
			return nil
		}
		dir := filepath.Dir(p)
		ns, _ := filepath.Rel(libDir, dir)
		// Lib namespaces are URL-style (slash-separated) regardless of
		// host OS — filepath.Rel returns backslashes on Windows, so
		// normalise.
		ns = filepath.ToSlash(ns)
		entries = append(entries, extractDocEntries(libProg, ns)...)
		return nil
	})
	return entries
}

// BuildCachedLibDocEntries extracts doc entries from every cached repo's
// default branch (origin/HEAD). One namespace per repo — the settings UI
// doesn't expose refs, and indexing every ref would just flood the doc
// prompt with near-duplicates. A .fct file that pins an older ref still
// works; it just won't show completions for signatures that have diverged.
func BuildCachedLibDocEntries(cacheDir string) []DocEntry {
	var entries []DocEntry
	ctx := context.Background()

	hosts, _ := os.ReadDir(cacheDir)
	for _, h := range hosts {
		if !h.IsDir() || strings.HasPrefix(h.Name(), ".") {
			continue
		}
		users, _ := os.ReadDir(filepath.Join(cacheDir, h.Name()))
		for _, u := range users {
			if !u.IsDir() {
				continue
			}
			repos, _ := os.ReadDir(filepath.Join(cacheDir, h.Name(), u.Name()))
			for _, r := range repos {
				if !r.IsDir() {
					continue
				}
				entries = append(entries, buildCachedRepoDocEntries(ctx, cacheDir, h.Name(), u.Name(), r.Name())...)
			}
		}
	}
	return entries
}

// buildCachedRepoDocEntries opens one repo's bare clone at origin/HEAD and
// emits doc entries for every <name>/<name>.fct file, namespaced as
// "<host>/<user>/<repo>[/<subpath>]".
func buildCachedRepoDocEntries(ctx context.Context, cacheDir, host, user, repo string) []DocEntry {
	var entries []DocEntry
	lp := &loader.LibPath{Host: host, User: user, Repo: repo, Raw: fmt.Sprintf("%s/%s/%s", host, user, repo)}
	tree, err := loader.OpenRepoHeadTree(ctx, cacheDir, lp)
	if err != nil {
		return nil
	}
	_ = tree.Walk(func(subPath string, r io.Reader) error {
		base := strings.TrimSuffix(path.Base(subPath), ".fct")
		if base == path.Base(subPath) {
			return nil // not a .fct file
		}
		if base != path.Base(path.Dir(subPath)) {
			return nil // not <name>/<name>.fct
		}
		// Skip embedded facet/std (handled separately).
		if strings.HasPrefix(subPath, "facet/std/") {
			return nil
		}
		data, err := io.ReadAll(r)
		if err != nil {
			return nil
		}
		libProg, err := parser.Parse(string(data), "", parser.SourceUser)
		if err != nil {
			return nil
		}
		dir := path.Dir(subPath)
		ns := path.Join(host, user, repo, dir)
		entries = append(entries, extractDocEntries(libProg, ns)...)
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
	entries = append(entries, extractDocEntries(stdlibSrc, "")...)

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
		libProg, err := parser.Parse(string(data), "", parser.SourceUser)
		if err != nil {
			return nil
		}
		// Derive library namespace from path: "libraries/facet/gears" → "facet/gears"
		dir := filepath.Dir(path)
		ns, _ := filepath.Rel("libraries", dir)
		entries = append(entries, extractDocEntries(libProg, ns)...)
		return nil
	})

	// 3. User-defined functions/methods and structs
	prog, err := parser.Parse(source, "", parser.SourceUser)
	if err == nil && prog != nil {
		entries = append(entries, extractDocEntries(prog, UserLibrary)...)
	}

	// 4. Built-in types (primitives not declared in .fct source)
	builtinTypes := []DocEntry{
		{Name: "Solid", Doc: "3D manifold geometry. Created by constructors like Cube, Sphere, Cylinder, or by extruding/revolving a Sketch.", Kind: "type", Section: "Built-in Types"},
		{Name: "Sketch", Doc: "2D cross-section. Created by Square, Circle, Polygon, or by slicing/projecting a Solid.", Kind: "type", Section: "Built-in Types"},
		{Name: "Length", Doc: "Dimensional value with a unit, stored internally as millimeters. Literals: 10 mm, 2.5 cm, 1 ft.", Kind: "type", Section: "Built-in Types"},
		{Name: "Angle", Doc: "Angular value, stored internally as degrees. Literals: 45 deg, 3.14 rad.", Kind: "type", Section: "Built-in Types"},
		{Name: "Number", Doc: "Plain numeric value without units. Literals: 10, 3.14, 1/2.", Kind: "type", Section: "Built-in Types"},
		{Name: "Vec2", Doc: "2D vector/coordinate created by Vec2(x, y).", Kind: "type", Section: "Built-in Types"},
		{Name: "Vec3", Doc: "3D vector/coordinate created by Vec3(x, y, z).", Kind: "type", Section: "Built-in Types"},
		{Name: "[]Type", Doc: "Typed array. Literals: [a, b, c]. Created by array literals or for-yield loops. All elements must be the same type.", Kind: "type", Section: "Built-in Types"},
		{Name: "Bool", Doc: "Boolean value: true or false.", Kind: "type", Section: "Built-in Types"},
		{Name: "String", Doc: "String value. Used for library paths, text, and pattern matching.", Kind: "type", Section: "Built-in Types"},
	}
	entries = append(entries, builtinTypes...)

	// 5. Keywords
	keywords := []DocEntry{
		{Name: "fn", Doc: "Declares a function or method: fn Name(params) ReturnType { ... }", Kind: "keyword", Section: "Keywords"},
		{Name: "type", Doc: "Declares a named struct type: type Name { field Type ... }", Kind: "keyword", Section: "Keywords"},
		{Name: "var", Doc: "Declares a local or global variable: var x = 10 mm;", Kind: "keyword", Section: "Keywords"},
		{Name: "for", Doc: "Loop construct. for-yield collects values into an Array.", Kind: "keyword", Section: "Keywords"},
		{Name: "fold", Doc: "Reduces an array with an accumulator: fold acc, elem array { acc + elem }.", Kind: "keyword", Section: "Keywords"},
		{Name: "if", Doc: "Conditional expression: if cond { ... } else if cond { ... } else { ... }", Kind: "keyword", Section: "Keywords"},
		{Name: "return", Doc: "Returns a value from the current function or block.", Kind: "keyword", Section: "Keywords"},
		{Name: "yield", Doc: "Yields a value inside a for-yield loop body, collecting results into an Array.", Kind: "keyword", Section: "Keywords"},
		{Name: "lib", Doc: "Loads a library: var T = lib \"facet/gears\"; then T.Func(...).", Kind: "keyword", Section: "Keywords"},
		{Name: "assert", Doc: "Asserts a condition at runtime: assert cond; or assert cond, \"message\";", Kind: "keyword", Section: "Keywords"},
		{Name: "self", Doc: "Implicit receiver inside a method body. Refers to the object the method was called on.", Kind: "keyword", Section: "Keywords"},
		{Name: "const", Doc: "Declares an immutable binding: const x = 10 mm;", Kind: "keyword", Section: "Keywords"},
		{Name: "where", Doc: "Attaches a constraint to a variable or parameter: var x = 10 where [0:100]; Constraints are re-validated on reassignment.", Kind: "keyword", Section: "Keywords"},
	}
	entries = append(entries, keywords...)

	return entries
}
