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
	"facet/pkg/fctlang/evaluator"
	"facet/pkg/fctlang/loader"
	"facet/pkg/fctlang/parser"
	"facet/pkg/manifold"
	"facet/share/examples"
)

// ── Parameter / entry-point types ─────────

type ParamConstraint struct {
	Kind      string        `json:"kind"`
	Min       interface{}   `json:"min,omitempty"`
	Max       interface{}   `json:"max,omitempty"`
	Step      interface{}   `json:"step,omitempty"`
	Exclusive bool          `json:"exclusive,omitempty"`
	Values    []interface{} `json:"values,omitempty"`
}

type ParamEntry struct {
	Name       string           `json:"name"`
	Type       string           `json:"type"`
	HasDefault bool             `json:"hasDefault"`
	Default    interface{}      `json:"default"`
	Unit       string           `json:"unit,omitempty"`
	Constraint *ParamConstraint `json:"constraint,omitempty"`
}

type EntryPoint struct {
	Name   string       `json:"name"`
	Params []ParamEntry `json:"params"`
}

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
	Errors      []parser.SourceError  `json:"errors,omitempty"`
	EntryPoints []EntryPoint          `json:"entryPoints,omitempty"`
	Mesh        *meshMeta             `json:"mesh,omitempty"`
	Stats       *evaluator.ModelStats `json:"stats,omitempty"`
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	js.Global().Set("facetParse", js.FuncOf(jsParse))
	js.Global().Set("facetEval", js.FuncOf(jsEval))
	js.Global().Set("facetExamples", js.FuncOf(jsExamples))
	js.Global().Set("facetExample", js.FuncOf(jsExample))
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
		eps := buildEntryPoints(checked.Prog, checked.InferredReturnTypes)
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
		defer func() {
			if r := recover(); r != nil {
				msg := fmt.Sprintf("panic in facetEval: %v\n%s", r, debug.Stack())
				fmt.Println(msg)
				bin, _ := packErrorResponse(msg)
				resolve.Invoke(bytesToU8(bin))
			}
		}()

		var overrides map[string]interface{}
		if overridesJSON != "" && overridesJSON != "{}" {
			if err := json.Unmarshal([]byte(overridesJSON), &overrides); err != nil {
				resolve.Invoke(jsErrorObj(fmt.Sprintf("invalid overrides JSON: %v", err)))
				return
			}
		}

		ctx := context.Background()
		prog, err := loader.Load(ctx, source, "model.fct", parser.SourceUser, "", wasmLoaderOpts())
		if err != nil {
			bin, _ := packErrorResponse(err.Error())
			resolve.Invoke(bytesToU8(bin))
			return
		}

		checked := checker.Check(prog)
		eps := buildEntryPoints(checked.Prog, checked.InferredReturnTypes)
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

		// An Animation entry has no static solids; the web preview can't play it,
		// but render a single-frame snapshot so the canvas isn't blank.
		solids, err := result.StaticSolids(float64(time.Now().UnixMilli()))
		if err != nil {
			header.Errors = append(header.Errors, parser.SourceError{Message: err.Error()})
			bin, _ := packResponse(header, nil)
			resolve.Invoke(bytesToU8(bin))
			return
		}

		dm := manifold.MergeExtractExpandedMeshes(solids, 40)
		stats := result.Stats
		var binData []byte
		meta, binData := appendMeshBinary(binData, dm)
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
		if !e.IsDir() && len(e.Name()) > 4 && e.Name()[len(e.Name())-4:] == ".fct" {
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

		if cols := buildExpandedColors(dm); len(cols) > 0 {
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

// defaultRenderColor matches the legacy flat shader color (0.55, 0.7, 0.88)
// so models without assigned face colors render exactly as before.
var defaultRenderColor = [3]byte{0x8c, 0xb3, 0xe0} // 140, 179, 224

// buildExpandedColors returns a per-expanded-vertex RGB buffer (3 bytes per
// vertex) parallel to dm.ExpandedRaw. Each vertex takes its triangle's face
// color; faces with no assigned color fall back to defaultRenderColor. Alpha
// is intentionally dropped for now — the web viewer renders opaque.
func buildExpandedColors(dm *manifold.DisplayMesh) []byte {
	expVerts := dm.ExpandedCount
	if expVerts == 0 {
		return nil
	}
	// Resolve each face id's hex color once.
	rgb := make(map[uint32][3]byte, len(dm.FaceColorMap))
	for k, hex := range dm.FaceColorMap {
		id, err := strconv.ParseUint(k, 10, 32)
		if err != nil {
			continue
		}
		if c, ok := parseHexRGB(hex); ok {
			rgb[uint32(id)] = c
		}
	}

	// FaceGroupRaw carries one uint32 face id per triangle (the common case)
	// or per expanded vertex; detect which so we index it correctly.
	fgN := len(dm.FaceGroupRaw) / 4
	perVertex := fgN == expVerts
	faceID := func(vert int) (uint32, bool) {
		idx := vert
		if !perVertex {
			idx = vert / 3
		}
		off := idx * 4
		if off+4 > len(dm.FaceGroupRaw) {
			return 0, false
		}
		return binary.LittleEndian.Uint32(dm.FaceGroupRaw[off : off+4]), true
	}

	out := make([]byte, expVerts*3)
	for v := 0; v < expVerts; v++ {
		c := defaultRenderColor
		if fgN > 0 {
			if id, ok := faceID(v); ok {
				if cc, ok := rgb[id]; ok {
					c = cc
				}
			}
		}
		out[v*3], out[v*3+1], out[v*3+2] = c[0], c[1], c[2]
	}
	return out
}

// parseHexRGB parses "#RRGGBB" or "#RRGGBBAA" (alpha ignored) into RGB bytes.
func parseHexRGB(s string) ([3]byte, bool) {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 && len(s) != 8 {
		return [3]byte{}, false
	}
	v, err := strconv.ParseUint(s[:6], 16, 32)
	if err != nil {
		return [3]byte{}, false
	}
	return [3]byte{byte(v >> 16), byte(v >> 8), byte(v)}, true
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

// ── Entry-point extraction ─────────────────

func buildEntryPoints(prog loader.Program, inferredReturnTypes map[string]string) []EntryPoint {
	var out []EntryPoint
	for _, src := range prog.Sources {
		for _, fn := range src.Functions() {
			if !isEntryPoint(fn, inferredReturnTypes) {
				continue
			}
			params := make([]ParamEntry, 0, len(fn.Params))
			for _, p := range fn.Params {
				pe := ParamEntry{
					Name:       p.Name,
					Type:       p.Type,
					HasDefault: p.Default != nil,
				}
				if p.Default != nil {
					pe.Default, _ = literalValue(p.Default)
				}
				pe.Constraint = extractConstraint(p.Constraint)
				pe.Unit = constraintUnit(p.Constraint)
				if pe.Unit == "" {
					pe.Unit = defaultUnit(p.Default)
				}
				params = append(params, pe)
			}
			out = append(out, EntryPoint{Name: fn.Name, Params: params})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i].Name, out[j].Name
		if a == b {
			return false
		}
		if a == "Main" {
			return true
		}
		if b == "Main" {
			return false
		}
		return a < b
	})
	return out
}

func isEntryPoint(fn *parser.Function, inferredReturnTypes map[string]string) bool {
	if fn.ReceiverType != "" || len(fn.Name) == 0 {
		return false
	}
	if fn.Name[0] < 'A' || fn.Name[0] > 'Z' {
		return false
	}
	inferred := inferredReturnTypes[fn.Name]
	if fn.Name != "Main" && fn.ReturnType != "Solid" && inferred != "Solid" {
		return false
	}
	for _, p := range fn.Params {
		if p.Default == nil {
			return false
		}
	}
	return true
}

func extractConstraint(c parser.Expr) *ParamConstraint {
	switch c := c.(type) {
	case *parser.ConstrainedRange:
		pc := &ParamConstraint{Kind: "range", Exclusive: c.Range.Exclusive}
		if min, ok := literalNumber(c.Range.Start); ok {
			pc.Min = min
		}
		if max, ok := literalNumber(c.Range.End); ok {
			pc.Max = max
		}
		if c.Range.Step != nil {
			if step, ok := literalNumber(c.Range.Step); ok {
				pc.Step = step
			}
		}
		return pc
	case *parser.RangeExpr:
		pc := &ParamConstraint{Kind: "range", Exclusive: c.Exclusive}
		if min, ok := literalValue(c.Start); ok {
			pc.Min = min
		}
		if max, ok := literalValue(c.End); ok {
			pc.Max = max
		}
		if c.Step != nil {
			if step, ok := literalValue(c.Step); ok {
				pc.Step = step
			}
		}
		return pc
	case *parser.ArrayLitExpr:
		if len(c.Elems) == 0 {
			return &ParamConstraint{Kind: "free"}
		}
		pc := &ParamConstraint{Kind: "enum"}
		for _, elem := range c.Elems {
			if v, ok := literalValue(elem); ok {
				pc.Values = append(pc.Values, v)
			}
		}
		return pc
	}
	return nil
}

func constraintUnit(c parser.Expr) string {
	switch c := c.(type) {
	case *parser.ConstrainedRange:
		return c.Unit
	case *parser.RangeExpr:
		if u := exprUnit(c.End); u != "" {
			return u
		}
		return exprUnit(c.Start)
	}
	return ""
}

func defaultUnit(e parser.Expr) string {
	if u, ok := e.(*parser.UnitExpr); ok {
		return u.Unit
	}
	return ""
}

func literalNumber(e parser.Expr) (float64, bool) {
	switch v := e.(type) {
	case *parser.NumberLit:
		return v.Value, true
	case *parser.UnitExpr:
		if num, ok := v.Expr.(*parser.NumberLit); ok {
			return num.Value * v.Factor, true
		}
	}
	return 0, false
}

func literalValue(e parser.Expr) (interface{}, bool) {
	switch v := e.(type) {
	case *parser.NumberLit:
		return v.Value, true
	case *parser.UnitExpr:
		if num, ok := v.Expr.(*parser.NumberLit); ok {
			return num.Value * v.Factor, true
		}
	case *parser.StringLit:
		return v.Value, true
	case *parser.BoolLit:
		return v.Value, true
	}
	return nil, false
}

func exprUnit(e parser.Expr) string {
	if u, ok := e.(*parser.UnitExpr); ok {
		return u.Unit
	}
	return ""
}
