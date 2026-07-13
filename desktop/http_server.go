package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// generateToken returns a 32-byte random token hex-encoded (64 chars).
func generateToken() (string, error) {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(buf[:]), nil
}

// authMiddleware enforces bearer-token authentication and Host-header
// validation on every incoming request.  The Host check is a belt-and-braces
// defense against DNS-rebinding attacks that target localhost services.
//
// CORS: the Wails WebView origin ("wails://wails.localhost" on macOS,
// "http://wails.localhost" on Windows) differs from the server's origin
// ("http://127.0.0.1:<port>"), so any fetch with a non-safelisted header
// triggers a preflight.  We answer preflights after the Host check but
// without requiring Authorization (the browser never sends credentials
// on preflight — that's the protocol).  Security still rests on the
// bearer token on the actual request: wildcard Access-Control-Allow-Origin
// only tells a browser it may read the response, it does not bypass auth.
// maxRequestBodyBytes caps a single request body. Requests are small JSON eval
// payloads; this only stops a runaway/malicious upload.
const maxRequestBodyBytes int64 = 64 << 20 // 64 MiB

func authMiddleware(token string, port int, next http.Handler) http.Handler {
	allowedHosts := map[string]bool{
		fmt.Sprintf("127.0.0.1:%d", port): true,
		fmt.Sprintf("localhost:%d", port): true,
	}
	expected := []byte("Bearer " + token)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// A panic deep in the evaluator/manifold cgo chain would otherwise crash
		// the whole app; contain it to a 500 for this one request.
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("[http] panic handling %s %s: %v", r.Method, r.URL.Path, rec)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		if !allowedHosts[r.Host] {
			http.Error(w, "forbidden host", http.StatusBadRequest)
			return
		}
		// Bound the request body so a malformed/huge upload can't exhaust memory.
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)

		w.Header().Set("Access-Control-Allow-Origin", "*")

		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Max-Age", "86400")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		got := r.Header.Get("Authorization")
		if subtle.ConstantTimeCompare([]byte(got), expected) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// HTTPServer is the app's localhost HTTP server. It owns the listener, the
// bearer token, and the readiness signal, and mounts the core editor/viewer
// routes (/eval, /frame, /check) alongside the assistant's /mcp endpoint. The
// eval routes do not depend on the assistant: the MCPService only contributes a
// handler, so a change or failure in the assistant tooling cannot stop /eval
// from binding.
type HTTPServer struct {
	eval     *EvalService
	mcp      *MCPService
	port     int
	token    string
	startErr error         // non-nil if Start failed; read after <-ready
	ready    chan struct{} // closed once Start has finished — success or failure
}

// NewHTTPServer wires the core eval service and the MCP handler provider. The
// listener is not bound until Start is called.
func NewHTTPServer(eval *EvalService, mcp *MCPService) *HTTPServer {
	return &HTTPServer{eval: eval, mcp: mcp, ready: make(chan struct{})}
}

// Start binds the listener, mounts the routes (including the MCP server the
// MCPService builds), and serves until ctx is cancelled. It sets port/token and
// closes ready before returning, so WaitReady unblocks as soon as the listener
// is accepting connections.
func (s *HTTPServer) Start(ctx context.Context) error {
	// bind owns all the fallible work; Start closes ready on every path (success
	// or failure) with startErr recorded, so WaitReady never hangs on a failed
	// Start and callers can tell "not ready yet" from "will never be ready".
	err := s.bind(ctx)
	s.startErr = err
	close(s.ready)
	return err
}

func (s *HTTPServer) bind(ctx context.Context) error {
	token, err := generateToken()
	if err != nil {
		return err
	}

	// Bind first so the port is known before wiring middleware.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	mcpServer := s.mcp.buildServer(ctx)
	handleMcp := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return mcpServer
	}, &mcp.StreamableHTTPOptions{Stateless: true})

	frameSessions := newSessionCache()

	mux := http.NewServeMux()
	mux.Handle("/mcp", handleMcp)
	mux.Handle("/eval", s.eval.HTTPHandler(frameSessions))
	mux.HandleFunc("/check", handleCheck)
	mux.HandleFunc("/frame", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req frameRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
			return
		}
		handleFrame(r.Context(), w, req, frameSessions)
	})

	log.Printf("[http] server listening on http://127.0.0.1:%d (eval, mcp)", port)

	httpServer := &http.Server{
		Handler: authMiddleware(token, port, mux),
		// Defend against a slow-header (slowloris) client holding a connection.
		// No ReadTimeout/WriteTimeout — an eval can legitimately run for a while.
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("[http] server error: %v", err)
		}
	}()
	go func() {
		<-ctx.Done()
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer shutCancel()
		httpServer.Shutdown(shutCtx)
	}()

	s.port = port
	s.token = token
	return nil
}

// Auth returns the port + bearer token as an HTTPAuth payload for the frontend.
func (s *HTTPServer) Auth() HTTPAuth { return HTTPAuth{Port: s.port, Token: s.token} }

// Endpoint returns the port + bearer token the assistant uses to reach /mcp.
// Satisfies the connection half of AssistantMCPBridge.
func (s *HTTPServer) Endpoint() (int, string) {
	_ = s.WaitReady() // same ready-or-10s gate; Endpoint just ignores the start error
	return s.port, s.token
}

// WaitReady blocks until Start has finished, returning nil once the listener is
// bound or the error Start failed with, or a timeout error if Start neither
// succeeds nor fails within 10s (it should always resolve quickly, since Start
// closes ready on every path). GetHTTPAuth calls this so a frontend eval issued
// during startup waits for the server and surfaces a real error instead of
// silently reading port 0. No caller context is needed: the ready signal plus
// the safety timeout bound the wait, and Start owns its own ctx-driven shutdown.
func (s *HTTPServer) WaitReady() error {
	select {
	case <-s.ready:
		return s.startErr
	case <-time.After(10 * time.Second):
		return fmt.Errorf("HTTP server did not become ready within 10s")
	}
}
