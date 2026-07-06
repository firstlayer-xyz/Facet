package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fakeMCPBridge is a minimal AssistantMCPBridge that reports a fixed endpoint,
// used to drive the MCP-enabled launch path in tests.
type fakeMCPBridge struct {
	port  int
	token string
}

func (f fakeMCPBridge) Endpoint() (int, string)                        { return f.port, f.token }
func (f fakeMCPBridge) SetContext(code, activeTabPath string, ro bool) {}

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
		return exec.CommandContext(ctx, os.Args[0], full...)
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

func TestClaudeAssistantSurfacesOversizeLine(t *testing.T) {
	rec := &recordingEmitter{}
	ca := newClaudeAssistant(rec, nil, fakeBinPath())
	ca.maxLineBytes = 512 // fake emits a 4096-byte line -> scanner overflow
	ca.newCmd = fakeCmdFactory(t, "hugeline", "")
	if err := ca.Send(Turn{UserMessage: "go"}, SessionConfig{MaxTurns: 1}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	// Must terminate with an error rather than hang forever on cmd.Wait.
	waitForEvent(t, rec, "assistant:error")
	ca.Close()
}

// TestClaudeMCPTokenNotInArgv guards the fix for the argv token leak: with MCP
// enabled the auth token must never appear on the child's command line (which
// any local process can read via ps//proc/cmdline). It must instead be passed
// as a --mcp-config file path.
func TestClaudeMCPTokenNotInArgv(t *testing.T) {
	const token = "s3cr3t-mcp-token"
	rec := &recordingEmitter{}
	ca := newClaudeAssistant(rec, fakeMCPBridge{port: 12345, token: token}, fakeBinPath())

	var capturedArgs []string
	ca.newCmd = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		capturedArgs = args
		full := append([]string{"-test.run=TestHelperProcess", "--", "claude", ""}, args...)
		return exec.CommandContext(ctx, os.Args[0], full...)
	}
	if err := ca.Send(Turn{UserMessage: "hi"}, SessionConfig{MaxTurns: 2}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	waitForEvent(t, rec, "assistant:done")
	ca.Close()

	for _, a := range capturedArgs {
		if strings.Contains(a, token) {
			t.Fatalf("token leaked into argv: %q", a)
		}
	}
	// The config must be handed over by path, not inlined as JSON.
	cfgVal := argValue(capturedArgs, "--mcp-config")
	if cfgVal == "" {
		t.Fatalf("no --mcp-config in args: %v", capturedArgs)
	}
	if strings.HasPrefix(strings.TrimSpace(cfgVal), "{") {
		t.Fatalf("--mcp-config is inline JSON, expected a file path: %q", cfgVal)
	}
}

// TestWriteMCPConfig verifies the config file is private (0600) and carries the
// token and endpoint, so passing it by path is a faithful substitute for the
// inline config.
func TestWriteMCPConfig(t *testing.T) {
	const token = "unit-token-abc"
	path, err := writeMCPConfig(45321, token)
	if err != nil {
		t.Fatalf("writeMCPConfig: %v", err)
	}
	defer os.Remove(path)

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("config perms = %o, want 0600 (token must be owner-only)", perm)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var cfg struct {
		MCPServers map[string]struct {
			URL     string            `json:"url"`
			Headers map[string]string `json:"headers"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		t.Fatalf("config is not valid JSON: %v\n%s", err, b)
	}
	facet, ok := cfg.MCPServers["facet"]
	if !ok {
		t.Fatalf("no facet server in config: %s", b)
	}
	if got, want := facet.Headers["Authorization"], "Bearer "+token; got != want {
		t.Errorf("Authorization = %q, want %q", got, want)
	}
	if !strings.Contains(facet.URL, "127.0.0.1:45321") {
		t.Errorf("url = %q, want it to target 127.0.0.1:45321", facet.URL)
	}
}
