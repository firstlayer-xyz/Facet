package main

import (
	"context"
	"encoding/json"
	"testing"
)

// stubAutomation returns a controller whose frontend is faked: every emit is
// immediately resolved with the given error string (empty = success), and the
// last invoke is captured for assertion.
func stubAutomation(resolveErr string) (*AutomationController, *AutomationInvoke) {
	c := NewAutomationController()
	c.SetEventContext(context.Background())
	last := &AutomationInvoke{}
	c.emit = func(_ context.Context, p AutomationInvoke) {
		*last = p
		go c.resolve(p.ID, "null", resolveErr)
	}
	return c, last
}

func TestInvokeGUISuccess(t *testing.T) {
	c, last := stubAutomation("")
	m := NewMCPService(NewEvalService(), c)

	res, _, err := m.invokeGUI(context.Background(), "viewer.setCamera", guiSetCameraInput{Azimuth: 45, Elevation: 30})
	if err != nil {
		t.Fatalf("invokeGUI go-err: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected IsError result: %+v", res)
	}
	if last.Name != "viewer.setCamera" {
		t.Fatalf("controller saw name %q, want viewer.setCamera", last.Name)
	}
	var got guiSetCameraInput
	if err := json.Unmarshal(last.Params, &got); err != nil {
		t.Fatalf("params unmarshal: %v", err)
	}
	if got.Azimuth != 45 || got.Elevation != 30 {
		t.Fatalf("params = %+v", got)
	}
}

func TestInvokeGUIError(t *testing.T) {
	c, _ := stubAutomation("viewer exploded")
	m := NewMCPService(NewEvalService(), c)

	res, _, err := m.invokeGUI(context.Background(), "viewer.setCamera", guiSetCameraInput{})
	if err != nil {
		t.Fatalf("invokeGUI should surface command failure as an IsError result, not a go err: %v", err)
	}
	if !res.IsError {
		t.Fatal("want IsError result on command failure")
	}
}
