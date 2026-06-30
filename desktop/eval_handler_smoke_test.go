package main

import (
	"context"
	"net/http/httptest"
	"testing"
)

// TestHandleEvalReturnsMeshForCube drives the static /eval path the viewport
// uses on every edit. make test runs pkg/... only, never desktop/, so this path
// had no coverage — a panic or empty mesh here surfaces in the app as a failed
// fetch ("Load failed") and a blank model.
func TestHandleEvalReturnsMeshForCube(t *testing.T) {
	sources := map[string]string{"main.fct": "fn Main() Solid { return Cube(s: 20 mm) }"}
	sessions := newSessionCache()
	rec := httptest.NewRecorder()
	handleEval(context.Background(), rec, evalRequest{
		Sources: sources, Key: "main.fct", Entry: "Main",
	}, func(runSummary) {}, sessions)
	hdr := decodeFrameMesh(t, rec)
	if len(hdr.Errors) > 0 {
		t.Fatalf("unexpected eval errors: %v", hdr.Errors)
	}
	if hdr.Mesh == nil || hdr.Mesh.VertexCount == 0 {
		t.Fatalf("EMPTY mesh for Cube() — Mesh=%v", hdr.Mesh)
	}
}
