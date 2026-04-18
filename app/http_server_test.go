package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestAuthMiddleware guards the security boundary introduced for Critical #1
// app-shell (2026-04-16 main-branch review): the localhost HTTP server must
// reject requests that lack the bearer token or target a non-local Host
// header (DNS rebinding defense).
func TestAuthMiddleware(t *testing.T) {
	const token = "test-token-abc123"
	const port = 12345
	stub := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := authMiddleware(token, port, stub)

	cases := []struct {
		name     string
		method   string
		host     string
		authHdr  string
		wantCode int
	}{
		{"valid 127.0.0.1", http.MethodPost, "127.0.0.1:12345", "Bearer " + token, http.StatusOK},
		{"valid localhost", http.MethodPost, "localhost:12345", "Bearer " + token, http.StatusOK},
		{"missing auth header", http.MethodPost, "127.0.0.1:12345", "", http.StatusUnauthorized},
		{"wrong token", http.MethodPost, "127.0.0.1:12345", "Bearer nope", http.StatusUnauthorized},
		{"wrong scheme", http.MethodPost, "127.0.0.1:12345", "Basic " + token, http.StatusUnauthorized},
		{"token without prefix", http.MethodPost, "127.0.0.1:12345", token, http.StatusUnauthorized},
		{"foreign host (DNS rebind)", http.MethodPost, "evil.example.com:12345", "Bearer " + token, http.StatusBadRequest},
		{"wrong port", http.MethodPost, "127.0.0.1:99999", "Bearer " + token, http.StatusBadRequest},
		{"empty host", http.MethodPost, "", "Bearer " + token, http.StatusBadRequest},
		// CORS preflight: the browser sends OPTIONS with no Authorization
		// header before a cross-origin POST that carries Authorization.
		// We must answer 204 so the real POST can follow.
		{"preflight without auth", http.MethodOptions, "127.0.0.1:12345", "", http.StatusNoContent},
		{"preflight on localhost", http.MethodOptions, "localhost:12345", "", http.StatusNoContent},
		{"preflight foreign host still rejected", http.MethodOptions, "evil.example.com:12345", "", http.StatusBadRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/eval", nil)
			req.Host = tc.host
			if tc.authHdr != "" {
				req.Header.Set("Authorization", tc.authHdr)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.wantCode {
				t.Errorf("got %d, want %d (body: %s)", rec.Code, tc.wantCode, rec.Body.String())
			}
		})
	}
}

// TestAuthMiddlewareCORSHeaders verifies that preflight responses carry
// the headers the browser needs to let the real POST proceed, and that
// successful responses carry Access-Control-Allow-Origin so the frontend
// can read the body.
func TestAuthMiddlewareCORSHeaders(t *testing.T) {
	const token = "test-token-abc123"
	const port = 12345
	stub := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := authMiddleware(token, port, stub)

	t.Run("preflight carries CORS headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/eval", nil)
		req.Host = "127.0.0.1:12345"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		want := map[string]string{
			"Access-Control-Allow-Origin":  "*",
			"Access-Control-Allow-Methods": "POST, OPTIONS",
			"Access-Control-Allow-Headers": "Authorization, Content-Type",
		}
		for k, v := range want {
			if got := rec.Header().Get(k); got != v {
				t.Errorf("header %s = %q, want %q", k, got, v)
			}
		}
	})

	t.Run("successful response carries ACAO", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/eval", nil)
		req.Host = "127.0.0.1:12345"
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
			t.Errorf("Access-Control-Allow-Origin = %q, want *", got)
		}
	})
}

// TestGenerateToken sanity-checks the token generator: non-empty, unique,
// and the expected 64-char hex length for 32 random bytes.
func TestGenerateToken(t *testing.T) {
	t1, err := generateToken()
	if err != nil {
		t.Fatalf("generateToken: %v", err)
	}
	t2, err := generateToken()
	if err != nil {
		t.Fatalf("generateToken: %v", err)
	}
	if len(t1) != 64 {
		t.Errorf("token length = %d, want 64 hex chars", len(t1))
	}
	if t1 == t2 {
		t.Error("two successive tokens are identical — generator is not random")
	}
}
