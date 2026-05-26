// Tiny dev server for web/. Sets Cross-Origin-Opener-Policy and
// Cross-Origin-Embedder-Policy so the page runs in a cross-origin
// isolated context — required for SharedArrayBuffer, which Emscripten
// pthread support depends on. Defaults to :8000; override with -addr.
//
// Usage:
//   go run scripts/serve-web.go
//   go run scripts/serve-web.go -addr :8080 -dir web
package main

import (
	"flag"
	"log"
	"net/http"
	"strings"
)

func main() {
	addr := flag.String("addr", ":8000", "listen address")
	dir := flag.String("dir", "web", "directory to serve")
	flag.Parse()

	fs := http.FileServer(http.Dir(*dir))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Cross-origin isolation: gives the page SharedArrayBuffer + Atomics,
		// at the cost of refusing any cross-origin sub-resource that doesn't
		// opt in via Cross-Origin-Resource-Policy.
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		w.Header().Set("Cross-Origin-Embedder-Policy", "require-corp")
		// Tell the browser the wasm assets we serve are same-origin OK,
		// so they pass COEP from sibling pages too.
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		// No caching during dev — always pick up the latest build.
		w.Header().Set("Cache-Control", "no-store")
		// Help the browser hand wasm to streaming compile.
		if strings.HasSuffix(r.URL.Path, ".wasm") {
			w.Header().Set("Content-Type", "application/wasm")
		}
		fs.ServeHTTP(w, r)
	})

	log.Printf("serving %s on http://localhost%s (COOP/COEP isolated)", *dir, *addr)
	log.Fatal(http.ListenAndServe(*addr, handler))
}
