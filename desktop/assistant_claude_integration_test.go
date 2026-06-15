package main

import (
	"os"
	"testing"
	"time"
)

// waitForEventLong is waitForEvent with a generous deadline for the real CLI,
// where a turn (including first-turn cache creation) can take several seconds.
func waitForEventLong(t *testing.T, r *recordingEmitter, name string, d time.Duration) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		for _, n := range r.names() {
			if n == name {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("event %q never emitted within %s; got %v", name, d, r.names())
}

// TestClaudeAssistantRealCLI drives the REAL claude binary to prove the core
// claim end-to-end: two turns are served by ONE persistent process (the warm
// prompt cache the whole migration depends on). Gated — costs real usage.
//
//	FACET_REAL_CLAUDE=1 ./.go-toolchain/bin/go test ./desktop/ -run TestClaudeAssistantRealCLI -v
func TestClaudeAssistantRealCLI(t *testing.T) {
	if os.Getenv("FACET_REAL_CLAUDE") != "1" {
		t.Skip("set FACET_REAL_CLAUDE=1 to run against the real claude binary")
	}
	bin := findBinary("claude")
	if bin == "" {
		t.Skip("claude not installed")
	}

	rec := &recordingEmitter{}
	ca := newClaudeAssistant(rec, nil, bin)
	defer ca.Close()

	cfg := SessionConfig{MaxTurns: 1, SystemPrompt: "You are a terse test assistant. Reply with a single word."}

	if err := ca.Send(Turn{UserMessage: "Reply with exactly: PONG"}, cfg); err != nil {
		t.Fatalf("Send turn 1: %v", err)
	}
	waitForEventLong(t, rec, "assistant:done", 60*time.Second)
	pid1 := ca.pidForTest()
	if pid1 == 0 {
		t.Fatalf("no live process after turn 1")
	}

	rec.reset()
	if err := ca.Send(Turn{UserMessage: "Reply with exactly: PING"}, cfg); err != nil {
		t.Fatalf("Send turn 2: %v", err)
	}
	waitForEventLong(t, rec, "assistant:done", 60*time.Second)

	if pid2 := ca.pidForTest(); pid2 != pid1 {
		t.Fatalf("real CLI did not reuse the process across turns: pid1=%d pid2=%d", pid1, pid2)
	}
}
