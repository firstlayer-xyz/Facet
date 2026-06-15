package main

import (
	"context"
	"testing"
)

func TestWailsEmitterImplementsEventEmitter(t *testing.T) {
	var _ EventEmitter = newWailsEmitter(context.Background())
}

func TestTurnAndSessionConfigZeroValues(t *testing.T) {
	var turn Turn
	if turn.UserMessage != "" || len(turn.ImagePaths) != 0 {
		t.Fatalf("unexpected non-zero Turn: %+v", turn)
	}
	var cfg SessionConfig
	if cfg.MaxTurns != 0 || cfg.SystemPrompt != "" {
		t.Fatalf("unexpected non-zero SessionConfig: %+v", cfg)
	}
}
