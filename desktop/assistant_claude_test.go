package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildUserFrameTextOnly(t *testing.T) {
	frame, err := buildUserFrame("hello world", nil)
	if err != nil {
		t.Fatalf("buildUserFrame: %v", err)
	}
	var msg struct {
		Type    string `json:"type"`
		Message struct {
			Role    string `json:"role"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(frame, &msg); err != nil {
		t.Fatalf("frame is not valid JSON: %v\n%s", err, frame)
	}
	if msg.Type != "user" || msg.Message.Role != "user" {
		t.Fatalf("bad envelope: %+v", msg)
	}
	if len(msg.Message.Content) != 1 || msg.Message.Content[0].Type != "text" || msg.Message.Content[0].Text != "hello world" {
		t.Fatalf("bad content: %+v", msg.Message.Content)
	}
	if bytes.ContainsRune(frame, '\n') {
		t.Fatalf("frame must be single-line NDJSON; got newline in %s", frame)
	}
}

func TestBuildUserFrameWithImage(t *testing.T) {
	dir := t.TempDir()
	png := filepath.Join(dir, "x.png")
	raw, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==")
	if err := os.WriteFile(png, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	frame, err := buildUserFrame("what is this", []string{png})
	if err != nil {
		t.Fatalf("buildUserFrame: %v", err)
	}
	if !bytes.Contains(frame, []byte(`"type":"image"`)) ||
		!bytes.Contains(frame, []byte(`"media_type":"image/png"`)) ||
		!bytes.Contains(frame, []byte(`"type":"base64"`)) {
		t.Fatalf("image block missing/wrong: %s", frame)
	}
}
