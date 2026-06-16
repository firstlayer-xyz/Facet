//go:build ignore

// check-shims verifies the web (wasm) build's JS bridge stays in sync with the
// native (cgo) build, so a hand-written shim can't silently diverge from the
// C++ kernel the way Insert once did (it faked `Difference().Union()` instead
// of forwarding to facet_insert).
//
// It enforces three invariants by scanning source — no compilation needed:
//
//  1. Every C function the NATIVE build calls (C.facet_X in pkg/manifold/*.go)
//     is reachable from the web bridge (M._facet_X in web/index.html), unless
//     it's on the documented exceptions list (browser stubs / different mesh
//     extraction path).
//  2. Every _mf_X shim the wasm Go code calls (pkg/manifold/*_js.go) is defined
//     as globalThis._mf_X in web/index.html (no dangling bridge calls).
//  3. Every C function the bridge reaches (M._facet_X) is actually declared in
//     facet_cxx.h (so it exists in the compiled wasm).
//
// Run from the repo root: `go run scripts/check-shims.go` (or `make check-shims`).

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// exceptions are native C functions intentionally NOT reachable from the web
// bridge, with the reason. Adding a new one is a deliberate, reviewed choice.
var exceptions = map[string]string{
	"facet_text_to_cross_section":      "text rendering is a wasm stub (FreeType excluded from the web build)",
	"facet_extract_display_mesh":       "web uses the expanded-mesh extraction path instead",
	"facet_merge_extract_display_mesh": "web uses the expanded-mesh extraction path instead",
	"facet_extract_mesh_with_runs":     "web uses the expanded-mesh extraction path instead",
}

var (
	reHeaderFunc = regexp.MustCompile(`\bfacet_[a-z0-9_]+\s*\(`)
	reNativeCall = regexp.MustCompile(`C\.(facet_[a-z0-9_]+)`)
	reMfCall     = regexp.MustCompile(`(_mf_[a-z0-9_]+)`)
	reMfDef      = regexp.MustCompile(`globalThis\.(_mf_[a-z0-9_]+)\s*=`)
	reBridgeCall = regexp.MustCompile(`M\._(facet_[a-z0-9_]+)`)
)

func mustRead(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "check-shims: cannot read %s: %v\n", path, err)
		os.Exit(2)
	}
	return string(b)
}

// names collects the first capture group of every match across the given text.
func names(re *regexp.Regexp, text string) map[string]bool {
	out := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(text, -1) {
		out[m[1]] = true
	}
	return out
}

func sorted(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func main() {
	manifoldGo, err := filepath.Glob("pkg/manifold/*.go")
	if err != nil || len(manifoldGo) == 0 {
		fmt.Fprintln(os.Stderr, "check-shims: run from the repo root (no pkg/manifold/*.go found)")
		os.Exit(2)
	}

	// Header: declared C functions.
	header := mustRead("pkg/manifold/cxx/include/facet_cxx.h")
	headerFuncs := map[string]bool{}
	for _, m := range reHeaderFunc.FindAllString(header, -1) {
		name := strings.TrimRight(strings.TrimSpace(m), "(")
		name = strings.TrimSpace(name)
		headerFuncs[name] = true
	}

	// Native C.facet_X calls (non-js, non-test) and wasm _mf_X calls (js).
	nativeC := map[string]bool{}
	mfCalls := map[string]bool{}
	for _, f := range manifoldGo {
		base := filepath.Base(f)
		if strings.HasSuffix(base, "_test.go") {
			continue
		}
		src := mustRead(f)
		if strings.HasSuffix(base, "_js.go") {
			for k := range names(reMfCall, src) {
				mfCalls[k] = true
			}
		} else {
			for k := range names(reNativeCall, src) {
				nativeC[k] = true
			}
		}
	}

	// Web bridge: defined _mf_X shims and the C funcs they reach.
	indexHTML := mustRead("web/index.html")
	mfDefs := names(reMfDef, indexHTML)
	bridgeC := names(reBridgeCall, indexHTML)

	var problems []string

	// Invariant 1: native C calls must be reachable from the web bridge.
	for _, fn := range sorted(nativeC) {
		if bridgeC[fn] || exceptions[fn] != "" {
			continue
		}
		problems = append(problems, fmt.Sprintf(
			"native build calls C.%s but the web bridge has no M._%s shim — the web op may be silently diverging.\n"+
				"      Fix: add `globalThis._mf_%s = (...) => (M._%s(..., SOLID_RET), solidPtr());` to web/index.html and forward to it from the matching *_js.go,\n"+
				"      or, if it's intentionally browser-only-unsupported, add %q to the exceptions list in scripts/check-shims.go with a reason.",
			fn, strings.TrimPrefix(fn, "facet_"), strings.TrimPrefix(fn, "facet_"), fn, fn))
	}

	// Invariant 2: every _mf_X the wasm Go code calls must be defined in index.html.
	for _, m := range sorted(mfCalls) {
		if !mfDefs[m] {
			problems = append(problems, fmt.Sprintf(
				"a *_js.go file calls %s but web/index.html defines no `globalThis.%s` shim (dangling bridge call → runtime failure in the browser).", m, m))
		}
	}

	// Invariant 3: every C func the bridge reaches must be declared in the header.
	for _, fn := range sorted(bridgeC) {
		if !headerFuncs[fn] {
			problems = append(problems, fmt.Sprintf(
				"web/index.html calls M._%s but facet_cxx.h declares no %s (the function won't exist in the wasm).", fn, fn))
		}
	}

	if len(problems) > 0 {
		fmt.Fprintf(os.Stderr, "check-shims: %d shim consistency problem(s):\n", len(problems))
		for _, p := range problems {
			fmt.Fprintf(os.Stderr, "  - %s\n", p)
		}
		os.Exit(1)
	}

	fmt.Printf("check-shims: OK — %d native C calls all reachable from the web bridge (%d exceptions), %d bridge calls all declared.\n",
		len(nativeC), len(exceptions), len(bridgeC))
}
