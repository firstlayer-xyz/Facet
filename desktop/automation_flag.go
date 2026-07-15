//go:build automation

package main

import (
	"strconv"
	"strings"
)

const defaultAutomationPort = 8791

// parseAutomationFlag reads --automation / --automation=PORT from args. An
// unparseable or non-positive port falls back to the default port (the flag is
// still enabled). Absent flag → disabled, zero port. Automation builds only;
// the shipped app uses the stub in automation_flag_stub.go, which ignores the
// flag entirely.
func parseAutomationFlag(args []string) AutomationConfig {
	for _, a := range args {
		if a == "--automation" {
			return AutomationConfig{Enabled: true, Port: defaultAutomationPort}
		}
		if strings.HasPrefix(a, "--automation=") {
			port := defaultAutomationPort
			if p, err := strconv.Atoi(strings.TrimPrefix(a, "--automation=")); err == nil && p > 0 {
				port = p
			}
			return AutomationConfig{Enabled: true, Port: port}
		}
	}
	return AutomationConfig{}
}
