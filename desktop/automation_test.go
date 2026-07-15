//go:build automation

package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// A fake frontend: capture the emitted invoke and resolve it out-of-band.
func TestInvokeRoundTrip(t *testing.T) {
	c := NewAutomationController()
	c.SetEventContext(context.Background())

	// Stub the emit so the test doesn't need a real Wails runtime.
	var got AutomationInvoke
	c.emit = func(_ context.Context, payload AutomationInvoke) {
		got = payload
		// Simulate the frontend acking on another goroutine.
		go func() {
			if err := c.resolve(payload.ID, `{"ok":true}`, ""); err != nil {
				t.Errorf("resolve: %v", err)
			}
		}()
	}

	out, err := c.Invoke(context.Background(), "viewer.setCamera", json.RawMessage(`{"azimuth":30}`))
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if got.Name != "viewer.setCamera" || string(got.Params) != `{"azimuth":30}` {
		t.Fatalf("emit payload wrong: %+v", got)
	}
	if string(out) != `{"ok":true}` {
		t.Fatalf("result = %s", out)
	}
}

func TestInvokeErrAck(t *testing.T) {
	c := NewAutomationController()
	c.SetEventContext(context.Background())
	c.emit = func(_ context.Context, p AutomationInvoke) {
		go c.resolve(p.ID, "", "no such panel")
	}
	_, err := c.Invoke(context.Background(), "ui.openPanel", json.RawMessage(`{"name":"nope"}`))
	if err == nil || err.Error() != "no such panel" {
		t.Fatalf("want error 'no such panel', got %v", err)
	}
}

func TestInvokeTimeout(t *testing.T) {
	c := NewAutomationController()
	c.SetEventContext(context.Background())
	c.emit = func(context.Context, AutomationInvoke) {} // frontend never acks
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := c.Invoke(ctx, "viewer.frameAll", nil)
	if err == nil {
		t.Fatal("want timeout error, got nil")
	}
}

func TestResolveUnknownID(t *testing.T) {
	c := NewAutomationController()
	if err := c.resolve("bogus", "{}", ""); err == nil {
		t.Fatal("want error for unknown id")
	}
}

func TestAppAutomationResultRoutes(t *testing.T) {
	a := NewApp()
	a.automation.SetEventContext(context.Background())
	a.automation.emit = func(_ context.Context, p AutomationInvoke) {
		go func() { _ = a.AutomationResult(p.ID, `42`, "") }()
	}
	out, err := a.automation.Invoke(context.Background(), "wait.ms", nil)
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if string(out) != `42` {
		t.Fatalf("out = %s", out)
	}
}
