package main

import (
	"context"
	"testing"
)

// TestAppContextPublicationRace exercises the atomic publication of App.ctx: a
// concurrent Store (startup) and Load (a binding goroutine) must be race-free
// under `go test -race`. Before atomic.Pointer this was a torn two-word access.
func TestAppContextPublicationRace(t *testing.T) {
	a := &App{}
	ctx := context.Background()
	done := make(chan struct{})
	go func() {
		a.ctx.Store(&ctx)
		close(done)
	}()
	_ = a.runtimeCtx()
	<-done
	if a.runtimeCtx() != ctx {
		t.Fatal("runtimeCtx did not return the stored context")
	}
}

// TestAssistantEventContextRace exercises the mutex-guarded publication of
// eventCtx: SetEventContext (startup) racing a mu-guarded read (as Send does)
// must be clean under -race. Before the lock the write had no happens-before
// edge with the reader.
func TestAssistantEventContextRace(t *testing.T) {
	s := NewAssistantService()
	done := make(chan struct{})
	go func() {
		s.SetEventContext(context.Background())
		close(done)
	}()
	s.mu.Lock()
	_ = s.eventCtx
	s.mu.Unlock()
	<-done
}
