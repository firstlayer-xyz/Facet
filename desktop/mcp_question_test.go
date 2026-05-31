package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

// newQuestionTestService constructs a minimal MCPService with just the
// fields AnswerQuestion and DeliverScreenshot touch — no HTTP server,
// no event context, no eval wiring. The real service is much heavier;
// nothing in those dependencies affects the channel-routing semantics
// under test.
func newQuestionTestService() *MCPService {
	return &MCPService{
		questions:   make(map[string]chan questionAnswer),
		screenshots: make(map[string]chan screenshotResult),
	}
}

// TestAnswerQuestionRoutesToPendingChannel is the happy path: register
// a channel, AnswerQuestion delivers the answer, the channel receives
// exactly the payload that was sent.
func TestAnswerQuestionRoutesToPendingChannel(t *testing.T) {
	m := newQuestionTestService()
	ch := make(chan questionAnswer, 1)
	m.questions["q1"] = ch

	err := m.AnswerQuestion("q1", map[string]string{"How tall?": "10 cm"}, map[string]string{"How tall?": "user typed 10 cm"})
	if err != nil {
		t.Fatalf("AnswerQuestion returned %v, want nil", err)
	}

	select {
	case got := <-ch:
		if got.Answers["How tall?"] != "10 cm" {
			t.Errorf("answer payload: got %v", got.Answers)
		}
		if got.Notes["How tall?"] != "user typed 10 cm" {
			t.Errorf("note payload: got %v", got.Notes)
		}
	default:
		t.Fatal("channel had no value — AnswerQuestion failed to deliver")
	}
}

// TestAnswerQuestionUnknownIDErrors guards against late or bogus
// answers reaching a question that has already completed (channel
// removed) or never existed (frontend bug or race).
func TestAnswerQuestionUnknownIDErrors(t *testing.T) {
	m := newQuestionTestService()
	err := m.AnswerQuestion("does-not-exist", map[string]string{"q": "a"}, nil)
	if err == nil {
		t.Fatal("expected error for unknown id, got nil")
	}
}

// TestAnswerQuestionDoubleSendDrops confirms the second AnswerQuestion
// call for the same id returns an error rather than blocking. The
// channel is buffered with cap 1; the second send must surface a
// problem rather than deadlock the calling goroutine.
func TestAnswerQuestionDoubleSendDrops(t *testing.T) {
	m := newQuestionTestService()
	ch := make(chan questionAnswer, 1)
	m.questions["q2"] = ch

	if err := m.AnswerQuestion("q2", map[string]string{"q": "first"}, nil); err != nil {
		t.Fatalf("first AnswerQuestion: %v", err)
	}
	if err := m.AnswerQuestion("q2", map[string]string{"q": "second"}, nil); err == nil {
		t.Fatal("expected second AnswerQuestion to error (channel full), got nil")
	}
}

// TestDeliverScreenshotStripsDataURLPrefix verifies the toDataURL
// envelope ("data:image/png;base64,...") is stripped before base64
// decoding — otherwise the raw decoder fails on the "data:" header
// and the model gets a malformed image. The PNG bytes that reach the
// channel must be the decoded payload, not the wrapped wire format.
func TestDeliverScreenshotStripsDataURLPrefix(t *testing.T) {
	m := newQuestionTestService()
	ch := make(chan screenshotResult, 1)
	m.screenshots["s1"] = ch

	rawPNG := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A} // PNG magic bytes
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(rawPNG)

	if err := m.DeliverScreenshot("s1", dataURL, ""); err != nil {
		t.Fatalf("DeliverScreenshot: %v", err)
	}

	select {
	case res := <-ch:
		if res.Err != "" {
			t.Fatalf("unexpected error on result: %s", res.Err)
		}
		if !bytes.Equal(res.PNG, rawPNG) {
			t.Errorf("decoded PNG mismatch: got %x, want %x", res.PNG, rawPNG)
		}
	default:
		t.Fatal("channel had no value — DeliverScreenshot failed to deliver")
	}
}

// TestDeliverScreenshotAcceptsBareBase64 confirms the prefix-strip is
// optional — a bare base64 string (no data: envelope) also works, so a
// caller that already stripped the header doesn't get a decode error.
func TestDeliverScreenshotAcceptsBareBase64(t *testing.T) {
	m := newQuestionTestService()
	ch := make(chan screenshotResult, 1)
	m.screenshots["s2"] = ch

	rawPNG := []byte{0x89, 0x50, 0x4E, 0x47}
	bare := base64.StdEncoding.EncodeToString(rawPNG)

	if err := m.DeliverScreenshot("s2", bare, ""); err != nil {
		t.Fatalf("DeliverScreenshot: %v", err)
	}
	res := <-ch
	if !bytes.Equal(res.PNG, rawPNG) {
		t.Errorf("decoded PNG mismatch: got %x, want %x", res.PNG, rawPNG)
	}
}

// TestDeliverScreenshotErrorPath confirms an errMsg propagates as a
// tool error instead of being swallowed — the model needs to see why
// the capture failed (e.g. WebGL context lost).
func TestDeliverScreenshotErrorPath(t *testing.T) {
	m := newQuestionTestService()
	ch := make(chan screenshotResult, 1)
	m.screenshots["s3"] = ch

	if err := m.DeliverScreenshot("s3", "", "webgl context lost"); err != nil {
		t.Fatalf("DeliverScreenshot: %v", err)
	}
	res := <-ch
	if res.Err != "webgl context lost" {
		t.Errorf("error message lost: got %q", res.Err)
	}
	if res.PNG != nil {
		t.Errorf("unexpected PNG payload on error path: %x", res.PNG)
	}
}

// TestDeliverScreenshotUnknownID guards the late/bogus-delivery path:
// a stale call (already returned, ctx cancelled, etc.) should not be
// able to land in an arbitrary other channel.
func TestDeliverScreenshotUnknownID(t *testing.T) {
	m := newQuestionTestService()
	err := m.DeliverScreenshot("does-not-exist", "anything", "")
	if err == nil {
		t.Fatal("expected error for unknown id, got nil")
	}
}

// TestDeliverScreenshotBadBase64 surfaces a malformed payload as a
// decode error rather than passing garbage bytes to the model.
func TestDeliverScreenshotBadBase64(t *testing.T) {
	m := newQuestionTestService()
	ch := make(chan screenshotResult, 1)
	m.screenshots["s4"] = ch

	err := m.DeliverScreenshot("s4", "this is not base64 !!!", "")
	if err == nil {
		t.Fatal("expected base64 decode error, got nil")
	}
	// Channel must NOT have received the bad payload — the call errored
	// before sending.
	select {
	case res := <-ch:
		t.Errorf("channel received unexpected payload after decode error: %+v", res)
	default:
	}
}
