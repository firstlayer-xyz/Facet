package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
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
