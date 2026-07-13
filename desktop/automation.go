package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// automationInvokeTimeout bounds a single GUI command so a hung frontend
// handler surfaces as an error instead of blocking the caller forever.
const automationInvokeTimeout = 60 * time.Second

// automationResult carries a command's outcome from the frontend back to the
// parked Invoke caller: Value is the JSON the command returned (may be null),
// Err is a non-empty message when the command failed.
type automationResult struct {
	Value json.RawMessage `json:"value"`
	Err   string          `json:"error"`
}

// AutomationInvoke is the payload emitted to the frontend for one command.
type AutomationInvoke struct {
	ID     string          `json:"id"`
	Name   string          `json:"name"`
	Params json.RawMessage `json:"params"`
}

// AutomationController is the single choke point both front doors (MCP tools,
// /control) call. Invoke assigns an id, emits an automation:invoke event, and
// blocks until the frontend calls App.AutomationResult (routed to resolve) or a
// timeout fires. It reuses the package-level takeResolution/awaitResolution
// generics so its concurrency semantics match the assistant's screenshot flow.
type AutomationController struct {
	eventCtx context.Context

	seq     atomic.Uint64
	pendMu  sync.Mutex
	pending map[string]chan automationResult

	// emit is the event sender, swappable in tests. Defaults to Wails.
	emit func(ctx context.Context, payload AutomationInvoke)
}

// NewAutomationController returns a controller wired to emit Wails events.
func NewAutomationController() *AutomationController {
	c := &AutomationController{pending: make(map[string]chan automationResult)}
	c.emit = func(ctx context.Context, payload AutomationInvoke) {
		wailsRuntime.EventsEmit(ctx, "automation:invoke", payload)
	}
	return c
}

// SetEventContext publishes the Wails runtime context used to emit events.
func (c *AutomationController) SetEventContext(ctx context.Context) { c.eventCtx = ctx }

// Invoke drives one GUI command and returns the JSON the frontend produced.
func (c *AutomationController) Invoke(ctx context.Context, name string, params json.RawMessage) (json.RawMessage, error) {
	if c.eventCtx == nil {
		return nil, fmt.Errorf("automation: no event context (app not started)")
	}
	id := fmt.Sprintf("auto-%d", c.seq.Add(1))
	ch := make(chan automationResult, 1)

	c.pendMu.Lock()
	c.pending[id] = ch
	c.pendMu.Unlock()

	c.emit(c.eventCtx, AutomationInvoke{ID: id, Name: name, Params: params})

	waitCtx, cancel := context.WithTimeout(ctx, automationInvokeTimeout)
	defer cancel()

	res, ok := awaitResolution(waitCtx, &c.pendMu, c.pending, id, ch)
	if !ok {
		return nil, fmt.Errorf("automation: command %q timed out or was cancelled", name)
	}
	if res.Err != "" {
		return nil, fmt.Errorf("%s", res.Err)
	}
	return res.Value, nil
}

// resolve delivers a frontend ack to the parked Invoke identified by id.
func (c *AutomationController) resolve(id, valueJSON, errMsg string) error {
	ch, ok := takeResolution(&c.pendMu, c.pending, id)
	if !ok {
		return fmt.Errorf("automation: no pending command with id %q", id)
	}
	ch <- automationResult{Value: json.RawMessage(valueJSON), Err: errMsg}
	return nil
}
