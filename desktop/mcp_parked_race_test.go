package main

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

// TestAnswerQuestionLosesToCancellation: when the handler cancels first (take-
// and-removes its own entry), a subsequent answer finds no waiter and errors —
// no silent nil-on-dropped-answer.
func TestAnswerQuestionLosesToCancellation(t *testing.T) {
	m := NewMCPService(NewEvalService(), NewAutomationController())
	id := "q1"
	ch := make(chan questionAnswer, 1)
	m.questionsMu.Lock()
	m.questions[id] = ch
	m.questionsMu.Unlock()

	// Handler's cancel branch removes the entry first.
	takeResolution(&m.questionsMu, m.questions, id)

	if err := m.AnswerQuestion(id, map[string]string{"a": "1"}, nil); err == nil {
		t.Fatal("expected an error answering an already-cancelled question, got nil")
	}
}

// TestAnswerQuestionWinsRace: when the answer lands first (take-and-remove +
// buffer), the handler's cancel branch observes the entry gone and drains the
// buffered answer instead of reporting cancellation.
func TestAnswerQuestionWinsRace(t *testing.T) {
	m := NewMCPService(NewEvalService(), NewAutomationController())
	id := "q1"
	ch := make(chan questionAnswer, 1)
	m.questionsMu.Lock()
	m.questions[id] = ch
	m.questionsMu.Unlock()

	if err := m.AnswerQuestion(id, map[string]string{"a": "1"}, nil); err != nil {
		t.Fatalf("answer should succeed: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // handler's ctx is already done, yet an answer is buffered
	ans, ok := awaitResolution(ctx, &m.questionsMu, m.questions, id, ch)
	if !ok {
		t.Fatal("handler should have drained the buffered answer, reported cancelled")
	}
	if ans.Answers["a"] != "1" {
		t.Fatalf("drained wrong answer: %v", ans.Answers)
	}
}

// TestAnswerQuestionCancelRaceInvariant stress-tests the resolution token under
// -race: for a handler parked on awaitResolution while cancel and AnswerQuestion
// fire concurrently, the answer is honored iff AnswerQuestion succeeded — the
// bug (handler cancelled AND answer silently returned nil) never occurs.
func TestAnswerQuestionCancelRaceInvariant(t *testing.T) {
	m := NewMCPService(NewEvalService(), NewAutomationController())
	for i := 0; i < 5000; i++ {
		id := fmt.Sprintf("q%d", i)
		ch := make(chan questionAnswer, 1)
		m.questionsMu.Lock()
		m.questions[id] = ch
		m.questionsMu.Unlock()

		ctx, cancel := context.WithCancel(context.Background())
		var handlerOK bool
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, handlerOK = awaitResolution(ctx, &m.questionsMu, m.questions, id, ch)
		}()
		go cancel()
		answerErr := m.AnswerQuestion(id, map[string]string{"a": "1"}, nil)
		wg.Wait()

		// The answer was honored (handlerOK) exactly when AnswerQuestion succeeded.
		if handlerOK != (answerErr == nil) {
			t.Fatalf("iter %d: invariant violated: handlerOK=%v answerErr=%v", i, handlerOK, answerErr)
		}
	}
}
