package main

import (
	"context"
	"testing"
)

// fakeAssistant is a no-op Assistant that records delegated calls.
type fakeAssistant struct {
	sends      int
	interrupts int
	resets     int
	closes     int
	lastTurn   Turn
	lastCfg    SessionConfig
}

func (f *fakeAssistant) Send(turn Turn, cfg SessionConfig) error {
	f.sends++
	f.lastTurn = turn
	f.lastCfg = cfg
	return nil
}
func (f *fakeAssistant) Interrupt() { f.interrupts++ }
func (f *fakeAssistant) Reset()     { f.resets++ }
func (f *fakeAssistant) Close()     { f.closes++ }

func TestServiceConstructsOnceAndDelegatesSend(t *testing.T) {
	s := NewAssistantService()
	s.SetEventContext(context.Background())
	fake := &fakeAssistant{}
	builds := 0
	s.newAssistant = func(cliID, binPath string, emit EventEmitter, mcp AssistantMCPBridge) Assistant {
		builds++
		return fake
	}
	s.resolveBinary = func(string) string { return "/fake/bin" }
	s.SetConfig(AssistantConfig{CLI: "claude", MaxTurns: 0})

	if err := s.Send("hi", "code", "", "/a.fct", false, nil, nil); err != nil {
		t.Fatalf("Send 1: %v", err)
	}
	if err := s.Send("again", "code", "", "/a.fct", false, nil, nil); err != nil {
		t.Fatalf("Send 2: %v", err)
	}

	if builds != 1 {
		t.Fatalf("backend should be constructed once and reused; got %d builds", builds)
	}
	if fake.sends != 2 {
		t.Fatalf("expected 2 delegated sends; got %d", fake.sends)
	}
	if fake.lastCfg.MaxTurns != 10 {
		t.Fatalf("MaxTurns 0 should resolve to default 10; got %d", fake.lastCfg.MaxTurns)
	}
	if fake.lastTurn.UserMessage != "again" {
		t.Fatalf("turn not threaded through; got %q", fake.lastTurn.UserMessage)
	}
}

func TestServiceCancelAndClearDelegate(t *testing.T) {
	s := NewAssistantService()
	s.SetEventContext(context.Background())
	fake := &fakeAssistant{}
	s.newAssistant = func(cliID, binPath string, emit EventEmitter, mcp AssistantMCPBridge) Assistant { return fake }
	s.resolveBinary = func(string) string { return "/fake/bin" }
	s.SetConfig(AssistantConfig{CLI: "claude"})
	_ = s.Send("hi", "", "", "/a.fct", false, nil, nil)

	s.Cancel()
	s.ClearHistory()
	if fake.interrupts != 1 || fake.resets != 1 {
		t.Fatalf("Cancel/ClearHistory not delegated: interrupts=%d resets=%d", fake.interrupts, fake.resets)
	}
}

func TestServiceShutdownClosesAndRebuilds(t *testing.T) {
	s := NewAssistantService()
	s.SetEventContext(context.Background())
	builds := 0
	s.newAssistant = func(cliID, binPath string, emit EventEmitter, mcp AssistantMCPBridge) Assistant {
		builds++
		return &fakeAssistant{}
	}
	s.resolveBinary = func(string) string { return "/fake/bin" }
	s.SetConfig(AssistantConfig{CLI: "claude"})

	_ = s.Send("hi", "", "", "/a.fct", false, nil, nil)
	first := s.current.(*fakeAssistant)

	s.Shutdown()
	if first.closes != 1 {
		t.Fatalf("Shutdown should Close the live backend; closes=%d", first.closes)
	}
	if s.current != nil {
		t.Fatalf("Shutdown should drop the current backend")
	}

	// A send after shutdown rebuilds a fresh backend.
	_ = s.Send("again", "", "", "/a.fct", false, nil, nil)
	if builds != 2 {
		t.Fatalf("Send after Shutdown should reconstruct the backend; builds=%d", builds)
	}
}
