//go:build automation

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestControlHandlerInvokes(t *testing.T) {
	c := NewAutomationController()
	c.SetEventContext(context.Background())
	c.emit = func(_ context.Context, p AutomationInvoke) {
		go c.resolve(p.ID, `{"echo":"`+p.Name+`"}`, "")
	}
	h := controlHandler(c)

	body := `{"name":"viewer.frameAll","params":{}}`
	req := httptest.NewRequest(http.MethodPost, "/control", strings.NewReader(body))
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rw.Code, rw.Body.String())
	}
	var got struct {
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(rw.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if string(got.Value) != `{"echo":"viewer.frameAll"}` {
		t.Fatalf("value = %s", got.Value)
	}
}

func TestControlHandlerCommandError(t *testing.T) {
	c := NewAutomationController()
	c.SetEventContext(context.Background())
	c.emit = func(_ context.Context, p AutomationInvoke) { go c.resolve(p.ID, "", "boom") }
	h := controlHandler(c)

	req := httptest.NewRequest(http.MethodPost, "/control", strings.NewReader(`{"name":"x"}`))
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)
	if rw.Code != http.StatusInternalServerError || !strings.Contains(rw.Body.String(), "boom") {
		t.Fatalf("want 500+boom, got %d %s", rw.Code, rw.Body.String())
	}
}

func TestControlHandlerMissingName(t *testing.T) {
	h := controlHandler(NewAutomationController())
	req := httptest.NewRequest(http.MethodPost, "/control", strings.NewReader(`{"params":{}}`))
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)
	if rw.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for missing name, got %d", rw.Code)
	}
}

func TestControlHandlerRejectsGET(t *testing.T) {
	h := controlHandler(NewAutomationController())
	req := httptest.NewRequest(http.MethodGet, "/control", nil)
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)
	if rw.Code != http.StatusMethodNotAllowed {
		t.Fatalf("want 405, got %d", rw.Code)
	}
}
