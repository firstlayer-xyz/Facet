package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"

	"facet/pkg/fctlang/checker"
	"facet/pkg/fctlang/evaluator"
	"facet/pkg/fctlang/loader"
	"facet/pkg/fctlang/parser"
	"facet/pkg/manifold"
)

// frameRequest is the body of POST /frame. It carries sources so a session
// can be (re)built on a cache miss; timeMs is the time for this frame in
// milliseconds (passed verbatim to the Animation.frame lambda).
type frameRequest struct {
	Sources   map[string]string      `json:"sources"`
	Key       string                 `json:"key"`
	Entry     string                 `json:"entry"`
	Overrides map[string]interface{} `json:"overrides"`
	TimeMs    float64                `json:"timeMs"`
}

// sessionCache retains the most recently evaluated Animation, keyed by a hash
// of (sources + key + entry + overrides). A single entry bounds memory to one
// model's invariant setup; a new key evicts and rebuilds.
type sessionCache struct {
	mu   sync.Mutex
	key  string
	anim *evaluator.Animation
}

func newSessionCache() *sessionCache { return &sessionCache{} }

// sessionKey returns a stable hash over the inputs that uniquely identify a
// compiled Animation: sorted source paths+contents, key, entry, and the
// JSON-marshaled overrides. Any change to these inputs forces a rebuild.
func sessionKey(sources map[string]string, key, entry string, overrides map[string]interface{}) string {
	h := sha256.New()

	// Sort source paths so the hash is stable regardless of map iteration order.
	paths := make([]string, 0, len(sources))
	for p := range sources {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		fmt.Fprintf(h, "%s\x00%s\x00", p, sources[p])
	}

	fmt.Fprintf(h, "key:%s\x00entry:%s\x00", key, entry)

	if len(overrides) > 0 {
		b, _ := json.Marshal(overrides)
		h.Write(b)
	}

	return fmt.Sprintf("%x", h.Sum(nil))
}

// getOrBuild returns the cached Animation for these inputs, building it (full
// Load → Check → Eval) on a cache miss. The caller receives an error when
// parsing, type-checking, or evaluation fails, or when the entry does not
// return an Animation.
func (c *sessionCache) getOrBuild(ctx context.Context, sources map[string]string, key, entry string, overrides map[string]interface{}) (*evaluator.Animation, error) {
	k := sessionKey(sources, key, entry, overrides)

	c.mu.Lock()
	if c.key == k && c.anim != nil {
		anim := c.anim
		c.mu.Unlock()
		return anim, nil
	}
	c.mu.Unlock()

	// Cache miss — build outside the lock so concurrent callers don't pile up
	// behind a slow geometry evaluation. The last writer wins; that is
	// acceptable because identical inputs always produce equivalent Animations.
	libDir, opts := loaderOpts()
	prog, err := loader.LoadMulti(ctx, sources, key, libDir, opts)
	if err != nil {
		return nil, err
	}

	checked := checker.Check(prog)
	if len(checked.Errors) > 0 {
		return nil, fmt.Errorf("%s", checked.Errors[0].Message)
	}

	result, err := evaluator.Eval(ctx, prog, key, overrides, entry)
	if err != nil {
		return nil, err
	}
	if result.Animation == nil {
		return nil, fmt.Errorf("entry %q did not return an Animation", entry)
	}

	anim := result.Animation

	c.mu.Lock()
	c.key = k
	c.anim = anim
	c.mu.Unlock()

	return anim, nil
}

// initialFrameTimeMs is the time used for an animation's initial (static)
// frame from /eval. v1 uses 0; live playback takes over once started.
func initialFrameTimeMs() float64 { return 0 }

// handleFrame serves POST /frame: build-or-reuse the session, render the
// frame at req.TimeMs, and write the SAME binary mesh framing /eval uses.
// A frame-time evaluation error is reported in the header Errors field so
// the frontend can pause; an absent/empty solid is not itself an error.
func handleFrame(ctx context.Context, w http.ResponseWriter, req frameRequest, sessions *sessionCache) {
	anim, err := sessions.getOrBuild(ctx, req.Sources, req.Key, req.Entry, req.Overrides)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		header := evalResponseHeader{Errors: []parser.SourceError{sourceErrorFromErr(err)}}
		writeBinaryResponse(w, header, nil)
		return
	}

	solid, err := anim.Frame(req.TimeMs)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		header := evalResponseHeader{Errors: []parser.SourceError{sourceErrorFromErr(err)}}
		writeBinaryResponse(w, header, nil)
		return
	}

	merged := manifold.MergeExtractExpandedMeshes([]*manifold.Solid{solid}, 40)
	var binaryData []byte
	meta, binaryData := appendMeshBinary(binaryData, merged)
	header := evalResponseHeader{Mesh: meta}
	writeBinaryResponse(w, header, binaryData)
}
