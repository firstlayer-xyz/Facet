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
	svc := NewMCPService(NewEvalService())

	returned := make(chan struct{})
	go func() {
		svc.WaitReady(context.Background())
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
	port, _, err := svc.Start(ctx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if port == 0 {
		t.Fatal("Start bound port 0")
	}

	// After Start signals ready, the waiter must unblock promptly.
	select {
	case <-returned:
	case <-time.After(2 * time.Second):
		t.Fatal("WaitReady did not return after Start signalled ready")
	}
}
