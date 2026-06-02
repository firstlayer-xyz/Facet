package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fetchContent is called with an explicit client so tests can target httptest
// servers (which listen on loopback and would be blocked by newFetchClient's
// dialer). The SSRF dialer is verified separately in urlfetch_ssrf_test.go.

func TestFetchContentImage(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\nfake-png-bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(png)
	}))
	defer srv.Close()

	got, err := fetchContent(context.Background(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("fetchContent: %v", err)
	}
	if !got.IsImage || got.MIME != "image/png" {
		t.Fatalf("got IsImage=%v MIME=%q, want image/png", got.IsImage, got.MIME)
	}
	if string(got.Data) != string(png) {
		t.Fatalf("image bytes mismatch")
	}
}

func TestFetchContentText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	got, err := fetchContent(context.Background(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("fetchContent: %v", err)
	}
	if got.IsImage {
		t.Fatal("json should not be IsImage")
	}
	if !strings.Contains(got.Text, `{"ok":true}`) {
		t.Fatalf("body not in text: %q", got.Text)
	}
}

func TestFetchContentSVGisText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Write([]byte(`<svg></svg>`))
	}))
	defer srv.Close()

	got, err := fetchContent(context.Background(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("fetchContent: %v", err)
	}
	if got.IsImage {
		t.Fatal("svg should be returned as text, not image")
	}
	if !strings.Contains(got.Text, "<svg>") {
		t.Fatalf("svg source not in text: %q", got.Text)
	}
}

func TestFetchContentUnsupportedTypeErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		w.Write([]byte("%PDF-1.4"))
	}))
	defer srv.Close()
	if _, err := fetchContent(context.Background(), srv.Client(), srv.URL); err == nil {
		t.Fatal("expected error for application/pdf, got nil")
	}
}

func TestFetchContentNon2xxErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer srv.Close()
	if _, err := fetchContent(context.Background(), srv.Client(), srv.URL); err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}

func TestFetchContentOversizeImageErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		big := make([]byte, maxImageBytes+10)
		w.Write(big)
	}))
	defer srv.Close()
	if _, err := fetchContent(context.Background(), srv.Client(), srv.URL); err == nil {
		t.Fatal("expected error for oversize image, got nil")
	}
}

func TestFetchContentTextTruncates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(strings.Repeat("a", maxTextBytes+100)))
	}))
	defer srv.Close()
	got, err := fetchContent(context.Background(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("fetchContent: %v", err)
	}
	if !strings.Contains(got.Text, "[truncated:") {
		t.Fatalf("expected truncation marker, got tail: %q", got.Text[len(got.Text)-80:])
	}
}
