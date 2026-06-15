package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestBuildUserFrameTextOnly(t *testing.T) {
	frame, err := buildUserFrame("hello world", nil)
	if err != nil {
		t.Fatalf("buildUserFrame: %v", err)
	}
	var msg struct {
		Type    string `json:"type"`
		Message struct {
			Role    string `json:"role"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(frame, &msg); err != nil {
		t.Fatalf("frame is not valid JSON: %v\n%s", err, frame)
	}
	if msg.Type != "user" || msg.Message.Role != "user" {
		t.Fatalf("bad envelope: %+v", msg)
	}
	if len(msg.Message.Content) != 1 || msg.Message.Content[0].Type != "text" || msg.Message.Content[0].Text != "hello world" {
		t.Fatalf("bad content: %+v", msg.Message.Content)
	}
	if bytes.ContainsRune(frame, '\n') {
		t.Fatalf("frame must be single-line NDJSON; got newline in %s", frame)
	}
}

func TestBuildUserFrameWithImage(t *testing.T) {
	dir := t.TempDir()
	png := filepath.Join(dir, "x.png")
	raw, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==")
	if err := os.WriteFile(png, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	frame, err := buildUserFrame("what is this", []string{png})
	if err != nil {
		t.Fatalf("buildUserFrame: %v", err)
	}
	if !bytes.Contains(frame, []byte(`"type":"image"`)) ||
		!bytes.Contains(frame, []byte(`"media_type":"image/png"`)) ||
		!bytes.Contains(frame, []byte(`"type":"base64"`)) {
		t.Fatalf("image block missing/wrong: %s", frame)
	}
}

func TestClaudeAssistantSingleTurn(t *testing.T) {
	rec := &recordingEmitter{}
	ca := newClaudeAssistant(rec, nil, fakeBinPath())
	ca.newCmd = fakeCmdFactory(t, "claude", "")
	if err := ca.Send(Turn{UserMessage: "hi"}, SessionConfig{MaxTurns: 2}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	waitForEvent(t, rec, "assistant:done")
	var gotToken string
	rec.mu.Lock()
	for _, e := range rec.events {
		if e[0] == "assistant:token" {
			gotToken = e[1].(string)
		}
	}
	rec.mu.Unlock()
	if gotToken != "echo: hi" {
		t.Fatalf("token = %q, want %q (events: %v)", gotToken, "echo: hi", rec.names())
	}
	ca.Close()
}

func TestClaudeAssistantEmitsToolUseAndThinking(t *testing.T) {
	rec := &recordingEmitter{}
	ca := newClaudeAssistant(rec, nil, fakeBinPath())
	ca.newCmd = fakeCmdFactory(t, "tools", "")
	if err := ca.Send(Turn{UserMessage: "edit"}, SessionConfig{MaxTurns: 5}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	waitForEvent(t, rec, "assistant:done")
	for _, want := range []string{"assistant:tool-use", "assistant:thinking"} {
		found := false
		for _, n := range rec.names() {
			if n == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing %s; got %v", want, rec.names())
		}
	}
	ca.Close()
}

func TestClaudeAssistantSurfacesResultError(t *testing.T) {
	rec := &recordingEmitter{}
	ca := newClaudeAssistant(rec, nil, fakeBinPath())
	ca.newCmd = fakeCmdFactory(t, "error", "")
	if err := ca.Send(Turn{UserMessage: "go"}, SessionConfig{MaxTurns: 3}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	waitForEvent(t, rec, "assistant:error")
	// An is_error result must NOT also emit assistant:done.
	for _, n := range rec.names() {
		if n == "assistant:done" {
			t.Fatalf("is_error result should not emit done; got %v", rec.names())
		}
	}
	ca.Close()
}

// mustTurn sends one turn and waits for it to finish, clearing the recorder so
// the next turn's events are isolated.
func mustTurn(t *testing.T, ca *claudeAssistant, rec *recordingEmitter, msg string, cfg SessionConfig) {
	t.Helper()
	rec.reset()
	if err := ca.Send(Turn{UserMessage: msg}, cfg); err != nil {
		t.Fatalf("Send(%q): %v", msg, err)
	}
	waitForEvent(t, rec, "assistant:done")
}

func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

func TestClaudeAssistantReusesProcessAcrossTurns(t *testing.T) {
	rec := &recordingEmitter{}
	ca := newClaudeAssistant(rec, nil, fakeBinPath())
	ca.newCmd = fakeCmdFactory(t, "claude", "")

	mustTurn(t, ca, rec, "one", SessionConfig{MaxTurns: 2})
	pid1 := ca.pidForTest()
	mustTurn(t, ca, rec, "two", SessionConfig{MaxTurns: 2})
	pid2 := ca.pidForTest()
	if pid1 == 0 || pid1 != pid2 {
		t.Fatalf("expected same process across turns; pid1=%d pid2=%d", pid1, pid2)
	}
	ca.Close()
}

func TestClaudeAssistantRespawnsOnConfigChange(t *testing.T) {
	rec := &recordingEmitter{}
	ca := newClaudeAssistant(rec, nil, fakeBinPath())
	var lastArgs []string
	ca.newCmd = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		lastArgs = args
		full := append([]string{"-test.run=TestHelperProcess", "--", "claude", ""}, args...)
		cmd := exec.CommandContext(ctx, os.Args[0], full...)
		cmd.Env = append(os.Environ(), "GO_WANT_FAKE_CLAUDE=1")
		return cmd
	}

	mustTurn(t, ca, rec, "one", SessionConfig{Model: "opus", MaxTurns: 2})
	pid1 := ca.pidForTest()
	if !hasFlag(lastArgs, "--session-id") {
		t.Fatalf("first launch should use --session-id; args=%v", lastArgs)
	}
	mustTurn(t, ca, rec, "two", SessionConfig{Model: "sonnet", MaxTurns: 2})
	pid2 := ca.pidForTest()
	if pid1 == pid2 {
		t.Fatalf("model change should respawn; pid stayed %d", pid1)
	}
	if !hasFlag(lastArgs, "--resume") {
		t.Fatalf("respawn should use --resume to preserve the conversation; args=%v", lastArgs)
	}
	ca.Close()
}

func TestClaudeInterruptIsBenignAndKeepsProcess(t *testing.T) {
	rec := &recordingEmitter{}
	ca := newClaudeAssistant(rec, nil, fakeBinPath())
	ca.newCmd = fakeCmdFactory(t, "interruptible", "")

	// Start a turn that hangs in-flight (fake emits "working..." and waits).
	if err := ca.Send(Turn{UserMessage: "WAIT"}, SessionConfig{MaxTurns: 2}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	waitForEvent(t, rec, "assistant:token") // "working..." -> turn is in-flight
	pid1 := ca.pidForTest()

	ca.Interrupt()
	waitForEvent(t, rec, "assistant:done") // benign stop, NOT an error

	for _, n := range rec.names() {
		if n == "assistant:error" {
			t.Fatalf("interrupt must be benign (done, not error); got %v", rec.names())
		}
	}
	if pid2 := ca.pidForTest(); pid2 != pid1 || pid1 == 0 {
		t.Fatalf("interrupt must keep the same process alive; pid1=%d pid2=%d", pid1, pid2)
	}
	// The process is still usable for a normal follow-up turn.
	mustTurn(t, ca, rec, "hello", SessionConfig{MaxTurns: 2})
	ca.Close()
}

func TestClaudeInterruptWhileIdleIsNoop(t *testing.T) {
	rec := &recordingEmitter{}
	ca := newClaudeAssistant(rec, nil, fakeBinPath())
	ca.newCmd = fakeCmdFactory(t, "claude", "")
	ca.Interrupt() // no process yet — must not panic
	mustTurn(t, ca, rec, "one", SessionConfig{MaxTurns: 2})
	ca.Interrupt() // idle (turn done) — must be a no-op, not emit anything
	ca.Close()
}

func TestClaudeAssistantRecoversFromCrash(t *testing.T) {
	rec := &recordingEmitter{}
	ca := newClaudeAssistant(rec, nil, fakeBinPath())
	ca.newCmd = fakeCmdFactory(t, "claude", "")

	mustTurn(t, ca, rec, "one", SessionConfig{MaxTurns: 2})
	rec.reset()
	if err := ca.Send(Turn{UserMessage: "CRASH"}, SessionConfig{MaxTurns: 2}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	waitForEvent(t, rec, "assistant:error") // crash surfaced
	// Next turn lazily respawns (with --resume) and works.
	mustTurn(t, ca, rec, "again", SessionConfig{MaxTurns: 2})
	ca.Close()
}
