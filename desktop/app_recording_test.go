package main

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveRecordingWritesFile(t *testing.T) {
	a := NewApp()
	// A tiny fake webm payload; SaveRecording persists bytes, it does not validate.
	data := []byte("webm-bytes")
	path, err := a.SaveRecording("data:video/webm;base64,"+base64.StdEncoding.EncodeToString(data), "")
	if err != nil {
		t.Fatalf("SaveRecording: %v", err)
	}
	if !strings.HasSuffix(path, ".webm") {
		t.Fatalf("path should end .webm: %s", path)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != "webm-bytes" {
		t.Fatalf("content = %q", got)
	}
	os.Remove(path)
}

func TestSaveRecordingAcceptsBareBase64(t *testing.T) {
	a := NewApp()
	path, err := a.SaveRecording(base64.StdEncoding.EncodeToString([]byte("x")), "")
	if err != nil {
		t.Fatalf("SaveRecording bare base64: %v", err)
	}
	os.Remove(path)
}

func TestSaveRecordingRejectsBadBase64(t *testing.T) {
	a := NewApp()
	if _, err := a.SaveRecording("data:video/webm;base64,!!!not-base64!!!", ""); err == nil {
		t.Fatal("want error for undecodable payload")
	}
}

func TestSaveRecordingUsesName(t *testing.T) {
	a := NewApp()
	path, err := a.SaveRecording(base64.StdEncoding.EncodeToString([]byte("x")), "Panda Bear!")
	if err != nil {
		t.Fatalf("SaveRecording: %v", err)
	}
	defer os.Remove(path)
	// "Panda Bear!" -> filename-safe "Panda-Bear" prefix.
	if base := filepath.Base(path); !strings.HasPrefix(base, "Panda-Bear-") {
		t.Fatalf("expected sanitized name prefix, got %s", base)
	}
}

func TestSanitizeRecordingName(t *testing.T) {
	cases := map[string]string{
		"panda":      "panda",
		"cube grid":  "cube-grid",
		"a/b:c*d":    "a-b-c-d",
		"  spaced  ": "spaced",
		"!!!":        "",
		"v1.2_final": "v1.2_final",
	}
	for in, want := range cases {
		if got := sanitizeRecordingName(in); got != want {
			t.Errorf("sanitizeRecordingName(%q) = %q, want %q", in, got, want)
		}
	}
}
