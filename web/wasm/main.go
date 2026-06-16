//go:build js

package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"syscall/js"
	"time"

	"facet/pkg/fctlang/checker"
	"facet/pkg/fctlang/entrypoints"
	"facet/pkg/fctlang/evaluator"
	"facet/pkg/fctlang/loader"
	"facet/pkg/fctlang/parser"
	"facet/pkg/manifold"
	"facet/share/examples"
)

type blobRef struct {
	Offset int `json:"offset"`
	Size   int `json:"size"`
}

type meshMeta struct {
	VertexCount    int               `json:"vertexCount"`
	IndexCount     int               `json:"indexCount"`
	FaceGroupCount int               `json:"faceGroupCount"`
	FaceColors     map[string]string `json:"faceColors,omitempty"`
	Expanded       *blobRef          `json:"expanded,omitempty"`
	ExpandedCount  int               `json:"expandedCount,omitempty"`
	Colors         *blobRef          `json:"colors,omitempty"` // uint8 RGB per expanded vertex
	EdgeLines      *blobRef          `json:"edgeLines,omitempty"`
	EdgeCount      int               `json:"edgeCount,omitempty"`
}

type evalResponseHeader struct {
	Errors      []parser.SourceError     `json:"errors,omitempty"`
	EntryPoints []entrypoints.EntryPoint `json:"entryPoints,omitempty"`
	Mesh        *meshMeta                `json:"mesh,omitempty"`
	Stats       *evaluator.ModelStats    `json:"stats,omitempty"`
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	js.Global().Set("facetParse", js.FuncOf(jsParse))
	js.Global().Set("facetEval", js.FuncOf(jsEval))
	js.Global().Set("facetFrame", js.FuncOf(jsFrame))
	js.Global().Set("facetExamples", js.FuncOf(jsExamples))
	js.Global().Set("facetExample", js.FuncOf(jsExample))
	js.Global().Set("facetExport", js.FuncOf(jsExport))
	// Block forever — WASM runtime must stay alive.
	select {}
}

// wasmLoaderOpts returns the loader Options used by every browser-side eval/
// parse. The Cache is in-memory because there is no disk to persist between
// page loads, and the disk-based NativeCache the loader would otherwise fall
// back to would fail on the first os.MkdirAll call. RemoteFetch routes remote
// libraries through jsDelivr instead of a go-git clone (CORS-blocked in the
// browser).
func wasmLoaderOpts() *loader.Options {
	return &loader.Options{
		Cache:       loader.MemoryCache(),
		RemoteFetch: jsDelivrFetch,
	}
}

// jsDelivrFetch resolves a remote library file over HTTP via the jsDelivr CDN,
// which mirrors public GitHub repos with permissive CORS headers — unlike
// GitHub's git endpoints, which the browser blocks. The pinned @ref (SHA,
// tag, or branch) maps straight onto a jsDelivr URL; for SHA pins the response
// is immutable and the browser's HTTP cache persists it across reloads.
//
// In wasm, net/http is backed by the Fetch API and runs inside the eval/parse
// goroutine, so the blocking call yields to the JS event loop.
func jsDelivrFetch(lp *loader.LibPath, subPath string) ([]byte, error) {
	if lp.Host != "github.com" {
		return nil, fmt.Errorf("browser library imports support github.com only (got host %q); use a CORS-friendly mirror or run the desktop app", lp.Host)
	}
	url := fmt.Sprintf("https://cdn.jsdelivr.net/gh/%s/%s@%s/%s", lp.User, lp.Repo, lp.Ref, subPath)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: HTTP %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// jsParse parses source and returns a Promise resolving to entry points
// with parameter constraints.
//
// JS signature: facetParse(source: string) → Promise<{ok: bool, error?: string, entryPoints?: string}>
//
// The Promise wrapping is what lets the loader path do I/O safely. A
// synchronous return would pin the JS event loop on the call frame, so
// fetch() responses for any net/http call inside the parse would never
// fire and Go's deadlock detector would panic. Returning a Promise hands
// control back to the event loop immediately; the work runs in a
// goroutine that can freely block on async I/O while JS services it.
func jsParse(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return resolvedPromise(jsErrorObj("facetParse: expected source string argument"))
	}
	source := args[0].String()
	return newPromise(func(resolve js.Value) {
		defer recoverIntoResolve(resolve, "facetParse")
		ctx := context.Background()
		prog, err := loader.Load(ctx, source, "model.fct", parser.SourceUser, "", wasmLoaderOpts())
		if err != nil {
			resolve.Invoke(jsErrorObj(err.Error()))
			return
		}
		checked := checker.Check(prog)
		if len(checked.Errors) > 0 {
			resolve.Invoke(jsErrorObj(checked.Errors[0].Message))
			return
		}
		eps := entrypoints.Build(checked.Prog, checked.InferredReturnTypes)
		epsJSON, _ := json.Marshal(eps)
		result := js.Global().Get("Object").New()
		result.Set("ok", true)
		result.Set("entryPoints", string(epsJSON))
		resolve.Invoke(result)
	})
}

// jsEval evaluates a Facet source and returns a Promise resolving to a
// Uint8Array in the binary response format.
//
// JS signature: facetEval(source, entryName, overridesJSON) → Promise<Uint8Array | {ok:false, error:string}>
func jsEval(this js.Value, args []js.Value) interface{} {
	if len(args) < 3 {
		return resolvedPromise(jsErrorObj("facetEval: expected (source, entryName, overridesJSON)"))
	}
	source := args[0].String()
	entryName := args[1].String()
	overridesJSON := args[2].String()

	return newPromise(func(resolve js.Value) {
		defer recoverIntoPackedResolve(resolve, "facetEval")

		overrides, oerr := parseOverrides(overridesJSON)
		if oerr != nil {
			resolve.Invoke(jsErrorObj(oerr.Error()))
			return
		}

		ctx := context.Background()
		prog, err := loader.Load(ctx, source, "model.fct", parser.SourceUser, "", wasmLoaderOpts())
		if err != nil {
			bin, _ := packErrorResponse(err.Error())
			resolve.Invoke(bytesToU8(bin))
			return
		}

		checked := checker.Check(prog)
		eps := entrypoints.Build(checked.Prog, checked.InferredReturnTypes)
		header := evalResponseHeader{EntryPoints: eps}

		if len(checked.Errors) > 0 {
			header.Errors = checked.Errors
			bin, _ := packResponse(header, nil)
			resolve.Invoke(bytesToU8(bin))
			return
		}

		result, err := evaluator.Eval(ctx, prog, "model.fct", overrides, entryName)
		if err != nil {
			header.Errors = append(header.Errors, parser.SourceError{Message: err.Error()})
			bin, _ := packResponse(header, nil)
			resolve.Invoke(bytesToU8(bin))
			return
		}

		// An Animation entry has no static solids. Prime the retained session so
		// the playback facetFrame calls that follow reuse this handle, then render
		// a single-frame snapshot at the current time so the canvas isn't blank.
		if result.Animation != nil {
			animSession.store(source, entryName, overridesJSON, result.Animation)
		}
		solids, err := result.StaticSolids(float64(time.Now().UnixMilli()))
		if err != nil {
			header.Errors = append(header.Errors, parser.SourceError{Message: err.Error()})
			bin, _ := packResponse(header, nil)
			resolve.Invoke(bytesToU8(bin))
			return
		}

		dm := manifold.MergeExtractExpandedMeshes(solids, 40)
		stats := result.Stats
		meta, binData := appendMeshBinary(nil, dm)
		header.Mesh = meta
		header.Stats = &stats
		bin, err := packResponse(header, binData)
		if err != nil {
			resolve.Invoke(jsErrorObj(err.Error()))
			return
		}
		resolve.Invoke(bytesToU8(bin))
	})
}

// ── Animation playback ──────────────────────────────────────────────────────

// webSession retains the most recently built Animation so each frame reuses its
// invariant setup (globals, `var base = Expensive()` captures) instead of
// re-running Load → Check → Eval. The browser runs the wasm single-threaded with
// at most one frame in flight, so no locking is needed. A single entry bounds
// memory; any changed input — edited source, different entry, or new overrides
// — evicts and rebuilds.
type webSession struct {
	source        string
	entry         string
	overridesJSON string
	anim          *evaluator.Animation
}

// animSession is the process-wide retained animation. A package global is safe
// here because the wasm event loop serializes every JS→Go call.
var animSession webSession

func (s *webSession) store(source, entry, overridesJSON string, a *evaluator.Animation) {
	s.source = source
	s.entry = entry
	s.overridesJSON = overridesJSON
	s.anim = a
}

// getOrBuild returns the retained Animation for these inputs, rebuilding via
// Load → Check → Eval whenever any input differs. Comparing the inputs directly
// (rather than rebuilding a hash key) keeps the per-frame hot path
// allocation-free. Errors if the entry is not an Animation.
func (s *webSession) getOrBuild(source, entry, overridesJSON string) (*evaluator.Animation, error) {
	if s.anim != nil && s.source == source && s.entry == entry && s.overridesJSON == overridesJSON {
		return s.anim, nil
	}
	// Inputs changed (or first build): drop the old Animation up front so a
	// failed rebuild leaves an empty session rather than one still bound to the
	// previous inputs.
	s.anim = nil

	result, err := evalEntry(source, entry, overridesJSON)
	if err != nil {
		return nil, err
	}
	if result.Animation == nil {
		return nil, fmt.Errorf("entry %q is not an Animation", entry)
	}
	s.store(source, entry, overridesJSON, result.Animation)
	return result.Animation, nil
}

// evalEntry runs the browser-side Load → Check → Eval sequence for one entry,
// returning the evaluator result or the first error. Shared by the animation
// session build and mesh export so the parse/check/eval steps live in one
// place. (jsEval keeps its own copy because it also surfaces entry points and
// per-stage errors back to the viewer.)
func evalEntry(source, entry, overridesJSON string) (*evaluator.EvalResult, error) {
	overrides, err := parseOverrides(overridesJSON)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	prog, err := loader.Load(ctx, source, "model.fct", parser.SourceUser, "", wasmLoaderOpts())
	if err != nil {
		return nil, err
	}
	checked := checker.Check(prog)
	if len(checked.Errors) > 0 {
		return nil, fmt.Errorf("%s", checked.Errors[0].Message)
	}
	return evaluator.Eval(ctx, prog, "model.fct", overrides, entry)
}

// jsFrame renders one frame of an Animation entry at timeMs, reusing the
// retained session so only the frame closure re-runs.
//
// JS signature: facetFrame(source, entryName, overridesJSON, timeMs) → Promise<Uint8Array | {ok:false, error:string}>
func jsFrame(this js.Value, args []js.Value) interface{} {
	if len(args) < 4 {
		return resolvedPromise(jsErrorObj("facetFrame: expected (source, entryName, overridesJSON, timeMs)"))
	}
	source := args[0].String()
	entryName := args[1].String()
	overridesJSON := args[2].String()
	timeMs := args[3].Float()

	return newPromise(func(resolve js.Value) {
		defer recoverIntoPackedResolve(resolve, "facetFrame")

		anim, err := animSession.getOrBuild(source, entryName, overridesJSON)
		if err != nil {
			bin, _ := packErrorResponse(err.Error())
			resolve.Invoke(bytesToU8(bin))
			return
		}
		solid, err := anim.Frame(timeMs)
		if err != nil {
			bin, _ := packErrorResponse(err.Error())
			resolve.Invoke(bytesToU8(bin))
			return
		}
		bin, err := packSolidFrame(solid)
		if err != nil {
			resolve.Invoke(jsErrorObj(err.Error()))
			return
		}
		resolve.Invoke(bytesToU8(bin))
	})
}

// packSolidFrame builds the binary response (mesh + per-frame stats) for a
// single Solid — the shape the viewer expects from a frame render.
func packSolidFrame(solid *manifold.Solid) ([]byte, error) {
	dm := manifold.MergeExtractExpandedMeshes([]*manifold.Solid{solid}, 40)
	meta, binData := appendMeshBinary(nil, dm)
	stats := evaluator.SolidFrameStats(solid, dm)
	return packResponse(evalResponseHeader{Mesh: meta, Stats: &stats}, binData)
}

// ── Mesh export (download) ────────────────────────────────────────────────────

// jsExport evaluates a Facet source and returns a Promise resolving to the
// serialized mesh bytes for a browser download. The bytes come from the same
// manifold.EncodeSolidMesh serializer the desktop app writes to disk, so a web
// download is byte-for-byte the desktop export (3MF carries the same per-face
// colors). An Animation entry exports the frame at timeMs — the geometry the
// viewer is currently showing — rather than erroring.
//
// JS signature: facetExport(source, entry, overridesJSON, format, timeMs) → Promise<Uint8Array | {ok:false, error:string}>.
// format is "3mf" or "stl"; .fct source download is handled in JS (no eval).
func jsExport(this js.Value, args []js.Value) interface{} {
	if len(args) < 5 {
		return resolvedPromise(jsErrorObj("facetExport: expected (source, entryName, overridesJSON, format, timeMs)"))
	}
	source := args[0].String()
	entryName := args[1].String()
	overridesJSON := args[2].String()
	format := args[3].String()
	timeMs := args[4].Float()

	return newPromise(func(resolve js.Value) {
		defer recoverIntoResolve(resolve, "facetExport")

		dm, err := exportDisplayMesh(source, entryName, overridesJSON, timeMs)
		if err != nil {
			resolve.Invoke(jsErrorObj(err.Error()))
			return
		}
		verts, indices, faceHex := displayMeshForExport(dm)
		data, err := manifold.EncodeSolidMesh(verts, indices, faceHex, format, nil)
		if err != nil {
			resolve.Invoke(jsErrorObj(err.Error()))
			return
		}
		resolve.Invoke(bytesToU8(data))
	})
}

// exportDisplayMesh evaluates source+entry and returns the merged DisplayMesh to
// serialize — the same StaticSolids → merge the viewer renders, so a download is
// exactly what's on screen. For an Animation entry StaticSolids renders the
// frame at timeMs. EncodeSolidMesh rejects an empty mesh downstream.
func exportDisplayMesh(source, entryName, overridesJSON string, timeMs float64) (*manifold.DisplayMesh, error) {
	result, err := evalEntry(source, entryName, overridesJSON)
	if err != nil {
		return nil, err
	}
	solids, err := result.StaticSolids(timeMs)
	if err != nil {
		return nil, err
	}
	return manifold.MergeExtractExpandedMeshes(solids, 40), nil
}

// displayMeshForExport flattens an extracted DisplayMesh into the inputs
// EncodeSolidMesh expects: expanded triangle vertices (3 unshared verts per
// triangle), a sequential index list, and one hex color per triangle resolved
// from the face-id → color map. The expanded form is exact for STL (triangle
// soup) and a valid, slicer-weldable 3MF.
func displayMeshForExport(dm *manifold.DisplayMesh) ([]float32, []uint32, []string) {
	verts := dm.ExpandedPositions()
	indices := make([]uint32, len(verts)/3)
	for i := range indices {
		indices[i] = uint32(i)
	}

	numTris := len(indices) / 3
	faceHex := make([]string, numTris)
	for t := 0; t < numTris; t++ {
		if id, ok := dm.FaceIDForVertex(t * 3); ok {
			if hex, ok := dm.FaceColorMap[strconv.FormatUint(uint64(id), 10)]; ok {
				faceHex[t] = hex
			}
		}
	}
	return verts, indices, faceHex
}

// parseOverrides decodes the JSON overrides string shared by eval and frame
// requests. An empty or "{}" payload yields a nil map (no overrides).
func parseOverrides(overridesJSON string) (map[string]interface{}, error) {
	if overridesJSON == "" || overridesJSON == "{}" {
		return nil, nil
	}
	var overrides map[string]interface{}
	if err := json.Unmarshal([]byte(overridesJSON), &overrides); err != nil {
		return nil, fmt.Errorf("invalid overrides JSON: %v", err)
	}
	return overrides, nil
}

// recoverIntoPackedResolve turns a goroutine panic into a packed binary error
// response on the Promise, so a panic surfaces as a normal eval/frame error
// instead of leaving the Promise pending forever.
func recoverIntoPackedResolve(resolve js.Value, fnName string) {
	if r := recover(); r != nil {
		msg := fmt.Sprintf("panic in %s: %v\n%s", fnName, r, debug.Stack())
		fmt.Println(msg)
		bin, _ := packErrorResponse(msg)
		resolve.Invoke(bytesToU8(bin))
	}
}

// jsExamples returns a JSON array of bundled example names, with Tutorial.fct
// first and the remainder sorted alphabetically.
//
// JS signature: facetExamples() → string  (synchronous — no Promise)
func jsExamples(this js.Value, args []js.Value) any {
	entries, err := examples.FS.ReadDir(".")
	if err != nil {
		// embed.FS.ReadDir never fails for a valid embed; this is defensive.
		return "[]"
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".fct") {
			names = append(names, e.Name())
		}
	}
	// Tutorial.fct first, remainder alphabetical (ReadDir already returns
	// lexicographic order, so a stable partition is sufficient).
	sort.SliceStable(names, func(i, j int) bool {
		if names[i] == "Tutorial.fct" {
			return true
		}
		if names[j] == "Tutorial.fct" {
			return false
		}
		return names[i] < names[j]
	})
	b, _ := json.Marshal(names)
	return string(b)
}

// jsExample returns the source of the named bundled example as a string.
// If the name is unknown or unreadable, returns a JS object {error: "..."}.
//
// JS signature: facetExample(name: string) → string | {error: string}  (synchronous)
func jsExample(this js.Value, args []js.Value) any {
	if len(args) < 1 {
		return js.ValueOf(map[string]any{"error": "facetExample: expected name argument"})
	}
	name := args[0].String()
	data, err := examples.FS.ReadFile(name)
	if err != nil {
		return js.ValueOf(map[string]any{"error": err.Error()})
	}
	return string(data)
}

// newPromise constructs a JS Promise whose body runs in a Go goroutine so
// the caller's JS event loop is free to service any async I/O the work
// triggers (fetch, timers, etc.). The body must call resolve exactly once.
//
// We don't expose `reject` because every result path here normalises errors
// into the result object (`{ok: false, error: string}` for parse, packed
// error response for eval) so JS callers always see one resolution shape.
func newPromise(body func(resolve js.Value)) js.Value {
	handler := js.FuncOf(func(_ js.Value, args []js.Value) interface{} {
		resolve := args[0]
		go body(resolve)
		return nil
	})
	// The Promise constructor invokes handler synchronously, so by the
	// time New returns the goroutine is already spawned and we can
	// release the wrapper.
	promise := js.Global().Get("Promise").New(handler)
	handler.Release()
	return promise
}

// resolvedPromise returns a Promise pre-resolved to v — used for the
// argument-validation error paths where we want to keep the JS-side
// signature uniform (always a Promise) without spinning up a goroutine.
func resolvedPromise(v interface{}) js.Value {
	return js.Global().Get("Promise").Call("resolve", v)
}

// recoverIntoResolve turns any goroutine panic into a resolved error
// object on the Promise. Without this, a panic would leave the Promise
// hanging forever.
func recoverIntoResolve(resolve js.Value, fnName string) {
	if r := recover(); r != nil {
		msg := fmt.Sprintf("panic in %s: %v\n%s", fnName, r, debug.Stack())
		fmt.Println(msg)
		resolve.Invoke(jsErrorObj(msg))
	}
}

// ── Binary response format ────────────────────────────────────────────────────

// packResponse packs [4B LE header-len][JSON header][binary data] — the same
// format used by the Wails app eval handler.
func packResponse(header evalResponseHeader, data []byte) ([]byte, error) {
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return nil, fmt.Errorf("marshal header: %w", err)
	}
	buf := make([]byte, 4+len(headerJSON)+len(data))
	binary.LittleEndian.PutUint32(buf[:4], uint32(len(headerJSON)))
	copy(buf[4:], headerJSON)
	copy(buf[4+len(headerJSON):], data)
	return buf, nil
}

func packErrorResponse(msg string) ([]byte, error) {
	return packResponse(evalResponseHeader{
		Errors: []parser.SourceError{{Message: msg}},
	}, nil)
}

func appendMeshBinary(buf []byte, dm *manifold.DisplayMesh) (*meshMeta, []byte) {
	if dm == nil || (dm.ExpandedCount == 0 && dm.VertexCount == 0) {
		return nil, buf
	}
	meta := &meshMeta{
		VertexCount:    dm.VertexCount,
		IndexCount:     dm.IndexCount,
		FaceGroupCount: dm.FaceGroupCount,
		FaceColors:     dm.FaceColorMap,
	}

	if len(dm.ExpandedRaw) > 0 {
		ref := blobRef{Offset: len(buf), Size: len(dm.ExpandedRaw)}
		meta.Expanded = &ref
		meta.ExpandedCount = dm.ExpandedCount
		buf = append(buf, dm.ExpandedRaw...)

		if cols := dm.ExpandedColors(); len(cols) > 0 {
			cref := blobRef{Offset: len(buf), Size: len(cols)}
			meta.Colors = &cref
			buf = append(buf, cols...)
		}
	}
	if len(dm.EdgeLinesRaw) > 0 {
		ref := blobRef{Offset: len(buf), Size: len(dm.EdgeLinesRaw)}
		meta.EdgeLines = &ref
		meta.EdgeCount = dm.EdgeCount
		buf = append(buf, dm.EdgeLinesRaw...)
	}
	return meta, buf
}

// bytesToU8 copies a Go byte slice into a JS Uint8Array.
func bytesToU8(b []byte) js.Value {
	u8 := js.Global().Get("Uint8Array").New(len(b))
	js.CopyBytesToJS(u8, b)
	return u8
}

func jsErrorObj(msg string) js.Value {
	result := js.Global().Get("Object").New()
	result.Set("ok", false)
	result.Set("error", msg)
	return result
}

// Entry-point extraction lives in pkg/fctlang/entrypoints, shared with the
// desktop app so both detect entries (including Animation) and present
// parameters identically.
