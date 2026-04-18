package main

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
)

// EvalService owns the in-flight-eval cancellation state. The /eval HTTP
// handler cancels any previous eval before starting a new one so that a
// stale long-running evaluation cannot keep consuming CPU after the user
// has moved on.
type EvalService struct {
	mu     sync.Mutex
	cancel context.CancelFunc
}

// NewEvalService creates a new eval service.
func NewEvalService() *EvalService {
	return &EvalService{}
}

// HTTPHandler returns the http.HandlerFunc for the /eval endpoint. It
// decodes the JSON request body, cancels any previous eval, and dispatches
// to handleEval with a fresh context derived from the HTTP request.
func (s *EvalService) HTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req evalRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Cancel previous eval, derive context from request
		s.mu.Lock()
		if s.cancel != nil {
			s.cancel()
		}
		ctx, cancel := context.WithCancel(r.Context())
		s.cancel = cancel
		s.mu.Unlock()
		defer cancel()

		handleEval(ctx, w, req)
	}
}
