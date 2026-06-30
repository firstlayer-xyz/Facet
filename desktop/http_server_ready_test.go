package main

import (
	"context"
	"testing"
	"time"
)

// TestWaitReadyBlocksUntilStart pins GetHTTPAuth's blocking contract: a waiter
// that arrives before Start binds the listener must not return until it does.
// Otherwise the frontend reads port 0, caches it, and every eval dead-ends with
// "Load failed" for the whole session.
func TestWaitReadyBlocksUntilStart(t *testing.T) {
	eval := NewEvalService()
	srv := NewHTTPServer(eval, NewMCPService(eval))

	returned := make(chan struct{})
	go func() {
		srv.WaitReady(context.Background())
		close(returned)
	}()

	// Before Start, WaitReady must stay blocked.
	select {
	case <-returned:
		t.Fatal("WaitReady returned before Start bound the listener")
	case <-time.After(150 * time.Millisecond):
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if port, _ := srv.Endpoint(); port == 0 {
		t.Fatal("Start bound port 0")
	}

	// After Start signals ready, the waiter must unblock promptly.
	select {
	case <-returned:
	case <-time.After(2 * time.Second):
		t.Fatal("WaitReady did not return after Start signalled ready")
	}
}
