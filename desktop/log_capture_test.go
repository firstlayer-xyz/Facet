package main

import (
	"io"
	"strings"
	"testing"
	"time"
)

// TestPumpStderrDrainsOversizedLine guards the log-capture deadlock: a stderr
// line longer than the scanner's buffer cap used to stop the reader goroutine,
// after which os.Stderr's pipe filled and every log.Printf blocked forever.
// pumpStderr must keep draining so writes never block.
func TestPumpStderrDrainsOversizedLine(t *testing.T) {
	r, w := io.Pipe()
	var out strings.Builder
	pumpDone := make(chan struct{})
	go func() { pumpStderr(r, &out, nil); close(pumpDone) }()

	writesDone := make(chan error, 1)
	go func() {
		// One line over the 64 KiB cap, then a normal line. Before the fix, the
		// second write blocks forever because nothing reads the pipe anymore.
		if _, err := io.WriteString(w, strings.Repeat("x", 100*1024)+"\n"); err != nil {
			writesDone <- err
			return
		}
		if _, err := io.WriteString(w, "after\n"); err != nil {
			writesDone <- err
			return
		}
		writesDone <- w.Close()
	}()

	select {
	case err := <-writesDone:
		if err != nil {
			t.Fatalf("writes failed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("writing after an oversized line blocked — the stderr pump wedged")
	}
	<-pumpDone
	if !strings.Contains(out.String(), "after") {
		t.Fatal("the line after the oversized one was never drained to the tee")
	}
}
