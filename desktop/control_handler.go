//go:build automation

package main

import (
	"encoding/json"
	"net/http"
)

// registerControlRoute mounts the /control endpoint on mux. Automation-build
// only; the non-automation stub is a no-op, so a shipped app never exposes the
// remote-control bus.
func registerControlRoute(mux *http.ServeMux, c *AutomationController) {
	mux.Handle("/control", controlHandler(c))
}

// controlRequest is the /control body: a command name and opaque params.
type controlRequest struct {
	Name   string          `json:"name"`
	Params json.RawMessage `json:"params"`
}

// controlHandler exposes the automation command registry over HTTP for
// hand-written driver scripts. It is a thin adapter over AutomationController:
// POST a name+params, get back the command's JSON result or its error. The
// route is only reachable without a token when the --automation flag disables
// auth (see authMiddleware); otherwise the bearer token still gates it.
func controlHandler(c *AutomationController) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req controlRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
			return
		}
		if req.Name == "" {
			http.Error(w, "missing command name", http.StatusBadRequest)
			return
		}
		value, err := c.Invoke(r.Context(), req.Name, req.Params)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if value == nil {
			value = json.RawMessage("null")
		}
		writeJSON(w, http.StatusOK, map[string]json.RawMessage{"value": value})
	})
}

// writeJSON serializes v as the response body with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
