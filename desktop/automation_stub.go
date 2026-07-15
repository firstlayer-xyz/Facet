//go:build !automation

package main

import "context"

// AutomationController is a no-op in non-automation builds: the demo remote-
// control bus (the real controller, /control route, and gui_* MCP tools) is
// compiled out of the shipped app, so nothing ever emits a command. The App
// keeps the field and the Wails-bound AutomationResult method so the frontend
// contract stays stable; both resolve to no-ops here.
type AutomationController struct{}

// NewAutomationController returns the no-op controller.
func NewAutomationController() *AutomationController { return &AutomationController{} }

// SetEventContext is a no-op: there is nothing to emit events to.
func (c *AutomationController) SetEventContext(context.Context) {}

// resolve is a no-op: no command is ever in flight in a non-automation build,
// so there is nothing to resolve.
func (c *AutomationController) resolve(id, valueJSON, errMsg string) error { return nil }
