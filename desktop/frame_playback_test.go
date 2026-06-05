package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"net/http/httptest"
	"testing"
)

const cubeAnimSrc = `fn Main() Animation {
    var base = Cube(s: 20 mm)
    return Animation{frame: fn(t Number) Solid {
        const deg = (t % 4000) / 4000 * 360
        return base.Rotate(z: deg * 1 deg)
    }}
}
`

// decodeFrameMesh parses a binary /frame response and returns its header.
func decodeFrameMesh(t *testing.T, rec *httptest.ResponseRecorder) evalResponseHeader {
	t.Helper()
	body := rec.Body.Bytes()
	if len(body) < 4 {
		t.Fatalf("short frame response (%d bytes)", len(body))
	}
	hlen := binary.LittleEndian.Uint32(body[:4])
	var hdr evalResponseHeader
	if err := json.Unmarshal(body[4:4+int(hlen)], &hdr); err != nil {
		t.Fatalf("frame header unmarshal: %v", err)
	}
	return hdr
}

// The live playback loop at the HTTP level: drive /frame several times for a
// rotating cube (as the viewer does each render tick) and assert every response
// carries a non-empty mesh.
func TestFramePlaybackReturnsMeshEachTick(t *testing.T) {
	sources := map[string]string{"main.fct": cubeAnimSrc}
	sessions := newSessionCache()
	for i, tm := range []float64{0, 1700000000000, 1700000000016, 1700000000033, 1700000000050} {
		rec := httptest.NewRecorder()
		handleFrame(context.Background(), rec, frameRequest{
			Sources: sources, Key: "main.fct", Entry: "Main", TimeMs: tm,
		}, sessions)
		hdr := decodeFrameMesh(t, rec)
		if len(hdr.Errors) > 0 {
			t.Fatalf("frame %d (t=%v): unexpected errors %v", i, tm, hdr.Errors)
		}
		if hdr.Mesh == nil || hdr.Mesh.VertexCount == 0 {
			t.Fatalf("frame %d (t=%v): EMPTY mesh (Mesh=%v)", i, tm, hdr.Mesh)
		}
	}
}

// The session is built inside the first /frame request and reused for later
// frames. The cached evaluator must not stay bound to that first request's
// context: once the request returns and its context is canceled, subsequent
// frames must still render. Regression for the "only the first frame renders"
// bug (Frame on the reused evaluator returned "context canceled").
func TestFramePlaybackSurvivesRequestContextCancel(t *testing.T) {
	sources := map[string]string{"main.fct": cubeAnimSrc}
	sessions := newSessionCache()

	// First frame builds the session under a cancelable request context.
	ctx, cancel := context.WithCancel(context.Background())
	rec1 := httptest.NewRecorder()
	handleFrame(ctx, rec1, frameRequest{Sources: sources, Key: "main.fct", Entry: "Main", TimeMs: 0}, sessions)
	if h := decodeFrameMesh(t, rec1); h.Mesh == nil {
		t.Fatalf("first frame produced no mesh: %v", h.Errors)
	}
	cancel() // the building request returns — its context is now canceled

	// A later frame reuses the cached session; it must still render.
	rec2 := httptest.NewRecorder()
	handleFrame(context.Background(), rec2, frameRequest{Sources: sources, Key: "main.fct", Entry: "Main", TimeMs: 1000}, sessions)
	h := decodeFrameMesh(t, rec2)
	if len(h.Errors) > 0 {
		t.Fatalf("second frame after request-context cancel: errors %v", h.Errors)
	}
	if h.Mesh == nil || h.Mesh.VertexCount == 0 {
		t.Fatalf("second frame after request-context cancel: empty mesh — context bug regressed")
	}
}
