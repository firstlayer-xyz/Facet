package manifold

import (
	"strings"
	"testing"
)

// The bridge functions must panic loudly when invoked with an unregistered id.
// Returning a neutral value silently (the previous behavior) corrupted geometry
// without surfacing any error — a programmer bug must crash, not produce
// garbage output.  We test the shared lookup helpers because //export bridges
// can't be called from a cgo test file.

func TestLookupWarpLockedPanicsOnUnknownID(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for unregistered warp id, got none")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("expected string panic, got %T: %v", r, r)
		}
		if !strings.Contains(msg, "facetWarpBridge") || !strings.Contains(msg, "no callback") {
			t.Errorf("panic message should identify the bridge and cause: %q", msg)
		}
	}()
	warpMu.Lock()
	defer warpMu.Unlock()
	lookupWarpLocked(-99999)
}

func TestLookupLevelSetLockedPanicsOnUnknownID(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for unregistered level-set id, got none")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("expected string panic, got %T: %v", r, r)
		}
		if !strings.Contains(msg, "facetLevelSetBridge") || !strings.Contains(msg, "no callback") {
			t.Errorf("panic message should identify the bridge and cause: %q", msg)
		}
	}()
	levelSetMu.Lock()
	defer levelSetMu.Unlock()
	lookupLevelSetLocked(-99999)
}

// Register-then-lookup happy path: confirms the helper returns the registered
// function and the mutex handoff with the register API works.
func TestLookupWarpLockedReturnsRegistered(t *testing.T) {
	id := registerWarp(func(x, y, z float64) (float64, float64, float64) {
		return x + 1, y + 2, z + 3
	})
	defer unregisterWarp(id)

	warpMu.Lock()
	defer warpMu.Unlock()
	fn := lookupWarpLocked(id)
	nx, ny, nz := fn(10, 20, 30)
	if nx != 11 || ny != 22 || nz != 33 {
		t.Errorf("registered fn returned (%v,%v,%v), want (11,22,33)", nx, ny, nz)
	}
}

func TestLookupLevelSetLockedReturnsRegistered(t *testing.T) {
	id := registerLevelSet(func(x, y, z float64) float64 {
		return x*x + y*y + z*z
	})
	defer unregisterLevelSet(id)

	levelSetMu.Lock()
	defer levelSetMu.Unlock()
	fn := lookupLevelSetLocked(id)
	if got := fn(1, 2, 2); got != 9 {
		t.Errorf("registered fn returned %v, want 9", got)
	}
}
