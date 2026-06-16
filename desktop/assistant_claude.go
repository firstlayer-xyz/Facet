package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// contentBlock is one block in a stream-json user message.
type contentBlock struct {
	Type   string       `json:"type"`
	Text   string       `json:"text,omitempty"`
	Source *imageSource `json:"source,omitempty"`
}

type imageSource struct {
	Type      string `json:"type"`       // "base64"
	MediaType string `json:"media_type"` // e.g. "image/png"
	Data      string `json:"data"`       // base64 (no newlines)
}

type userFrame struct {
	Type    string `json:"type"` // "user"
	Message struct {
		Role    string         `json:"role"` // "user"
		Content []contentBlock `json:"content"`
	} `json:"message"`
}

// buildUserText assembles the user-visible prompt text: the message plus the
// inlined editor code and any errors. Images are not inlined here; they ride
// the frame as separate content blocks (see buildUserFrame).
func buildUserText(userMessage, editorCode, errorsText string) string {
	var sb strings.Builder
	sb.WriteString(userMessage)

	if editorCode != "" {
		sb.WriteString("\n\n---\nCurrent editor code:\n```facet\n")
		sb.WriteString(editorCode)
		sb.WriteString("\n```")
	}

	if errorsText != "" {
		sb.WriteString("\n\nCurrent errors:\n```\n")
		sb.WriteString(errorsText)
		sb.WriteString("\n```")
	}

	return sb.String()
}

// buildUserFrame marshals one stream-json user turn: a text block plus one
// base64 image block per attached path. Output is single-line NDJSON (no
// embedded newlines) ready to write to the persistent process's stdin.
func buildUserFrame(text string, imagePaths []string) ([]byte, error) {
	var f userFrame
	f.Type = "user"
	f.Message.Role = "user"
	f.Message.Content = append(f.Message.Content, contentBlock{Type: "text", Text: text})
	for _, p := range imagePaths {
		raw, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read image %s: %w", p, err)
		}
		f.Message.Content = append(f.Message.Content, contentBlock{
			Type: "image",
			Source: &imageSource{
				Type:      "base64",
				MediaType: imageMediaType(p),
				Data:      base64.StdEncoding.EncodeToString(raw),
			},
		})
	}
	return json.Marshal(f)
}

func imageMediaType(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

// launchSig is the set of parameters the CLI fixes at launch; a change requires
// respawning the process. Comparable by ==.
type launchSig struct {
	model        string
	effort       string
	maxTurns     int
	systemPrompt string
	mcpPort      int
	mcpToken     string
}

// claudeProc is one live claude process.
type claudeProc struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	ctx    context.Context
	cancel context.CancelFunc
	doneCh chan struct{} // closed when the reader goroutine returns
}

// claudeAssistant owns one persistent claude stream-json process per
// conversation, keeping the prompt cache warm across turns.
type claudeAssistant struct {
	emit    EventEmitter
	mcp     AssistantMCPBridge
	binPath string
	newCmd  cmdFactory

	// maxLineBytes caps one NDJSON stdout line. A single line can carry a whole
	// tool_result (e.g. a base64 screenshot ~15-20 MiB for a 4K canvas), so the
	// default is generous; tests lower it to exercise the overflow path.
	maxLineBytes int

	mu        sync.Mutex
	proc      *claudeProc
	sessionID string    // assigned UUID, stable across respawns of one conversation
	sig       launchSig // signature the live proc was launched with
	streaming bool
	emitted      bool   // any assistant text emitted during the current turn
	tools        int    // tool_use count during the current turn
	streamErr    string // stream-level error captured this turn, surfaced on failure
	interrupting bool   // true between an Interrupt and the resulting stop result
}

func newClaudeAssistant(emit EventEmitter, mcp AssistantMCPBridge, binPath string) *claudeAssistant {
	return &claudeAssistant{emit: emit, mcp: mcp, binPath: binPath, newCmd: newExecCmd, maxLineBytes: 64 * 1024 * 1024}
}

func (c *claudeAssistant) Send(turn Turn, cfg SessionConfig) error {
	frame, err := buildUserFrame(buildUserText(turn.UserMessage, turn.EditorCode, turn.ErrorsText), turn.ImagePaths)
	if err != nil {
		return err
	}
	mcpPort, mcpToken := 0, ""
	if c.mcp != nil {
		mcpPort, mcpToken = c.mcp.Endpoint()
		c.mcp.SetContext(turn.EditorCode, turn.ActiveTabPath, turn.ActiveTabReadOnly)
	}
	want := launchSig{cfg.Model, cfg.Effort, cfg.MaxTurns, cfg.SystemPrompt, mcpPort, mcpToken}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.streaming {
		return fmt.Errorf("assistant is busy")
	}
	if c.proc == nil || c.sig != want {
		if err := c.startLocked(want); err != nil {
			return err
		}
	}
	c.streaming = true
	c.emitted = false
	c.tools = 0
	c.streamErr = ""
	c.interrupting = false
	if _, err := c.proc.stdin.Write(append(frame, '\n')); err != nil {
		c.streaming = false
		return fmt.Errorf("write frame: %w", err)
	}
	return nil
}

// startLocked spawns a fresh process for sig. Caller holds c.mu. An existing
// process is cancelled fire-and-forget first; if we already have a sessionID we
// resume it, otherwise we assign a new UUID.
func (c *claudeAssistant) startLocked(sig launchSig) error {
	c.closeProcLocked()
	resume := c.sessionID != ""
	if c.sessionID == "" {
		c.sessionID = newUUID()
	}
	args := claudeArgs(sig, c.sessionID, resume)
	ctx, cancel := context.WithCancel(context.Background())
	cmd := c.newCmd(ctx, c.binPath, args...)
	cmd.Dir = os.TempDir()
	cmd.Env = claudeEnv()
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("stdout pipe: %w", err)
	}
	cmd.Stderr = newStderrLogger()
	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("start claude: %w", err)
	}
	p := &claudeProc{cmd: cmd, stdin: stdin, ctx: ctx, cancel: cancel, doneCh: make(chan struct{})}
	c.proc = p
	c.sig = sig
	log.Printf("[assistant] claude session %s started (resume=%v)", c.sessionID, resume)
	go c.read(p, stdout)
	return nil
}

// closeProcLocked cancels the current process WITHOUT waiting. The reader
// detects the closed pipe and cleans up; because callers replace or clear
// c.proc, the old reader's cleanup becomes a no-op (guarded by c.proc == p).
// Caller holds c.mu.
func (c *claudeAssistant) closeProcLocked() {
	if c.proc == nil {
		return
	}
	c.proc.cancel()
	_ = c.proc.stdin.Close()
	c.proc = nil
}

// read consumes the process's stdout for its whole lifetime, emitting events.
func (c *claudeAssistant) read(p *claudeProc, stdout io.Reader) {
	defer close(p.doneCh)
	sc := bufio.NewScanner(stdout)
	init := 256 * 1024
	if c.maxLineBytes < init {
		init = c.maxLineBytes
	}
	sc.Buffer(make([]byte, 0, init), c.maxLineBytes)
	for sc.Scan() {
		c.handleLine(sc.Bytes())
	}
	// A scan error (e.g. a line exceeding maxLineBytes) leaves the child running,
	// so cancel it — otherwise cmd.Wait below blocks forever and the turn never
	// terminates. The error is surfaced explicitly even though the cancel makes
	// ctx.Err() non-nil.
	scanErr := sc.Err()
	if scanErr != nil {
		log.Printf("[assistant] stdout read error: %v", scanErr)
		p.cancel()
	}
	err := p.cmd.Wait()
	c.mu.Lock()
	if c.proc != p {
		c.mu.Unlock()
		return // superseded by a respawn/close — touch no shared state
	}
	wasStreaming := c.streaming
	c.proc = nil
	c.streaming = false
	c.mu.Unlock()
	if !wasStreaming {
		return
	}
	if scanErr != nil {
		c.emit.Emit("assistant:error", fmt.Sprintf("claude: output read error: %v", scanErr))
		return
	}
	if p.ctx.Err() == nil {
		msg := c.takeStreamErr()
		if msg == "" {
			msg = claudeExitError(err)
		}
		c.emit.Emit("assistant:error", msg)
	}
}

// handleLine parses one NDJSON event and emits the matching assistant:* event.
// Text streams as tokens; tool_use bumps the tool counter; tool-result user
// events drive the thinking indicator; a result ends the turn. thinking blocks
// and benign system/rate_limit events are ignored.
func (c *claudeAssistant) handleLine(line []byte) {
	if len(line) == 0 {
		return
	}
	var event map[string]any
	if json.Unmarshal(line, &event) != nil {
		log.Printf("[assistant] non-json line: %.100s", line)
		return
	}
	switch event["type"] {
	case "assistant":
		c.handleAssistant(event)
	case "user":
		c.emit.Emit("assistant:thinking", c.toolCount())
	case "result":
		c.handleResult(event)
	case "error":
		c.setStreamErr(extractErrorMessage(event))
	case "system":
		if event["subtype"] == "error" {
			c.setStreamErr(extractErrorMessage(event))
		}
	}
}

func (c *claudeAssistant) handleAssistant(event map[string]any) {
	msg, ok := event["message"].(map[string]any)
	if !ok {
		return
	}
	content, ok := msg["content"].([]any)
	if !ok {
		return
	}
	for _, block := range content {
		cb, ok := block.(map[string]any)
		if !ok {
			continue
		}
		switch cb["type"] {
		case "text":
			if text, ok := cb["text"].(string); ok && text != "" {
				c.mu.Lock()
				c.emitted = true
				c.mu.Unlock()
				c.emit.Emit("assistant:token", text)
			}
		case "tool_use":
			n := c.bumpToolCount()
			if name, ok := cb["name"].(string); ok {
				c.emit.Emit("assistant:tool-use", name, n)
			}
		}
		// "thinking" blocks carry no user-facing surface and are ignored.
	}
}

func (c *claudeAssistant) handleResult(event map[string]any) {
	if sid, ok := event["session_id"].(string); ok && sid != "" {
		c.mu.Lock()
		c.sessionID = sid
		c.mu.Unlock()
	}
	if isErr, _ := event["is_error"].(bool); isErr {
		c.mu.Lock()
		interrupting := c.interrupting
		c.mu.Unlock()
		if interrupting {
			// We initiated this stop — end the turn cleanly, not as an error.
			c.endTurn()
			c.emit.Emit("assistant:done", "")
			return
		}
		msg := ""
		if text, ok := event["result"].(string); ok && text != "" {
			msg = text
		} else if sub, ok := event["subtype"].(string); ok && sub != "" {
			msg = friendlyResultSubtypeError(sub, c.maxTurns())
		} else {
			msg = c.takeStreamErr()
		}
		c.endTurn()
		c.emit.Emit("assistant:error", msg)
		return
	}
	c.mu.Lock()
	emitted := c.emitted
	c.mu.Unlock()
	if !emitted {
		if text, ok := event["result"].(string); ok && text != "" {
			c.emit.Emit("assistant:token", text)
		}
	}
	c.endTurn()
	c.emit.Emit("assistant:done", "")
}

// endTurn resets per-turn state under the lock. The process stays alive.
func (c *claudeAssistant) endTurn() {
	c.mu.Lock()
	c.streaming = false
	c.emitted = false
	c.tools = 0
	c.streamErr = ""
	c.interrupting = false
	c.mu.Unlock()
}

func (c *claudeAssistant) bumpToolCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tools++
	return c.tools
}

func (c *claudeAssistant) toolCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.tools
}

func (c *claudeAssistant) maxTurns() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sig.maxTurns
}

func (c *claudeAssistant) setStreamErr(msg string) {
	if msg == "" {
		return
	}
	c.mu.Lock()
	c.streamErr = msg
	c.mu.Unlock()
}

func (c *claudeAssistant) takeStreamErr() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	m := c.streamErr
	c.streamErr = ""
	return m
}

// Interrupt stops the in-flight turn by sending the stream-json interrupt
// control frame. The process and its warm prompt cache stay alive; the reader
// sees the resulting interrupted result and ends the turn cleanly. A no-op when
// no turn is in flight.
func (c *claudeAssistant) Interrupt() {
	c.mu.Lock()
	if c.proc == nil || !c.streaming {
		c.mu.Unlock()
		return
	}
	c.interrupting = true
	stdin := c.proc.stdin
	c.mu.Unlock()
	if _, err := stdin.Write([]byte(`{"type":"control_request","request_id":"int","request":{"subtype":"interrupt"}}` + "\n")); err != nil {
		log.Printf("[assistant] interrupt write failed: %v", err)
	}
}

// Reset discards the session so the next Send starts a brand-new conversation.
func (c *claudeAssistant) Reset() {
	c.mu.Lock()
	p := c.proc
	c.proc = nil
	c.sessionID = ""
	c.streaming = false
	c.mu.Unlock()
	waitProc(p)
}

// Close terminates the process (the session UUID is retained for a possible
// resume, but the conversation effectively ends when the app closes).
func (c *claudeAssistant) Close() {
	c.mu.Lock()
	p := c.proc
	c.proc = nil
	c.streaming = false
	c.mu.Unlock()
	waitProc(p)
}

// waitProc cancels a process and waits for its reader to exit, OUTSIDE c.mu so
// the reader can take the lock during cleanup.
func waitProc(p *claudeProc) {
	if p == nil {
		return
	}
	p.cancel()
	_ = p.stdin.Close()
	<-p.doneCh
}

func claudeArgs(sig launchSig, sessionID string, resume bool) []string {
	args := []string{
		"-p",
		"--input-format", "stream-json",
		"--output-format", "stream-json",
		"--verbose",
		"--system-prompt", sig.systemPrompt,
		"--max-turns", fmt.Sprintf("%d", sig.maxTurns),
	}
	if sig.mcpPort > 0 && sig.mcpToken != "" {
		args = append(args, mcpArgs(sig.mcpPort, sig.mcpToken)...)
	} else {
		args = append(args, "--tools", "")
	}
	if sig.model != "" {
		args = append(args, "--model", sig.model)
	}
	if sig.effort != "" {
		args = append(args, "--effort", sig.effort)
	}
	if resume {
		args = append(args, "--resume", sessionID)
	} else {
		args = append(args, "--session-id", sessionID)
	}
	return args
}

// mcpToolTimeoutMS is the per-tool-call timeout the Claude CLI (acting as our
// MCP client) applies while waiting for one of our MCP tools to return. The
// interactive tools — ask_user_question, request_permission,
// screenshot_viewport — block until the human responds, which has no natural
// upper bound, so this is set far beyond any real session. The CLI has no
// "infinite" setting, so an effectively-forever ceiling stands in for "never
// time out on user input".
const mcpToolTimeoutMS int64 = 365 * 24 * 60 * 60 * 1000 // 1 year

// mcpArgs builds the --mcp-config block pointing at our in-process MCP server.
func mcpArgs(port int, token string) []string {
	mcpCfg := map[string]any{
		"mcpServers": map[string]any{
			"facet": map[string]any{
				"type":    "http",
				"url":     fmt.Sprintf("http://127.0.0.1:%d/mcp", port),
				"headers": map[string]string{"Authorization": "Bearer " + token},
				"timeout": mcpToolTimeoutMS,
			},
		},
	}
	b, _ := json.Marshal(mcpCfg)
	return []string{
		"--mcp-config", string(b), "--strict-mcp-config",
		"--allowedTools", "mcp__facet__*",
		"--permission-prompt-tool", "mcp__facet__request_permission",
	}
}

// claudeEnv builds the child env: inherit ours, drop CLAUDECODE/CLAUDE_CODE so a
// nested Claude Code session doesn't crash, and set the long MCP tool timeout so
// interactive tools can block on the human. Reuses the package-level filterEnv
// and mcpToolTimeoutMS from assistant_service.go.
func claudeEnv() []string {
	env := filterEnv(os.Environ(), "CLAUDECODE", "CLAUDE_CODE", "MCP_TOOL_TIMEOUT")
	return append(env, fmt.Sprintf("MCP_TOOL_TIMEOUT=%d", mcpToolTimeoutMS))
}

// pidForTest returns the live process's PID, or 0 if none. Test-only accessor.
func (c *claudeAssistant) pidForTest() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.proc == nil || c.proc.cmd.Process == nil {
		return 0
	}
	return c.proc.cmd.Process.Pid
}

// stderrLogger forwards a child's stderr to the app log, one line at a time.
type stderrLogger struct{ buf []byte }

func newStderrLogger() io.Writer { return &stderrLogger{} }

func (s *stderrLogger) Write(p []byte) (int, error) {
	s.buf = append(s.buf, p...)
	for {
		i := bytes.IndexByte(s.buf, '\n')
		if i < 0 {
			break
		}
		log.Printf("[assistant] stderr: %s", s.buf[:i])
		s.buf = s.buf[i+1:]
	}
	return len(p), nil
}

func claudeExitError(err error) string {
	if err == nil {
		return "claude exited unexpectedly"
	}
	return fmt.Sprintf("claude exited: %v", err)
}

// friendlyResultSubtypeError maps a result-event subtype string to a
// user-facing sentence.  The CLI surfaces these as terse codes
// ("error_max_turns") that mean nothing to a user; translate the ones
// we know about and fall back to the raw code so unknown subtypes
// still reach the UI.
func friendlyResultSubtypeError(subtype string, maxTurns int) string {
	switch subtype {
	case "error_max_turns":
		return fmt.Sprintf("Assistant reached the max-turns limit (%d) without finishing. Send another message to continue the conversation, or raise the limit in Settings → AI Assistant → Max Turns.", maxTurns)
	default:
		return "result subtype: " + subtype
	}
}

// extractErrorMessage pulls a human-readable message out of a claude
// stream-json error/system event.  The CLI's error shape has drifted
// across versions — check the common fields, then fall back to the raw
// JSON so no information is lost.
func extractErrorMessage(event map[string]interface{}) string {
	if m, ok := event["message"].(string); ok && m != "" {
		return m
	}
	if m, ok := event["error"].(string); ok && m != "" {
		return m
	}
	if sub, ok := event["subtype"].(string); ok && sub != "" {
		return "error subtype: " + sub
	}
	if raw, err := json.Marshal(event); err == nil {
		return string(raw)
	}
	return "unknown error"
}

// newUUID returns a random RFC-4122 v4 UUID without external deps.
func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
