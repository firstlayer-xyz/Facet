package runner

import (
	"facet/app/pkg/fctlang/loader"
	"facet/app/pkg/fctlang/parser"
	"testing"
)

func makeTestProgram() (loader.Program, string, string) {
	prog := loader.NewProgram()

	// Stdlib
	stdSrc := &parser.Source{Kind: parser.SourceStdLib, Path: loader.StdlibPath, Text: "# stdlib"}
	prog.Sources[loader.StdlibPath] = stdSrc

	// User file that imports a library
	userKey := "/test/main.fct"
	userSrc, _ := parser.Parse(`var L = lib "mylib@main"`, userKey, parser.SourceUser)
	prog.Sources[userKey] = userSrc

	// Library source
	libKey := "/libs/mylib/mylib.fct"
	libSrc, _ := parser.Parse(`fn Helper() { return Cube(size: 10 mm) }`, libKey, parser.SourceLibrary)
	prog.Sources[libKey] = libSrc

	// Import mapping
	prog.Imports["mylib@main"] = libKey

	return prog, userKey, libKey
}

func newTestRunner(prog loader.Program) (*ProgramRunner, **RunResult) {
	var lastResult *RunResult
	r := &ProgramRunner{
		callbacks:    Callbacks{OnResult: func(result *RunResult) { lastResult = result }},
		resultNotify: make(chan struct{}),
	}
	r.progMu.Lock()
	r.prog = prog
	r.progMu.Unlock()
	return r, &lastResult
}

func TestPruneRemovesUnreachableLibrary(t *testing.T) {
	prog, _, libKey := makeTestProgram()
	r, lastResult := newTestRunner(prog)

	// Close the user tab — only stdlib remains in open tabs
	r.PruneSources([]string{})

	if _, exists := r.prog.Sources[libKey]; exists {
		t.Error("library should have been pruned — no open tab references it")
	}
	if *lastResult == nil {
		t.Error("expected a result to be pushed after pruning")
	}
}

func TestPruneKeepsOpenLibraryTab(t *testing.T) {
	prog, _, libKey := makeTestProgram()
	r, _ := newTestRunner(prog)

	// User closed the user tab but library tab is still open
	r.PruneSources([]string{libKey})

	if _, exists := r.prog.Sources[libKey]; !exists {
		t.Error("library should NOT be pruned — its tab is open")
	}
}

func TestPruneKeepsTransitiveDeps(t *testing.T) {
	prog, userKey, libKey := makeTestProgram()

	// Add transitive: mylib imports sublib
	sublibKey := "/libs/sublib/sublib.fct"
	sublibSrc, _ := parser.Parse(`fn SubHelper() { return Sphere(radius: 5 mm) }`, sublibKey, parser.SourceLibrary)
	prog.Sources[sublibKey] = sublibSrc

	// Update mylib to import sublib
	libSrc, _ := parser.Parse(`var S = lib "sublib@main"`, libKey, parser.SourceLibrary)
	prog.Sources[libKey] = libSrc
	prog.Imports["sublib@main"] = sublibKey

	r, _ := newTestRunner(prog)

	// User tab is open — both mylib and sublib should stay
	r.PruneSources([]string{userKey})

	if _, exists := r.prog.Sources[libKey]; !exists {
		t.Error("mylib should be kept — user file imports it")
	}
	if _, exists := r.prog.Sources[sublibKey]; !exists {
		t.Error("sublib should be kept — transitively imported via mylib")
	}
}

func TestPruneCascadesWhenUserFileClosed(t *testing.T) {
	prog, _, libKey := makeTestProgram()

	// Add transitive: mylib imports sublib
	sublibKey := "/libs/sublib/sublib.fct"
	sublibSrc, _ := parser.Parse(`fn SubHelper() { return Sphere(radius: 5 mm) }`, sublibKey, parser.SourceLibrary)
	prog.Sources[sublibKey] = sublibSrc

	libSrc, _ := parser.Parse(`var S = lib "sublib@main"`, libKey, parser.SourceLibrary)
	prog.Sources[libKey] = libSrc
	prog.Imports["sublib@main"] = sublibKey

	r, _ := newTestRunner(prog)

	// Close all tabs — everything except stdlib should be pruned
	r.PruneSources([]string{})

	if _, exists := r.prog.Sources[libKey]; exists {
		t.Error("mylib should have been pruned")
	}
	if _, exists := r.prog.Sources[sublibKey]; exists {
		t.Error("sublib should have been pruned (transitive)")
	}
	if _, exists := r.prog.Sources[loader.StdlibPath]; !exists {
		t.Error("stdlib should always be kept")
	}
}

func TestPruneNoChangeNoPush(t *testing.T) {
	prog, userKey, _ := makeTestProgram()
	r, lastResult := newTestRunner(prog)

	// All tabs still open — nothing to prune
	r.PruneSources([]string{userKey})

	if *lastResult != nil {
		t.Error("should not push a result when nothing changed")
	}
}
