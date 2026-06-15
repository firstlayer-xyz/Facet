package main

import (
	"context"
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
// only when the test binary is re-exec'd with GO_WANT_FAKE_CLAUDE=1. Args after
// "--" are: behavior, payload, then the real CLI argv (binary name excluded).
// Later tasks add "claude"/"tools" branches; Task 3 implements only "echo".
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_FAKE_CLAUDE") != "1" {
		return
	}
	rest := os.Args
	for i, a := range os.Args {
		if a == "--" {
			rest = os.Args[i+1:]
			break
		}
	}
	behavior := rest[0]
	payload := rest[1]
	// realArgs := rest[2:] // the CLI argv; used by later tasks' branches.

	switch behavior {
	case "echo":
		// generic-CLI fake: print the payload's lines verbatim, then exit.
		for _, line := range strings.Split(payload, "\n") {
			fmt.Fprintln(os.Stdout, line)
		}
		os.Exit(0)
	}
	os.Exit(0)
}
