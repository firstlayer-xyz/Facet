//go:build automation

package main

import "testing"

func TestParseAutomationFlag(t *testing.T) {
	cases := []struct {
		args []string
		want AutomationConfig
	}{
		{nil, AutomationConfig{Enabled: false, Port: 0}},
		{[]string{"--automation"}, AutomationConfig{Enabled: true, Port: 8791}},
		{[]string{"--automation=9000"}, AutomationConfig{Enabled: true, Port: 9000}},
		{[]string{"--other", "--automation=9000", "x"}, AutomationConfig{Enabled: true, Port: 9000}},
		{[]string{"--automation=notaport"}, AutomationConfig{Enabled: true, Port: 8791}},
	}
	for _, c := range cases {
		if got := parseAutomationFlag(c.args); got != c.want {
			t.Errorf("parseAutomationFlag(%v) = %+v, want %+v", c.args, got, c.want)
		}
	}
}
