package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"facet/app/pkg/fctlang/checker"
	"facet/app/pkg/fctlang/doc"
	"facet/app/pkg/fctlang/evaluator"
	"facet/app/pkg/fctlang/loader"
	"facet/app/pkg/fctlang/parser"
	"facet/app/pkg/manifold"
)

// evalRequest is the JSON body of a POST /eval request.
type evalRequest struct {
	Sources   map[string]string      `json:"sources"`   // all open tabs: path → content
	Key       string                 `json:"key"`       // active tab (entry point file)
	Entry     string                 `json:"entry"`     // entry point function name (empty = check-only)
	Overrides map[string]interface{} `json:"overrides"` // parameter overrides
	Debug     bool                   `json:"debug"`     // debug mode
}

// meshMeta describes a binary mesh section within the response body.
type meshMeta struct {
	VertexCount    int               `json:"vertexCount"`
	IndexCount     int               `json:"indexCount"`
	FaceGroupCount int               `json:"faceGroupCount"`
	FaceColors     map[string]string `json:"faceColors,omitempty"`
	Vertices       blobRef           `json:"vertices"`
	Indices        blobRef           `json:"indices"`
	FaceGroups     *blobRef          `json:"faceGroups,omitempty"`
	// Pre-expanded non-indexed positions (replaces toNonIndexed on frontend)
	Expanded      *blobRef `json:"expanded,omitempty"`
	ExpandedCount int      `json:"expandedCount,omitempty"`
	// Pre-computed edge line segments (replaces EdgesGeometry on frontend)
	EdgeLines *blobRef `json:"edgeLines,omitempty"`
	EdgeCount int      `json:"edgeCount,omitempty"`
}

type blobRef struct {
	Offset int `json:"offset"`
	Size   int `json:"size"`
}

// debugStepMeta describes a debug step in the response header with binary mesh references.
type debugStepMeta struct {
	Op     string          `json:"op"`
	Line   int             `json:"line"`
	Col    int             `json:"col"`
	File   string          `json:"file"`
	Meshes []debugMeshMeta `json:"meshes,omitempty"`
}

type debugMeshMeta struct {
	Role string    `json:"role"`
	Mesh *meshMeta `json:"mesh,omitempty"`
}

// evalResponseHeader is the JSON header of a binary eval response.
type evalResponseHeader struct {
	Errors       []parser.SourceError          `json:"errors,omitempty"`
	Sources      map[string]SourceEntry  `json:"sources,omitempty"`
	VarTypes     checker.VarTypeMap             `json:"varTypes,omitempty"`
	Declarations *checker.DeclResult            `json:"declarations,omitempty"`
	EntryPoints  []EntryPoint            `json:"entryPoints,omitempty"`
	DocIndex     []doc.DocEntry                 `json:"docIndex,omitempty"`

	// Eval data
	Mesh   *meshMeta            `json:"mesh,omitempty"`
	Stats  *evaluator.ModelStats `json:"stats,omitempty"`
	Time   float64               `json:"time,omitempty"`
	PosMap []evaluator.PosEntry  `json:"posMap,omitempty"`

	// Debug data
	DebugFinal []*meshMeta    `json:"debugFinal,omitempty"`
	DebugSteps []debugStepMeta `json:"debugSteps,omitempty"`
}

// GetHTTPPort returns the port of the localhost HTTP server (eval, MCP, etc.).
// Exposed to the frontend via Wails binding.
func (a *App) GetHTTPPort() int {
	return a.mcpPort
}

// handleEval processes a stateless eval request: Load → Check → Eval → binary response.
func handleEval(ctx context.Context, w http.ResponseWriter, req evalRequest) {
	start := time.Now()

	// Load from scratch
	libDir, _ := libraryDir()
	cfg := loadConfig()
	opts := &loader.Options{}
	if len(cfg.InstalledLibs) > 0 {
		opts.InstalledLibs = cfg.InstalledLibs
	}
	prog, err := loader.LoadMulti(ctx, req.Sources, req.Key, libDir, opts)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		writeEvalError(w, err)
		return
	}

	// Check
	checked := checker.Check(prog)

	// Build sources map for frontend
	sources := buildSourcesMap(prog)

	// Build doc index
	docText := ""
	if docSrc := prog.Sources[req.Key]; docSrc != nil {
		docText = docSrc.Text
	}
	docIndex := buildDocIndex(libDir, docText)

	// Build entry points
	var entryPoints []EntryPoint
	if checked.Prog.Sources != nil {
		entryPoints = getEntryPoints(checked.Prog, checked.InferredReturnTypes)
	}

	header := evalResponseHeader{
		Errors:       checked.Errors,
		Sources:      sources,
		VarTypes:     checked.VarTypes,
		Declarations: checked.Declarations,
		EntryPoints:  entryPoints,
		DocIndex:     docIndex,
	}

	// If errors or no entry, return check-only response
	if len(checked.Errors) > 0 || req.Entry == "" {
		header.Time = time.Since(start).Seconds()
		writeBinaryResponse(w, header, nil)
		return
	}

	// Eval or Debug
	if req.Debug {
		debugResult, err := evaluator.EvalDebug(ctx, prog, req.Key, req.Overrides, req.Entry)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			header.Errors = append(header.Errors, sourceErrorFromErr(err))
			writeBinaryResponse(w, header, nil)
			return
		}
		var binaryData []byte

		// Build final meshes
		for _, s := range debugResult.Solids {
			dm := s.ToDisplayMesh()
			debugResult.Final = append(debugResult.Final, dm)
		}
		for _, dm := range debugResult.Final {
			meta, data := appendMeshBinary(binaryData, dm)
			binaryData = data
			header.DebugFinal = append(header.DebugFinal, meta)
		}

		// Resolve and serialize all step meshes into binary
		for i := range debugResult.Steps {
			step := &debugResult.Steps[i]
			meshes := debugResult.ResolveMeshes(i)
			sm := debugStepMeta{
				Op:   step.Op,
				Line: step.Line,
				Col:  step.Col,
				File: step.File,
			}
			for _, dm := range meshes {
				meta, data := appendMeshBinary(binaryData, dm.Mesh)
				binaryData = data
				sm.Meshes = append(sm.Meshes, debugMeshMeta{Role: dm.Role, Mesh: meta})
			}
			header.DebugSteps = append(header.DebugSteps, sm)
		}

		header.Time = time.Since(start).Seconds()
		writeBinaryResponse(w, header, binaryData)
		return
	}

	evalResult, err := evaluator.Eval(ctx, prog, req.Key, req.Overrides, req.Entry)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		header.Errors = append(header.Errors, sourceErrorFromErr(err))
		writeBinaryResponse(w, header, nil)
		return
	}

	merged := manifold.MergeExtractExpandedMeshes(evalResult.Solids, 40)
	stats := evalResult.Stats
	stats.Triangles += merged.IndexCount / 3
	stats.Vertices += merged.VertexCount
	_, globalMin, globalMax := solidBBoxes(evalResult.Solids)
	stats.BBoxMin = globalMin
	stats.BBoxMax = globalMax

	var binaryData []byte
	meta, binaryData := appendMeshBinary(binaryData, merged)
	header.Mesh = meta
	header.Stats = &stats
	header.Time = time.Since(start).Seconds()
	header.PosMap = evalResult.PosMap
	writeBinaryResponse(w, header, binaryData)
}

// evalHTTPHandler returns an http.HandlerFunc that cancels the previous eval
// and dispatches to handleEval with a fresh context.
func (a *App) evalHTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// CORS
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req evalRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Cancel previous eval, derive context from request
		a.evalMu.Lock()
		if a.cancelEval != nil {
			a.cancelEval()
		}
		ctx, cancel := context.WithCancel(r.Context())
		a.cancelEval = cancel
		a.evalMu.Unlock()
		defer cancel()

		handleEval(ctx, w, req)
	}
}

// appendMeshBinary appends a DisplayMesh's raw data to buf and returns the metadata.
func appendMeshBinary(buf []byte, dm *manifold.DisplayMesh) (*meshMeta, []byte) {
	if dm == nil || dm.VertexCount == 0 {
		return nil, buf
	}
	meta := &meshMeta{
		VertexCount:    dm.VertexCount,
		IndexCount:     dm.IndexCount,
		FaceGroupCount: dm.FaceGroupCount,
		FaceColors:     dm.FaceColorMap,
	}

	meta.Vertices = blobRef{Offset: len(buf), Size: len(dm.VertRaw)}
	buf = append(buf, dm.VertRaw...)

	meta.Indices = blobRef{Offset: len(buf), Size: len(dm.IdxRaw)}
	buf = append(buf, dm.IdxRaw...)

	if len(dm.FaceGroupRaw) > 0 {
		ref := blobRef{Offset: len(buf), Size: len(dm.FaceGroupRaw)}
		meta.FaceGroups = &ref
		buf = append(buf, dm.FaceGroupRaw...)
	}

	if len(dm.ExpandedRaw) > 0 {
		ref := blobRef{Offset: len(buf), Size: len(dm.ExpandedRaw)}
		meta.Expanded = &ref
		meta.ExpandedCount = dm.ExpandedCount
		buf = append(buf, dm.ExpandedRaw...)
	}

	if len(dm.EdgeLinesRaw) > 0 {
		ref := blobRef{Offset: len(buf), Size: len(dm.EdgeLinesRaw)}
		meta.EdgeLines = &ref
		meta.EdgeCount = dm.EdgeCount
		buf = append(buf, dm.EdgeLinesRaw...)
	}

	return meta, buf
}

// writeBinaryResponse writes a binary-framed response: [4B header len][JSON header][binary data].
func writeBinaryResponse(w http.ResponseWriter, header evalResponseHeader, binaryData []byte) {
	headerJSON, err := json.Marshal(header)
	if err != nil {
		http.Error(w, "internal error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)

	// Write 4-byte header length (uint32 LE)
	var lenBuf [4]byte
	binary.LittleEndian.PutUint32(lenBuf[:], uint32(len(headerJSON)))
	w.Write(lenBuf[:])

	// Write JSON header
	w.Write(headerJSON)

	// Write binary data
	if len(binaryData) > 0 {
		w.Write(binaryData)
	}
}

// writeEvalError writes a binary response with a single error message.
func writeEvalError(w http.ResponseWriter, err error) {
	header := evalResponseHeader{
		Errors: []parser.SourceError{sourceErrorFromErr(err)},
	}
	writeBinaryResponse(w, header, nil)
}

// buildSourcesMap creates the sources map for the frontend from a loaded Program.
func buildSourcesMap(prog loader.Program) map[string]SourceEntry {
	// Build reverse map: disk path → import path
	diskToImport := make(map[string]string)
	for importPath, diskPath := range prog.Imports {
		diskToImport[diskPath] = importPath
	}

	sources := make(map[string]SourceEntry)
	for key, src := range prog.Sources {
		sources[key] = SourceEntry{Text: src.Text, Kind: src.Kind, ImportPath: diskToImport[key]}
	}
	return sources
}

// buildDocIndex builds the documentation index from the primary source and library directories.
func buildDocIndex(libDir, sourceText string) []doc.DocEntry {
	entries := doc.BuildDocIndex(sourceText, nil)
	seen := make(map[string]bool)
	for _, e := range entries {
		seen[e.Name+"|"+e.Library] = true
	}
	for _, dir := range []string{libDir, loader.DefaultGitCacheDir()} {
		for _, e := range doc.BuildLibDocEntries(dir) {
			key := e.Name + "|" + e.Library
			if !seen[key] {
				seen[key] = true
				entries = append(entries, e)
			}
		}
	}
	return entries
}

// checkRequest is the JSON body of a POST /check request.
type checkRequest struct {
	Source string `json:"source"`
	Key    string `json:"key"`
}

// handleCheck parses and type-checks source code without evaluating it.
func handleCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	var req checkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Key == "" {
		req.Key = "check.fct"
	}

	libDir, _ := libraryDir()
	cfg := loadConfig()
	opts := &loader.Options{}
	if len(cfg.InstalledLibs) > 0 {
		opts.InstalledLibs = cfg.InstalledLibs
	}

	prog, err := loader.Load(r.Context(), req.Source, req.Key, parser.SourceUser, libDir, opts)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"valid": false, "errors": []parser.SourceError{sourceErrorFromErr(err)}})
		return
	}

	checked := checker.Check(prog)
	if len(checked.Errors) > 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"valid": false, "errors": checked.Errors})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"valid":true,"errors":[]}`))
}

// evalSolids runs a fresh Load → Check → Eval and returns the resulting solids.
// Used by export/slicer which need geometry but not a binary HTTP response.
func evalSolids(ctx context.Context, req evalRequest) ([]*manifold.Solid, error) {
	libDir, _ := libraryDir()
	cfg := loadConfig()
	opts := &loader.Options{}
	if len(cfg.InstalledLibs) > 0 {
		opts.InstalledLibs = cfg.InstalledLibs
	}
	prog, err := loader.LoadMulti(ctx, req.Sources, req.Key, libDir, opts)
	if err != nil {
		return nil, err
	}
	checked := checker.Check(prog)
	if len(checked.Errors) > 0 {
		return nil, fmt.Errorf("%s", checked.Errors[0].Message)
	}
	if req.Entry == "" {
		return nil, fmt.Errorf("no entry point")
	}
	result, err := evaluator.Eval(ctx, prog, req.Key, req.Overrides, req.Entry)
	if err != nil {
		return nil, err
	}
	return result.Solids, nil
}

// sourceErrorFromErr converts a generic error into a parser.SourceError.
func sourceErrorFromErr(err error) parser.SourceError {
	var se *parser.SourceError
	if errors.As(err, &se) {
		return *se
	}
	return parser.SourceError{Message: err.Error()}
}
