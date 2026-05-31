package checker

import (
	"facet/pkg/fctlang/loader"
	"facet/pkg/fctlang/parser"
	"testing"
)

// findSymbol returns the first Symbol matching name + receiver.
func findSymbol(syms []Symbol, name, receiver string) *Symbol {
	for i := range syms {
		if syms[i].Name == name && syms[i].Receiver == receiver {
			return &syms[i]
		}
	}
	return nil
}

// TestBuildSymbolsStdlibPresent pins the contract that stdlib
// functions and types are emitted with no library tag, so top-level
// completion's `library === ""` filter finds them.
func TestBuildSymbolsStdlibPresent(t *testing.T) {
	prog := parseTestProg(t, `var x = 1;`)
	syms := BuildSymbols(prog, testMainKey)

	cube := findSymbol(syms, "Cube", "")
	if cube == nil {
		t.Fatal("expected stdlib Cube in symbol table")
	}
	if cube.Library != "" {
		t.Errorf("stdlib Cube should have library=\"\", got %q", cube.Library)
	}
	if cube.Kind != "function" {
		t.Errorf("Cube kind should be \"function\", got %q", cube.Kind)
	}
	if cube.Signature == "" {
		t.Error("Cube should have a signature")
	}
}

// TestBuildSymbolsBuiltinsPresent verifies the synthetic types and
// keywords ride along so hover on `x Number` or `fn` still finds them.
func TestBuildSymbolsBuiltinsPresent(t *testing.T) {
	prog := parseTestProg(t, `var x = 1;`)
	syms := BuildSymbols(prog, testMainKey)

	if findSymbol(syms, "Length", "") == nil {
		t.Error("expected builtin type Length")
	}
	if findSymbol(syms, "fn", "") == nil {
		t.Error("expected keyword fn")
	}
}

// TestBuildSymbolsUserFunction confirms user functions in the entry
// source appear with library="" and the right kind.
func TestBuildSymbolsUserFunction(t *testing.T) {
	prog := parseTestProg(t, `fn myFunc(x Length) Length { return x }`)
	syms := BuildSymbols(prog, testMainKey)

	fn := findSymbol(syms, "myFunc", "")
	if fn == nil {
		t.Fatal("expected user function myFunc in symbol table")
	}
	if fn.Library != "" {
		t.Errorf("user function should have library=\"\", got %q", fn.Library)
	}
	if fn.Kind != "function" {
		t.Errorf("kind should be \"function\", got %q", fn.Kind)
	}
}

// TestBuildSymbolsUserMethod confirms methods carry their receiver
// type so instance-dot completion can filter by it.
func TestBuildSymbolsUserMethod(t *testing.T) {
	prog := parseTestProg(t, `
type Box { size Length }
fn Box.Volume() Length { return self.size }
`)
	syms := BuildSymbols(prog, testMainKey)

	m := findSymbol(syms, "Volume", "Box")
	if m == nil {
		t.Fatal("expected method Volume on receiver Box")
	}
	if m.Kind != "method" {
		t.Errorf("kind should be \"method\", got %q", m.Kind)
	}
	if m.Receiver != "Box" {
		t.Errorf("receiver should be \"Box\", got %q", m.Receiver)
	}
}

// TestBuildSymbolsStructAndFields confirms user struct types appear
// and their fields appear with the struct as receiver so dot
// completion on a Box instance finds `size`.
func TestBuildSymbolsStructAndFields(t *testing.T) {
	prog := parseTestProg(t, `type Box { size Length; tag String }`)
	syms := BuildSymbols(prog, testMainKey)

	if findSymbol(syms, "Box", "") == nil {
		t.Error("expected type Box")
	}
	field := findSymbol(syms, "size", "Box")
	if field == nil {
		t.Fatal("expected field size on receiver Box")
	}
	if field.Kind != "field" {
		t.Errorf("kind should be \"field\", got %q", field.Kind)
	}
	if field.Signature != "Length" {
		t.Errorf("field signature should be its type \"Length\", got %q", field.Signature)
	}
}

// TestBuildSymbolsNonEntryUserSourceExcluded pins the fix for the
// multi-tab leakage bug: when a non-entry user source is loaded
// (e.g. another open tab), its declarations are NOT in scope from
// the entry — and the symbol table must not surface them either, or
// completion offers names the checker won't resolve.
func TestBuildSymbolsNonEntryUserSourceExcluded(t *testing.T) {
	prog := parseTestProg(t, `fn EntryFn() Length { return 1 mm }`)

	// Add a second user source that the entry does NOT import.
	otherKey := "/test/other.fct"
	otherSrc, err := parser.Parse(`fn OtherFn() Length { return 2 mm }`, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse other source: %v", err)
	}
	prog.Sources[otherKey] = otherSrc

	syms := BuildSymbols(prog, testMainKey)

	if findSymbol(syms, "EntryFn", "") == nil {
		t.Error("entry source function should appear")
	}
	if findSymbol(syms, "OtherFn", "") != nil {
		t.Error("non-entry user source function must NOT leak as top-level — it's not in scope from the entry")
	}
}

// TestBuildSymbolsLibraryNamespaceCanonical pins the fix for the
// qualified-completion drift bug: every library symbol's Library tag
// must equal loader.LibPathToNamespace applied to the raw import,
// since that is what the checker stamps on varTypes["Library:<ns>"]
// — the editor's dot-completion filter matches the two strings.
func TestBuildSymbolsLibraryNamespaceCanonical(t *testing.T) {
	prog := parseTestProg(t, `var L = lib "github.com/x/lib@main"`)

	// Stub a resolved library source under the import key. The loader
	// would normally do this; for the unit test we mimic the shape.
	libKey := "/test/libs/github.com/x/lib"
	libSrc, err := parser.Parse(`fn DoThing(x Length) Length { return x }`, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse lib source: %v", err)
	}
	prog.Sources[libKey] = libSrc
	prog.Imports["github.com/x/lib@main"] = libKey

	syms := BuildSymbols(prog, testMainKey)

	want := loader.LibPathToNamespace("github.com/x/lib@main")
	if want == "" {
		t.Fatal("LibPathToNamespace returned empty for valid remote path")
	}
	thing := findSymbol(syms, "DoThing", "")
	if thing == nil {
		t.Fatal("expected DoThing from imported library")
	}
	if thing.Library != want {
		t.Errorf("library tag = %q, want %q (the namespace the checker stamps on varTypes)", thing.Library, want)
	}
}

// TestBuildSymbolsDuplicateImportPathsDedup confirms that importing
// the same library at two different refs (e.g. @main and @v1) only
// produces one set of symbols — the loader resolves both raw paths
// to the same disk source, and the symbol-builder dedups by it.
func TestBuildSymbolsDuplicateImportPathsDedup(t *testing.T) {
	prog := parseTestProg(t, `var A = lib "github.com/x/lib@main"; var B = lib "github.com/x/lib@v1"`)

	libKey := "/test/libs/github.com/x/lib"
	libSrc, err := parser.Parse(`fn Once() Length { return 1 mm }`, "", parser.SourceUser)
	if err != nil {
		t.Fatalf("parse lib source: %v", err)
	}
	prog.Sources[libKey] = libSrc
	prog.Imports["github.com/x/lib@main"] = libKey
	prog.Imports["github.com/x/lib@v1"] = libKey

	syms := BuildSymbols(prog, testMainKey)

	count := 0
	for _, s := range syms {
		if s.Name == "Once" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly one Once symbol, got %d", count)
	}
}
