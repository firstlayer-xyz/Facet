// Tiny dev server for web/. Serves the single-threaded wasm bundle as plain
// static files with the correct wasm content-type — mirroring GitHub Pages
// (no cross-origin-isolation headers, since the build needs no SharedArrayBuffer).
//
// Usage:
//
//	go run scripts/serve-web.go
//	go run scripts/serve-web.go -addr :8080 -dir web
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
		// No caching during dev — always pick up the latest build.
		w.Header().Set("Cache-Control", "no-store")
		// Help the browser hand wasm to streaming compile.
		if strings.HasSuffix(r.URL.Path, ".wasm") {
			w.Header().Set("Content-Type", "application/wasm")
		}
		fs.ServeHTTP(w, r)
	})

	log.Printf("serving %s on http://localhost%s", *dir, *addr)
	log.Fatal(http.ListenAndServe(*addr, handler))
}
