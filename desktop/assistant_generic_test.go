package main

import "testing"

func TestGenericCLIAssistantStreamsTokensThenDone(t *testing.T) {
	rec := &recordingEmitter{}
	// cliID must be a CLI that genericCLIArgs understands ("ollama"); the fake
	// ignores the resulting args and just echoes the payload.
	ga := newGenericCLIAssistant(rec, "ollama", fakeBinPath())
	ga.newCmd = fakeCmdFactory(t, "echo", "line one\nline two")
	if err := ga.Send(Turn{UserMessage: "hi"}, SessionConfig{}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	waitForEvent(t, rec, "assistant:done")
	var sawToken bool
	for _, n := range rec.names() {
		if n == "assistant:token" {
			sawToken = true
		}
	}
	if !sawToken {
		t.Fatalf("no assistant:token before done; got %v", rec.names())
	}
}
