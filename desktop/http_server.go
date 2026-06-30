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
func authMiddleware(token string, port int, next http.Handler) http.Handler {
	allowedHosts := map[string]bool{
		fmt.Sprintf("127.0.0.1:%d", port): true,
		fmt.Sprintf("localhost:%d", port): true,
	}
	expected := []byte("Bearer " + token)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowedHosts[r.Host] {
			http.Error(w, "forbidden host", http.StatusBadRequest)
			return
		}

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
	eval  *EvalService
	mcp   *MCPService
	port  int
	token string
	ready chan struct{} // closed once Start has bound the listener and set port/token
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

	httpServer := &http.Server{Handler: authMiddleware(token, port, mux)}
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
	close(s.ready)
	return nil
}

// Auth returns the port + bearer token as an HTTPAuth payload for the frontend.
func (s *HTTPServer) Auth() HTTPAuth { return HTTPAuth{Port: s.port, Token: s.token} }

// Endpoint returns the port + bearer token the assistant uses to reach /mcp.
// Satisfies the connection half of AssistantMCPBridge.
func (s *HTTPServer) Endpoint() (int, string) { return s.port, s.token }

// WaitReady blocks until Start has bound the listener (port/token set), or until
// ctx is cancelled, or a 10s safety timeout elapses. GetHTTPAuth calls this so a
// frontend eval issued during startup waits for the server instead of reading
// port 0 — which the client caches, dead-ending every eval with "Load failed"
// for the whole session.
func (s *HTTPServer) WaitReady(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-s.ready:
	case <-ctx.Done():
	case <-time.After(10 * time.Second):
	}
}
