package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestWaitReadyBlocksUntilStart pins GetHTTPAuth's blocking contract: a waiter
// that arrives before Start binds the listener must not return until it does.
// Otherwise the frontend reads port 0, caches it, and every eval dead-ends with
// "Load failed" for the whole session.
func TestWaitReadyBlocksUntilStart(t *testing.T) {
	eval := NewEvalService()
	srv := NewHTTPServer(eval, NewMCPService(eval, NewAutomationController()), NewAutomationController())

	returned := make(chan error, 1)
	go func() {
		returned <- srv.WaitReady()
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

	// After Start signals ready, the waiter must unblock promptly with nil (bound).
	select {
	case err := <-returned:
		if err != nil {
			t.Fatalf("WaitReady returned an error after a successful Start: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("WaitReady did not return after Start signalled ready")
	}
}

// TestWaitReadyReturnsStartError verifies the failure path: when Start finishes
// with an error, WaitReady returns it immediately (ready is already closed)
// instead of hanging for 10s and then handing back zero auth. GetHTTPAuth turns
// this into a surfaced error rather than pinning port 0 for the session.
func TestWaitReadyReturnsStartError(t *testing.T) {
	srv := &HTTPServer{ready: make(chan struct{})}
	srv.startErr = fmt.Errorf("listen failed")
	close(srv.ready)

	done := make(chan error, 1)
	go func() { done <- srv.WaitReady() }()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "listen failed") {
			t.Fatalf("WaitReady should return the start error, got: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("WaitReady hung on a failed Start instead of returning immediately")
	}
}
