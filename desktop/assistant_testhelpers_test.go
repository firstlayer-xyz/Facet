package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

// recordingEmitter captures emitted events for assertions.
type recordingEmitter struct {
	mu     sync.Mutex
	events [][]any
}

func (r *recordingEmitter) Emit(event string, data ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, append([]any{event}, data...))
}

func (r *recordingEmitter) names() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, e := range r.events {
		out = append(out, e[0].(string))
	}
	return out
}

func (r *recordingEmitter) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = nil
}

// waitForEvent blocks until an event with the given name has been emitted, or
// fails the test after 5s.
func waitForEvent(t *testing.T, r *recordingEmitter, name string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, n := range r.names() {
			if n == name {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("event %q never emitted; got %v", name, r.names())
}

func fakeBinPath() string { return os.Args[0] }

// fakeCmdFactory returns a cmdFactory that re-execs the test binary as a fake
// CLI. behavior selects which branch TestHelperProcess takes; payload is passed
// to that branch. The real binary name (first arg to the factory) is dropped —
// the fake is the test binary, not the real CLI.
func fakeCmdFactory(t *testing.T, behavior, payload string) cmdFactory {
	t.Helper()
	return func(ctx context.Context, name string, args ...string) *exec.Cmd {
		full := append([]string{"-test.run=TestHelperProcess", "--", behavior, payload}, args...)
		cmd := exec.CommandContext(ctx, os.Args[0], full...)
		cmd.Env = append(os.Environ(), "GO_WANT_FAKE_CLAUDE=1")
		return cmd
	}
}

// TestHelperProcess is not a real test; it is the fake CLI process body, invoked
// only when the test binary is re-exec'd with a "--" sentinel in its argv. Args
// after "--" are: behavior, payload, then the real CLI argv (binary name
// excluded). The "echo" branch fakes a generic one-shot CLI; the "claude" branch
// fakes a persistent stream-json claude process.
func TestHelperProcess(t *testing.T) {
	rest := fakeProcessArgs()
	if rest == nil {
		return // not a re-exec'd fake invocation
	}
	behavior := rest[0]
	payload := rest[1]
	realArgs := rest[2:]

	switch behavior {
	case "echo":
		// generic-CLI fake: print the payload's lines verbatim, then exit.
		for _, line := range strings.Split(payload, "\n") {
			fmt.Fprintln(os.Stdout, line)
		}
		os.Exit(0)
	case "claude":
		sessionID := argValue(realArgs, "--session-id")
		if sessionID == "" {
			sessionID = argValue(realArgs, "--resume")
		}
		if sessionID == "" {
			sessionID = "fake-session"
		}
		out := bufio.NewWriter(os.Stdout)
		emit := func(v any) {
			b, _ := json.Marshal(v)
			out.Write(b)
			out.WriteByte('\n')
			out.Flush()
		}
		emit(map[string]any{"type": "system", "subtype": "init", "session_id": sessionID})
		sc := bufio.NewScanner(os.Stdin)
		sc.Buffer(make([]byte, 0, 1<<20), 64<<20)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" {
				continue
			}
			var in map[string]any
			if json.Unmarshal([]byte(line), &in) != nil {
				continue
			}
			if in["type"] == "control_request" {
				emit(map[string]any{"type": "control_response", "response": map[string]any{"subtype": "success"}})
				continue
			}
			text := firstUserText(in)
			if text == "CRASH" {
				os.Exit(1)
			}
			emit(map[string]any{"type": "assistant", "message": map[string]any{
				"role": "assistant", "content": []any{map[string]any{"type": "text", "text": "echo: " + text}},
			}})
			emit(map[string]any{"type": "result", "subtype": "success", "is_error": false,
				"result": "echo: " + text, "session_id": sessionID})
		}
		os.Exit(0)
	case "tools":
		sessionID := argValue(realArgs, "--session-id")
		if sessionID == "" {
			sessionID = argValue(realArgs, "--resume")
		}
		if sessionID == "" {
			sessionID = "fake-session"
		}
		out := bufio.NewWriter(os.Stdout)
		emit := func(v any) {
			b, _ := json.Marshal(v)
			out.Write(b)
			out.WriteByte('\n')
			out.Flush()
		}
		emit(map[string]any{"type": "system", "subtype": "init", "session_id": sessionID})
		sc := bufio.NewScanner(os.Stdin)
		sc.Buffer(make([]byte, 0, 1<<20), 64<<20)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" {
				continue
			}
			var in map[string]any
			if json.Unmarshal([]byte(line), &in) != nil {
				continue
			}
			if in["type"] == "control_request" {
				emit(map[string]any{"type": "control_response", "response": map[string]any{"subtype": "success"}})
				continue
			}
			// One tool round-trip, then a text answer, then the result.
			emit(map[string]any{"type": "assistant", "message": map[string]any{
				"role": "assistant", "content": []any{map[string]any{"type": "tool_use", "id": "t1", "name": "get_editor_code", "input": map[string]any{}}},
			}})
			emit(map[string]any{"type": "user", "message": map[string]any{
				"role": "user", "content": []any{map[string]any{"type": "tool_result", "tool_use_id": "t1", "content": "code"}},
			}})
			emit(map[string]any{"type": "assistant", "message": map[string]any{
				"role": "assistant", "content": []any{map[string]any{"type": "text", "text": "done"}},
			}})
			emit(map[string]any{"type": "result", "subtype": "success", "is_error": false, "result": "done", "session_id": sessionID})
		}
		os.Exit(0)
	case "error":
		out := bufio.NewWriter(os.Stdout)
		emit := func(v any) {
			b, _ := json.Marshal(v)
			out.Write(b)
			out.WriteByte('\n')
			out.Flush()
		}
		emit(map[string]any{"type": "system", "subtype": "init", "session_id": "fake-session"})
		sc := bufio.NewScanner(os.Stdin)
		sc.Buffer(make([]byte, 0, 1<<20), 64<<20)
		for sc.Scan() {
			if strings.TrimSpace(sc.Text()) == "" {
				continue
			}
			emit(map[string]any{"type": "result", "subtype": "error_max_turns", "is_error": true, "session_id": "fake-session"})
		}
		os.Exit(0)
	}
	os.Exit(0)
}

// fakeProcessArgs returns the args after the "--" sentinel when the test binary
// was re-exec'd as a fake CLI, or nil for a normal test run. A normal `go test`
// invocation never contains a bare "--".
func fakeProcessArgs() []string {
	for i, a := range os.Args {
		if a == "--" {
			return os.Args[i+1:]
		}
	}
	return nil
}

func argValue(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func firstUserText(in map[string]any) string {
	msg, _ := in["message"].(map[string]any)
	content, _ := msg["content"].([]any)
	for _, b := range content {
		cb, _ := b.(map[string]any)
		if cb["type"] == "text" {
			if s, ok := cb["text"].(string); ok {
				return s
			}
		}
	}
	return ""
}
