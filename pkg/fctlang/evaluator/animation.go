package evaluator

import (
	"context"
	"fmt"
	"sync"

	"facet/pkg/manifold"
)

// Animation is an evaluated, time-driven model. It retains the evaluator that
// produced it so the frame closure's captured invariant setup (and globals)
// stay live and are reused on every Frame call — the model body runs once, not
// per frame. Frame produces the Solid for a given instant.
type Animation struct {
	mu         sync.Mutex
	e          *evaluator
	frame      *functionVal
	argName    string // the frame lambda's sole parameter name (time, in ms)
	baseTracks int    // len(*e.solidTracks) at construction; truncated to this before each frame
}

// StaticSolids returns the solids a one-shot consumer — file export, the CLI,
// the web preview — should render for this result. For an Animation entry it
// renders a single frame at timeMs (a static snapshot) under ctx; otherwise it
// returns the entry's solids unchanged. This keeps non-playback callers from
// silently emitting an empty model when the entry is an Animation.
func (r *EvalResult) StaticSolids(ctx context.Context, timeMs float64) ([]*manifold.Solid, error) {
	if r.Animation != nil {
		s, err := r.Animation.Frame(ctx, timeMs)
		if err != nil {
			return nil, err
		}
		return []*manifold.Solid{s}, nil
	}
	return r.Solids, nil
}

// Frame evaluates the model at timeMs (milliseconds; the frame lambda decides
// how to interpret it) and returns the Solid for that instant.
//
// Frame is concurrency-safe: concurrent calls on the same handle are serialized.
// It is NOT re-entrant — the lock is held across the frame closure, so the
// closure must not call back into Frame on the same handle. ctx cancels a
// runaway frame (e.g. a heavy per-iteration loop).
func (a *Animation) Frame(ctx context.Context, timeMs float64) (*manifold.Solid, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.frameLocked(ctx, timeMs)
}

// FrameWithPosMap evaluates the frame and also returns its source-position →
// face-ID map, so interactive playback supports face-click → source navigation
// exactly as the static render does. The PosMap is built from the live per-frame
// solidTracks under the same lock, before the next Frame truncates them.
func (a *Animation) FrameWithPosMap(ctx context.Context, timeMs float64) (*manifold.Solid, []PosEntry, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	solid, err := a.frameLocked(ctx, timeMs)
	if err != nil {
		return nil, nil, err
	}
	return solid, buildPosMap(*a.e.solidTracks), nil
}

// frameLocked evaluates the model at timeMs and returns its Solid. The caller
// must hold a.mu; the per-frame solidTracks are left live for the caller to
// read (e.g. FrameWithPosMap) until the next frame truncates them.
func (a *Animation) frameLocked(ctx context.Context, timeMs float64) (*manifold.Solid, error) {
	// Install this call's context so a runaway frame (nested ranges, heavy
	// per-iteration geometry) is cancelable — the eval loop polls e.ctx.Err()
	// each iteration. Safe under a.mu: the lock serializes frame calls, and any
	// sub-evaluator spawned during the frame snapshots e.ctx at creation, so it
	// inherits this ctx. The build context that produced the handle is no
	// longer consulted at frame time, so a canceled build request can't kill
	// playback while a live per-frame ctx keeps each frame cancelable.
	a.e.ctx = ctx

	// Discard the previous frame's per-frame solidTracks (indices >= baseTracks);
	// invariant-setup tracks accumulated before the Animation was built are kept.
	// A plain reslice suffices: a track holds only a face-ID slice now (no
	// *Solid), so nothing pins per-frame C++ geometry, and truncation keeps the
	// per-frame posMap from accumulating stale entries across frames.
	*a.e.solidTracks = (*a.e.solidTracks)[:a.baseTracks]

	v, err := a.e.callFunctionVal(a.frame, map[string]value{a.argName: timeMs})
	if err != nil {
		return nil, err
	}
	solid, ok := v.(*manifold.Solid)
	if !ok {
		return nil, fmt.Errorf("animation frame returned %s, expected Solid", typeName(v))
	}
	return solid, nil
}
