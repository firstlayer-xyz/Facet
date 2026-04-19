package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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
	Errors       []parser.SourceError   `json:"errors,omitempty"`
	Sources      map[string]SourceEntry `json:"sources,omitempty"`
	VarTypes     checker.VarTypeMap     `json:"varTypes,omitempty"`
	Declarations *checker.DeclResult    `json:"declarations,omitempty"`
	References   checker.References     `json:"references,omitempty"`
	EntryPoints  []EntryPoint           `json:"entryPoints,omitempty"`
	DocIndex     []doc.DocEntry         `json:"docIndex,omitempty"`

	// Eval data
	Mesh   *meshMeta            `json:"mesh,omitempty"`
	Stats  *evaluator.ModelStats `json:"stats,omitempty"`
	Time   float64               `json:"time,omitempty"`
	PosMap []evaluator.PosEntry  `json:"posMap,omitempty"`

	// Debug data
	DebugFinal []*meshMeta    `json:"debugFinal,omitempty"`
	DebugSteps []debugStepMeta `json:"debugSteps,omitempty"`
}

// loaderOpts returns the library directory and loader options from the current
// config. A corrupt settings file is logged and treated as empty — the user
// should still be able to evaluate code with the built-in stdlib.
func loaderOpts() (string, *loader.Options) {
	libDir, _ := libraryDir()
	cfg, err := loadConfig()
	if err != nil {
		log.Printf("[settings] loaderOpts: %v — no installed libraries for this eval", err)
	}
	opts := &loader.Options{}
	if len(cfg.InstalledLibs) > 0 {
		opts.InstalledLibs = cfg.InstalledLibs
	}
	return libDir, opts
}

// handleEval processes a stateless eval request: Load → Check → Eval → binary response.
// recordRun may be nil; when set, it is invoked with a runSummary after every
// response is written so the get_last_run MCP tool can read the latest stats.
func handleEval(ctx context.Context, w http.ResponseWriter, req evalRequest, recordRun func(runSummary)) {
	start := time.Now()

	// respond finalises header.Time, writes the binary response, and — if a
	// recorder is wired — records a runSummary for get_last_run. Using this
	// helper instead of calling writeBinaryResponse directly guarantees every
	// exit path updates the lastRun slot. Pass solids on the main-path success
	// to populate per-object bboxes and piece counts; nil on error paths.
	respond := func(header *evalResponseHeader, binary []byte, solids []*manifold.Solid) {
		header.Time = time.Since(start).Seconds()
		writeBinaryResponse(w, *header, binary)
		if recordRun != nil {
			recordRun(buildRunSummary(header, req.Entry, req.Key, req.Sources, solids))
		}
	}

	// Load from scratch
	libDir, opts := loaderOpts()
	prog, err := loader.LoadMulti(ctx, req.Sources, req.Key, libDir, opts)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		header := evalResponseHeader{Errors: []parser.SourceError{sourceErrorFromErr(err)}}
		respond(&header, nil, nil)
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
	docIndex := buildDocIndex(docText)

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
		References:   checked.References,
		EntryPoints:  entryPoints,
		DocIndex:     docIndex,
	}

	// If errors or no entry, return check-only response
	if len(checked.Errors) > 0 || req.Entry == "" {
		respond(&header, nil, nil)
		return
	}

	// Entry-point return type is a static constraint: validate before evaluating.
	// A mistyped entry is a code error like any other — surface it in header.Errors
	// instead of running the evaluator just to reject the result.
	if entryErr := checked.ValidateEntryPoint(req.Key, req.Entry); entryErr != nil {
		header.Errors = append(header.Errors, *entryErr)
		respond(&header, nil, nil)
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
			respond(&header, nil, nil)
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

		respond(&header, binaryData, nil)
		return
	}

	evalResult, err := evaluator.Eval(ctx, prog, req.Key, req.Overrides, req.Entry)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		header.Errors = append(header.Errors, sourceErrorFromErr(err))
		respond(&header, nil, nil)
		return
	}

	merged := manifold.MergeExtractExpandedMeshes(evalResult.Solids, 40)
	stats := evalResult.Stats
	stats.Triangles += merged.IndexCount / 3
	stats.Vertices += merged.VertexCount
	globalMin, globalMax := solidBounds(evalResult.Solids)
	stats.BBoxMin = globalMin
	stats.BBoxMax = globalMax

	var binaryData []byte
	meta, binaryData := appendMeshBinary(binaryData, merged)
	header.Mesh = meta
	header.Stats = &stats
	header.PosMap = evalResult.PosMap
	respond(&header, binaryData, evalResult.Solids)
}

// buildRunSummary extracts a runSummary from an eval response header, the
// raw request sources (user-authored tab contents), and — optionally — the
// list of top-level solids. Used by handleEval's respond helper to feed
// get_last_run. solids is nil on check-only and error paths.
func buildRunSummary(header *evalResponseHeader, entry, key string, sources map[string]string, solids []*manifold.Solid) runSummary {
	s := runSummary{
		Errors:  header.Errors,
		TimeSec: header.Time,
		Entry:   entry,
		Key:     key,
		Sources: sources,
		Ok:      len(header.Errors) == 0 && header.Stats != nil,
	}
	if header.Stats != nil {
		s.Triangles = header.Stats.Triangles
		s.Vertices = header.Stats.Vertices
		s.Volume = header.Stats.Volume
		s.SurfaceArea = header.Stats.SurfaceArea
		s.BBoxMin = header.Stats.BBoxMin
		s.BBoxMax = header.Stats.BBoxMax
	}
	for _, sol := range solids {
		if sol == nil {
			continue
		}
		minX, minY, minZ, maxX, maxY, maxZ := sol.BoundingBox()
		s.Objects = append(s.Objects, objectSummary{
			BBoxMin: [3]float64{minX, minY, minZ},
			BBoxMax: [3]float64{maxX, maxY, maxZ},
			Pieces:  sol.NumComponents(),
		})
	}
	return s
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

// collectLibDocEntries collects deduplicated doc entries from both user-local
// libraries (filesystem) and git-cached virtualized libraries (bare clones).
func collectLibDocEntries() []doc.DocEntry {
	libDir, _ := libraryDir()
	var entries []doc.DocEntry
	seen := map[string]bool{}
	collect := func(batch []doc.DocEntry) {
		for _, e := range batch {
			key := e.Name + "|" + e.Library
			if seen[key] {
				continue
			}
			seen[key] = true
			entries = append(entries, e)
		}
	}
	collect(doc.BuildLibDocEntries(libDir))
	collect(doc.BuildCachedLibDocEntries(loader.DefaultGitCacheDir()))
	return entries
}

// buildDocIndex builds the documentation index from the primary source and library directories.
func buildDocIndex(sourceText string) []doc.DocEntry {
	entries := doc.BuildDocIndex(sourceText, nil)
	seen := make(map[string]bool)
	for _, e := range entries {
		seen[e.Name+"|"+e.Library] = true
	}
	for _, e := range collectLibDocEntries() {
		key := e.Name + "|" + e.Library
		if !seen[key] {
			seen[key] = true
			entries = append(entries, e)
		}
	}
	return entries
}

// checkRequest is the JSON body of a POST /check request.
type checkRequest struct {
	Source string `json:"source"`
	Key    string `json:"key"`
}

// checkResult is the JSON response from /check and the check_syntax MCP tool.
type checkResult struct {
	Valid  bool                  `json:"valid"`
	Errors []parser.SourceError  `json:"errors"`
}

// checkSource parses and type-checks source code without evaluating it.
// Shared between the /check HTTP endpoint and the check_syntax MCP tool
// so both produce identical output and we do not self-loop over HTTP.
func checkSource(ctx context.Context, source, key string) checkResult {
	if key == "" {
		key = "check.fct"
	}
	libDir, opts := loaderOpts()
	prog, err := loader.Load(ctx, source, key, parser.SourceUser, libDir, opts)
	if err != nil {
		return checkResult{Valid: false, Errors: []parser.SourceError{sourceErrorFromErr(err)}}
	}
	checked := checker.Check(prog)
	if len(checked.Errors) > 0 {
		return checkResult{Valid: false, Errors: checked.Errors}
	}
	return checkResult{Valid: true, Errors: []parser.SourceError{}}
}

// handleCheck parses and type-checks source code without evaluating it.
func handleCheck(w http.ResponseWriter, r *http.Request) {
	var req checkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	result := checkSource(r.Context(), req.Source, req.Key)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// evalSolids runs a fresh Load → Check → Eval and returns the resulting solids.
// Used by export/slicer which need geometry but not a binary HTTP response.
func evalSolids(ctx context.Context, req evalRequest) ([]*manifold.Solid, error) {
	libDir, opts := loaderOpts()
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
	if entryErr := checked.ValidateEntryPoint(req.Key, req.Entry); entryErr != nil {
		return nil, fmt.Errorf("%s", entryErr.Message)
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
