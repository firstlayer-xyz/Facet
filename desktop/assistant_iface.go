package main

import (
	"context"
	"os/exec"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Assistant is one selectable AI backend for the assistant panel. An
// implementation owns its own process lifecycle and turns a user message into
// a stream of assistant:* events on the app event bus.
type Assistant interface {
	// Send dispatches one user turn. It returns once the turn has been handed
	// to the backend; streaming happens asynchronously via emitted events
	// (assistant:token/:tool-use/:thinking, terminated by :done or :error).
	// Implementations reject a Send while a turn is already in flight.
	Send(turn Turn, cfg SessionConfig) error
	// Interrupt stops the in-flight turn, if any. Claude keeps its process and
	// warm cache alive; one-shot CLIs kill the subprocess.
	Interrupt()
	// Reset discards conversation/session state so the next Send starts fresh.
	Reset()
	// Close terminates the backend and any child process.
	Close()
}

// Turn is the per-message input: the user's text plus the editor context the
// backend may inline (generic CLIs) or expose via MCP tools (Claude).
type Turn struct {
	UserMessage       string
	EditorCode        string
	ErrorsText        string
	ActiveTabPath     string
	ActiveTabReadOnly bool
	ImagePaths        []string
}

// SessionConfig is the resolved per-session configuration. SystemPrompt is
// already resolved by the coordinator (user override -> MCP-short -> full).
type SessionConfig struct {
	Model        string
	Effort       string
	MaxTurns     int
	SystemPrompt string
}

// EventEmitter decouples backends from Wails so they can be unit-tested with a
// recording fake.
type EventEmitter interface {
	Emit(event string, data ...any)
}

// wailsEmitter emits assistant:* events to the frontend over the Wails bus.
type wailsEmitter struct {
	ctx context.Context
}

func newWailsEmitter(ctx context.Context) *wailsEmitter {
	return &wailsEmitter{ctx: ctx}
}

func (e *wailsEmitter) Emit(event string, data ...any) {
	wailsRuntime.EventsEmit(e.ctx, event, data...)
}

// cmdFactory builds an exec.Cmd; overridable in tests.
type cmdFactory func(ctx context.Context, name string, args ...string) *exec.Cmd

func newExecCmd(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}
