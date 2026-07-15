//go:build !automation

package main

import "net/http"

// registerControlRoute is a no-op in non-automation builds: the demo remote-
// control bus is compiled out, so the shipped app never mounts /control.
func registerControlRoute(mux *http.ServeMux, c *AutomationController) {}
