package main

import (
	"context"
	"testing"
)

// newPermTestService builds a bare MCPService with just the permission maps
// initialised — mirrors the bare-service pattern in mcp_question_test.go.
func newPermTestService() *MCPService {
	return &MCPService{
		permissions: make(map[string]chan permissionDecision),
		remembered:  make(map[string]struct{}),
	}
}

func TestAnswerPermissionRoutesToPendingChannel(t *testing.T) {
	m := newPermTestService()
	ch := make(chan permissionDecision, 1)
	m.permissions["p1"] = ch

	if err := m.AnswerPermission("p1", true, true); err != nil {
		t.Fatalf("AnswerPermission returned %v, want nil", err)
	}
	select {
	case d := <-ch:
		if !d.Allow || !d.Remember {
			t.Fatalf("got %+v, want Allow=true Remember=true", d)
		}
	default:
		t.Fatal("channel had no value — AnswerPermission failed to deliver")
	}
}

func TestAnswerPermissionUnknownIDErrors(t *testing.T) {
	m := newPermTestService()
	if err := m.AnswerPermission("nope", true, false); err == nil {
		t.Fatal("expected error for unknown id, got nil")
	}
}

func TestAnswerPermissionDoubleSendDrops(t *testing.T) {
	m := newPermTestService()
	m.permissions["p2"] = make(chan permissionDecision, 1)
	if err := m.AnswerPermission("p2", true, false); err != nil {
		t.Fatalf("first AnswerPermission: %v", err)
	}
	if err := m.AnswerPermission("p2", false, false); err == nil {
		t.Fatal("expected second AnswerPermission to error (channel full), got nil")
	}
}

func TestRequestPermissionRememberedShortCircuits(t *testing.T) {
	m := newPermTestService()
	m.remembered["WebSearch"] = struct{}{}
	d := m.requestPermission(context.Background(), "WebSearch", "Search the web", "WebSearch")
	if !d.Allow {
		t.Fatal("remembered key should auto-allow without prompting")
	}
}

func TestRequestPermissionCancelledCtxDenies(t *testing.T) {
	m := newPermTestService()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	d := m.requestPermission(ctx, "WebSearch", "Search the web", "WebSearch")
	if d.Allow {
		t.Fatal("cancelled ctx must deny")
	}
}

func TestClearRememberedPermissions(t *testing.T) {
	m := newPermTestService()
	m.remembered["WebSearch"] = struct{}{}
	m.ClearRememberedPermissions()
	if len(m.remembered) != 0 {
		t.Fatalf("remembered not cleared: %d entries", len(m.remembered))
	}
}
