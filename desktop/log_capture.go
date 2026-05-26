package main

import (
	"bufio"
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// ringBuffer is a simple capped byte buffer for capturing stderr output.
type ringBuffer struct {
	mu   sync.Mutex
	data []byte
	max  int
}

func (rb *ringBuffer) Write(p []byte) (int, error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.data = append(rb.data, p...)
	if len(rb.data) > rb.max {
		rb.data = rb.data[len(rb.data)-rb.max:]
	}
	return len(p), nil
}

func (rb *ringBuffer) String() string {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return string(rb.data)
}

// logDir returns the path to the Facet logs directory.
func logDir() string {
	return filepath.Join(configDir(), "logs")
}

// rotateOldLogs deletes log files older than 7 days from the logs directory.
func rotateOldLogs() {
	dir := logDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".log") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

// LogCapture redirects process stderr through a pipe, tees to the original
// stderr plus an in-memory ring buffer plus a daily log file, and emits
// "log:stderr" events so the frontend log panel can stream lines as they
// arrive. Start must be called before Stderr returns non-empty data.
type LogCapture struct {
	stderrBuf *ringBuffer
	logFile   *os.File
}

// NewLogCapture creates a new (unstarted) log capture.
func NewLogCapture() *LogCapture {
	return &LogCapture{}
}

// Start begins stderr capture. The scanner goroutine exits when ctx is
// cancelled. Start is not safe to call concurrently or more than once — the
// app calls it exactly once during startup.
func (lc *LogCapture) Start(ctx context.Context) {
	lc.stderrBuf = &ringBuffer{max: 256 * 1024} // 256 KB ring buffer

	// Create log directory and rotate old logs
	dir := logDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("LogCapture.Start: mkdir error: %v", err)
	}
	rotateOldLogs()

	// Open today's log file (append mode)
	logName := time.Now().Format("2006-01-02") + ".log"
	logFile, fileErr := os.OpenFile(filepath.Join(dir, logName), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if fileErr == nil {
		lc.logFile = logFile
	}

	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		return
	}
	os.Stderr = w

	// Tee pipe output to original stderr + ring buffer + log file
	writers := []io.Writer{origStderr, lc.stderrBuf}
	if fileErr == nil {
		writers = append(writers, logFile)
	}
	tee := io.MultiWriter(writers...)

	go func() {
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 64*1024), 64*1024)
		for scanner.Scan() {
			line := scanner.Text() + "\n"
			tee.Write([]byte(line))
			if ctx != nil {
				wailsRuntime.EventsEmit(ctx, "log:stderr", line)
			}
		}
	}()

	// Close the write end of the pipe when ctx is cancelled,
	// which causes the scanner goroutine above to exit.
	go func() {
		<-ctx.Done()
		w.Close()
	}()
}

// Stderr returns the current stderr ring-buffer contents. Empty string if
// Start has not been called.
func (lc *LogCapture) Stderr() string {
	if lc.stderrBuf == nil {
		return ""
	}
	return lc.stderrBuf.String()
}

// Close closes the underlying log file if one is open.
func (lc *LogCapture) Close() {
	if lc.logFile != nil {
		lc.logFile.Close()
	}
}
