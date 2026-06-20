package evaluator

import (
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
// renders a single frame at timeMs (a static snapshot); otherwise it returns
// the entry's solids unchanged. This keeps non-playback callers from silently
// emitting an empty model when the entry is an Animation.
func (r *EvalResult) StaticSolids(timeMs float64) ([]*manifold.Solid, error) {
	if r.Animation != nil {
		s, err := r.Animation.Frame(timeMs)
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
// closure must not call back into Frame on the same handle.
func (a *Animation) Frame(timeMs float64) (*manifold.Solid, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.frameLocked(timeMs)
}

// FrameWithPosMap evaluates the frame and also returns its source-position →
// face-ID map, so interactive playback supports face-click → source navigation
// exactly as the static render does. The PosMap is built from the live per-frame
// solidTracks under the same lock, before the next Frame truncates them.
func (a *Animation) FrameWithPosMap(timeMs float64) (*manifold.Solid, []PosEntry, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	solid, err := a.frameLocked(timeMs)
	if err != nil {
		return nil, nil, err
	}
	return solid, buildPosMap(*a.e.solidTracks), nil
}

// frameLocked evaluates the model at timeMs and returns its Solid. The caller
// must hold a.mu; the per-frame solidTracks are left live for the caller to
// read (e.g. FrameWithPosMap) until the next frame truncates them.
func (a *Animation) frameLocked(timeMs float64) (*manifold.Solid, error) {
	// Discard the previous frame's solidTracks so their C++ geometry is released.
	// Invariant-setup tracks (accumulated before the Animation was built) are
	// preserved; only per-frame tracks (indices >= baseTracks) are pruned.
	// Zero the trailing slots before reslicing: reslicing alone leaves the
	// per-frame *Solid pointers reachable in the backing array, so their
	// finalizer-driven C++ release would never fire (a native-memory leak that
	// grows whenever a later frame produces fewer tracks than an earlier one).
	tracks := *a.e.solidTracks
	for i := a.baseTracks; i < len(tracks); i++ {
		tracks[i] = SolidTrack{}
	}
	*a.e.solidTracks = tracks[:a.baseTracks]

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
