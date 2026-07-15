//go:build !automation

package main

// parseAutomationFlag is inert in non-automation builds: it always reports the
// automation subsystem as disabled, so the --automation flag can never bind a
// fixed port or drop the HTTP bearer-token auth in a shipped app. The demo
// remote-control bus is compiled out entirely (see automation_stub.go).
func parseAutomationFlag(args []string) AutomationConfig { return AutomationConfig{} }
