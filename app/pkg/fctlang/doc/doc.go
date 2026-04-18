package doc

import (
	"context"
	"facet/app/pkg/fctlang/loader"
	"facet/app/pkg/fctlang/parser"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
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
	Name      string `json:"name"`      // "Cube" or "Solid.Move"
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

// extractDocEntries extracts doc entries (functions, methods, types, fields) from
// a parsed Source. The library string is set on each entry (empty for stdlib/user code).
func extractDocEntries(src *parser.Source, library string) []DocEntry {
	var entries []DocEntry
	for _, fn := range src.Functions() {
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
			Library:   library,
		})
	}
	for _, sd := range src.StructDecls() {
		entries = append(entries, DocEntry{
			Name:      sd.Name,
			Signature: formatStructSignature(sd),
			Doc:       parser.DocComment(sd.Comments),
			Kind:      "type",
			Library:   library,
		})
		for _, f := range sd.Fields {
			entries = append(entries, DocEntry{
				Name:      fmt.Sprintf("%s.%s", sd.Name, f.Name),
				Signature: f.Type,
				Doc:       fmt.Sprintf("Field of %s (type %s)", sd.Name, f.Type),
				Kind:      "field",
				Library:   library,
			})
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
		entries = append(entries, extractDocEntries(prog, "")...)
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
