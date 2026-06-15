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

func TestClaudeAssistantSingleTurn(t *testing.T) {
	rec := &recordingEmitter{}
	ca := newClaudeAssistant(rec, nil, fakeBinPath())
	ca.newCmd = fakeCmdFactory(t, "claude", "")
	if err := ca.Send(Turn{UserMessage: "hi"}, SessionConfig{MaxTurns: 2}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	waitForEvent(t, rec, "assistant:done")
	var gotToken string
	rec.mu.Lock()
	for _, e := range rec.events {
		if e[0] == "assistant:token" {
			gotToken = e[1].(string)
		}
	}
	rec.mu.Unlock()
	if gotToken != "echo: hi" {
		t.Fatalf("token = %q, want %q (events: %v)", gotToken, "echo: hi", rec.names())
	}
	ca.Close()
}

func TestClaudeAssistantEmitsToolUseAndThinking(t *testing.T) {
	rec := &recordingEmitter{}
	ca := newClaudeAssistant(rec, nil, fakeBinPath())
	ca.newCmd = fakeCmdFactory(t, "tools", "")
	if err := ca.Send(Turn{UserMessage: "edit"}, SessionConfig{MaxTurns: 5}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	waitForEvent(t, rec, "assistant:done")
	for _, want := range []string{"assistant:tool-use", "assistant:thinking"} {
		found := false
		for _, n := range rec.names() {
			if n == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing %s; got %v", want, rec.names())
		}
	}
	ca.Close()
}

func TestClaudeAssistantSurfacesResultError(t *testing.T) {
	rec := &recordingEmitter{}
	ca := newClaudeAssistant(rec, nil, fakeBinPath())
	ca.newCmd = fakeCmdFactory(t, "error", "")
	if err := ca.Send(Turn{UserMessage: "go"}, SessionConfig{MaxTurns: 3}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	waitForEvent(t, rec, "assistant:error")
	// An is_error result must NOT also emit assistant:done.
	for _, n := range rec.names() {
		if n == "assistant:done" {
			t.Fatalf("is_error result should not emit done; got %v", rec.names())
		}
	}
	ca.Close()
}
