package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// AssistantMCPBridge exposes what AssistantService needs from the MCP layer:
// connection details so the Claude CLI can reach our in-process MCP server,
// and a way to latch the per-run editor context (code + active tab path +
// read-only flag) so MCP tools can read and enforce it. A nil bridge, or one
// whose Endpoint returns port=0, means MCP is unavailable — Claude falls back
// to a no-tools mode and generic CLIs are unaffected.
type AssistantMCPBridge interface {
	Endpoint() (port int, token string)
	SetContext(code, activeTabPath string, readOnly bool)
}

// AssistantService owns all state for the AI assistant panel: the chosen CLI
// config, the cached system prompt, the session ID used to resume Claude
// conversations, and the cancel func for the in-flight request.
type AssistantService struct {
	mu                 sync.Mutex
	cancel             context.CancelFunc
	sessionID          string
	config             AssistantConfig
	cachedSystemPrompt string

	// eventCtx is the app-lifetime context used for emitting assistant:*
	// events to the frontend. Set via SetEventContext at startup.
	eventCtx context.Context
}

// NewAssistantService creates an unstarted service. Call SetEventContext at
// startup before any Send/RebuildSystemPrompt call that emits events.
func NewAssistantService() *AssistantService {
	return &AssistantService{}
}

// SetEventContext stores the context used to emit "assistant:*" events.
func (s *AssistantService) SetEventContext(ctx context.Context) {
	s.eventCtx = ctx
}

// SetConfig stores the assistant configuration.
func (s *AssistantService) SetConfig(config AssistantConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = config
}

// GetDefaultSystemPrompt returns the cached system prompt, or builds one on
// demand if RebuildSystemPrompt has not yet been called.
func (s *AssistantService) GetDefaultSystemPrompt() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cachedSystemPrompt != "" {
		return s.cachedSystemPrompt
	}
	// Fallback: build on demand if not yet cached
	return buildFullSystemPrompt(nil)
}

// RebuildSystemPrompt rebuilds the cached AI system prompt from the language
// spec, curated examples, and library catalog. Safe to call from any
// goroutine.
func (s *AssistantService) RebuildSystemPrompt() {
	catalog := collectLibDocEntries()
	prompt := buildFullSystemPrompt(catalog)
	s.mu.Lock()
	s.cachedSystemPrompt = prompt
	s.mu.Unlock()
	log.Printf("[assistant] system prompt rebuilt (%d bytes)", len(prompt))
}

// Cancel cancels any in-flight assistant request.
func (s *AssistantService) Cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
}

// ClearHistory resets the conversation by clearing the session ID.
func (s *AssistantService) ClearHistory() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessionID = ""
}

// Send dispatches a user message through the configured CLI. A previous
// in-flight request (if any) is cancelled first. mcp may be nil when MCP is
// unavailable; Claude then runs in no-tools mode.
func (s *AssistantService) Send(userMessage, editorCode, errorsText, activeTabPath string, activeTabReadOnly bool, imagePaths []string, mcp AssistantMCPBridge) error {
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	config := s.config
	sessionID := s.sessionID
	cachedPrompt := s.cachedSystemPrompt
	s.mu.Unlock()

	cliID := config.CLI
	if cliID == "" {
		cliID = "claude"
	}

	// Find the CLI binary
	var binName string
	for _, cli := range knownCLIs {
		if cli.ID == cliID {
			binName = cli.Bin
			break
		}
	}
	if binName == "" {
		cancel()
		return fmt.Errorf("unknown CLI: %s", cliID)
	}
	binPath := findBinary(binName)
	if binPath == "" {
		cancel()
		return fmt.Errorf("%s CLI not found", binName)
	}

	mcpPort, mcpToken := 0, ""
	if mcp != nil {
		mcpPort, mcpToken = mcp.Endpoint()
	}

	sysPrompt := config.SystemPrompt
	if sysPrompt == "" {
		if cliID == "claude" && mcpPort > 0 {
			// MCP-enabled: short prompt, Claude fetches docs on-demand via tools
			sysPrompt = buildMCPSystemPrompt()
		} else {
			// Generic CLIs: full prompt with all docs inlined. Use cached
			// prompt if available, else build fresh (same fallback as
			// GetDefaultSystemPrompt).
			sysPrompt = cachedPrompt
			if sysPrompt == "" {
				sysPrompt = buildFullSystemPrompt(nil)
			}
		}
	}

	// Latch per-run context for MCP tools: editor code, active tab path, and
	// read-only flag. Latched at run start so mid-run tab switches in the UI
	// don't redirect edit_code/replace_code to a different tab.
	if mcp != nil {
		mcp.SetContext(editorCode, activeTabPath, activeTabReadOnly)
	}

	fullPrompt := buildPrompt(userMessage, editorCode, errorsText, imagePaths)

	log.Printf("[assistant] starting %s CLI, model=%q, images=%d, prompt length=%d",
		cliID, config.Model, len(imagePaths), len(fullPrompt))

	go func() {
		defer cancel()
		var err error
		if cliID == "claude" {
			var result streamResult
			result, err = s.runClaudeStream(ctx, binPath, fullPrompt, sessionID, imagePaths, config.Model, sysPrompt, mcpPort, mcpToken)
			// Store the session ID regardless of err: partial-failure
			// results (notably error_max_turns) still carry a valid
			// session_id, and dropping it would force the next user
			// message to start from scratch instead of continuing the
			// conversation the CLI already persisted.
			if result.sessionID != "" {
				s.mu.Lock()
				s.sessionID = result.sessionID
				s.mu.Unlock()
			}
		} else {
			err = s.runGenericCLIStream(ctx, cliID, binPath, fullPrompt, config.Model, sysPrompt)
		}
		if err != nil {
			log.Printf("[assistant] error: %v", err)
			if ctx.Err() != nil {
				return
			}
			wailsRuntime.EventsEmit(s.eventCtx, "assistant:error", err.Error())
			return
		}
		wailsRuntime.EventsEmit(s.eventCtx, "assistant:done", "")
	}()

	return nil
}

// runClaudeStream runs the claude CLI with --output-format stream-json and
// parses the NDJSON output line by line, emitting text deltas as they arrive.
func (s *AssistantService) runClaudeStream(ctx context.Context, binPath, prompt, sessionID string, imagePaths []string, model, sysPrompt string, mcpPort int, mcpToken string) (streamResult, error) {
	s.mu.Lock()
	maxTurns := s.config.MaxTurns
	s.mu.Unlock()
	if maxTurns <= 0 {
		maxTurns = 10
	}

	args := []string{
		"-p",
		"--output-format", "stream-json",
		"--verbose",
		"--system-prompt", sysPrompt,
		"--max-turns", fmt.Sprintf("%d", maxTurns),
	}

	// Connect to the in-process MCP server for tool use.  The MCP endpoint
	// requires a bearer token generated at server startup.
	if mcpPort > 0 && mcpToken != "" {
		mcpCfg := map[string]any{
			"mcpServers": map[string]any{
				"facet": map[string]any{
					"type":    "http",
					"url":     fmt.Sprintf("http://127.0.0.1:%d/mcp", mcpPort),
					"headers": map[string]string{"Authorization": "Bearer " + mcpToken},
				},
			},
		}
		mcpConfigBytes, err := json.Marshal(mcpCfg)
		if err != nil {
			return streamResult{}, fmt.Errorf("marshal mcp config: %w", err)
		}
		args = append(args, "--mcp-config", string(mcpConfigBytes), "--strict-mcp-config", "--allowedTools", "mcp__facet__*")
	} else {
		// No MCP: disable all tools so Claude responds with text only
		args = append(args, "--tools", "")
	}

	for _, img := range imagePaths {
		args = append(args, "--file", img)
	}

	if model != "" {
		args = append(args, "--model", model)
	}

	if sessionID != "" {
		args = append(args, "--resume", sessionID)
	}

	cmd := exec.CommandContext(ctx, binPath, args...)
	cmd.Stdin = strings.NewReader(prompt)
	cmd.Dir = os.TempDir() // Prevent access to user directories (Photos, Desktop, etc.)

	// Unset CLAUDECODE so a nested Claude Code session doesn't crash.
	cmd.Env = filterEnv(os.Environ(), "CLAUDECODE", "CLAUDE_CODE")

	log.Printf("[assistant] running: %s %v", binPath, args)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return streamResult{}, fmt.Errorf("failed to create stdout pipe: %v", err)
	}
	stderrR, err := cmd.StderrPipe()
	if err != nil {
		return streamResult{}, fmt.Errorf("failed to create stderr pipe: %v", err)
	}

	// Stream stderr in the background so we see errors immediately.  The
	// WaitGroup is load-bearing: cmd.Wait() only syncs Go's own stderr-copy
	// goroutine, not ours — without this sync, reading stderrBuf after Wait
	// races the scanner and can return an empty string even when stderr had
	// content (the "exit status 1" with no detail symptom).
	var stderrBuf strings.Builder
	var stderrWG sync.WaitGroup
	stderrWG.Add(1)
	go func() {
		defer stderrWG.Done()
		sc := bufio.NewScanner(stderrR)
		for sc.Scan() {
			line := sc.Text()
			stderrBuf.WriteString(line)
			stderrBuf.WriteByte('\n')
			log.Printf("[assistant] stderr: %s", line)
		}
	}()

	if err := cmd.Start(); err != nil {
		return streamResult{}, fmt.Errorf("failed to start claude: %v", err)
	}
	var result streamResult
	emittedAny := false
	toolCallCount := 0
	// streamErr captures error information reported inline on stdout —
	// the claude CLI emits error/system-error events and is_error=true
	// result events there, not on stderr.  Without harvesting these, a
	// non-zero exit with no stderr content degrades to the useless
	// "exit status 1" message.
	var streamErr string
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)

	for scanner.Scan() {
		if ctx.Err() != nil {
			break
		}
		line := scanner.Text()
		if line == "" {
			continue
		}

		var event map[string]interface{}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			log.Printf("[assistant] non-json line: %.100s", line)
			continue
		}

		eventType, _ := event["type"].(string)

		// --- Message-level events (Claude CLI stream-json format) ---
		// type="assistant": message with content blocks (text and/or tool_use)
		// type="user": tool results
		// type="result": final summary (has "result" text and session_id)
		// type="error" / type="system" subtype="error": stream-level errors

		switch eventType {
		case "assistant":
			if msg, ok := event["message"].(map[string]interface{}); ok {
				if content, ok := msg["content"].([]interface{}); ok {
					for _, block := range content {
						cb, ok := block.(map[string]interface{})
						if !ok {
							continue
						}
						switch cb["type"] {
						case "text":
							if text, ok := cb["text"].(string); ok && text != "" {
								emittedAny = true
								wailsRuntime.EventsEmit(s.eventCtx, "assistant:token", text)
							}
						case "tool_use":
							toolCallCount++
							if toolName, ok := cb["name"].(string); ok {
								wailsRuntime.EventsEmit(s.eventCtx, "assistant:tool-use", toolName, toolCallCount)
							}
						}
					}
				}
			}

		case "user":
			// Tool results returned — Claude will think about next step
			wailsRuntime.EventsEmit(s.eventCtx, "assistant:thinking", toolCallCount)

		case "result":
			// Final summary: session_id + (when the stream produced no
			// intermediate tokens) the result text itself, so the user
			// doesn't see an empty assistant turn for tool-only runs.
			if sid, ok := event["session_id"].(string); ok && sid != "" {
				result.sessionID = sid
				log.Printf("[assistant] session_id: %s", sid)
			}
			if isErr, _ := event["is_error"].(bool); isErr {
				if text, ok := event["result"].(string); ok && text != "" {
					streamErr = text
				} else if sub, ok := event["subtype"].(string); ok && sub != "" {
					streamErr = friendlyResultSubtypeError(sub, maxTurns)
				}
				log.Printf("[assistant] result is_error: %s", streamErr)
			} else if !emittedAny {
				if text, ok := event["result"].(string); ok && text != "" {
					wailsRuntime.EventsEmit(s.eventCtx, "assistant:token", text)
				}
			}

		case "error":
			streamErr = extractErrorMessage(event)
			log.Printf("[assistant] error event: %s", streamErr)

		case "system":
			if sub, _ := event["subtype"].(string); sub == "error" {
				streamErr = extractErrorMessage(event)
				log.Printf("[assistant] system error: %s", streamErr)
			}
			// Non-error system events (init, etc.) are silent by design.

		default:
			log.Printf("[assistant] unhandled event type %q: %.200s", eventType, line)
		}
	}
	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		return result, fmt.Errorf("claude: output read error: %v", err)
	}

	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return result, fmt.Errorf("claude was cancelled")
		}
		// Drain the stderr scanner before reading its buffer (see
		// stderrWG comment above).
		stderrWG.Wait()
		// Preference order: stream error (most specific — tells the user
		// *what* went wrong) → stderr text → raw exit status as last
		// resort.  The raw fallback was the only path before; now it
		// fires only when both the stream and stderr are empty.
		if streamErr != "" {
			return result, fmt.Errorf("claude error: %s", strings.TrimSpace(streamErr))
		}
		if errMsg := strings.TrimSpace(stderrBuf.String()); errMsg != "" {
			return result, fmt.Errorf("claude error: %s", errMsg)
		}
		return result, fmt.Errorf("claude error: %v", err)
	}
	// Even on success, drain stderr so the goroutine doesn't outlive
	// this call (the scanner exits when the pipe closes anyway, but
	// this makes the ordering explicit).
	stderrWG.Wait()

	return result, nil
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

// runGenericCLIStream runs a non-Claude AI CLI, pipes the prompt to stdin,
// and streams stdout text back as assistant tokens.
func (s *AssistantService) runGenericCLIStream(ctx context.Context, cliID, binPath, prompt, model, sysPrompt string) error {
	var args []string
	switch cliID {
	case "ollama":
		m := model
		if m == "" {
			m = "llama3"
		}
		args = []string{"run", m, "--nowordwrap"}
		if sysPrompt != "" {
			args = append(args, "--system", sysPrompt)
		}
	case "aichat":
		if model != "" {
			args = append(args, "-m", model)
		}
		if sysPrompt != "" {
			args = append(args, "-S", sysPrompt)
		}
	case "llm":
		if model != "" {
			args = append(args, "-m", model)
		}
		if sysPrompt != "" {
			args = append(args, "-s", sysPrompt)
		}
	case "chatgpt":
		if model != "" {
			args = append(args, "-m", model)
		}
		if sysPrompt != "" {
			args = append(args, "-p", sysPrompt)
		}
	case "qwen":
		// Qwen Code one-shot mode: prompt arrives via stdin, plain text out.
		// Unlike gemini-cli (which qwen-code forks), the system prompt has a
		// native flag — no env-var or temp-file dance needed.
		args = []string{"--output-format", "text"}
		if model != "" {
			args = append(args, "-m", model)
		}
		if sysPrompt != "" {
			args = append(args, "--system-prompt", sysPrompt)
		}
	default:
		return fmt.Errorf("unsupported CLI: %s", cliID)
	}

	cmd := exec.CommandContext(ctx, binPath, args...)
	cmd.Stdin = strings.NewReader(prompt)
	cmd.Dir = os.TempDir() // Prevent access to user directories (Photos, Desktop, etc.)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %v", err)
	}
	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start %s: %v", cliID, err)
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)
	for scanner.Scan() {
		if ctx.Err() != nil {
			break
		}
		line := scanner.Text()
		wailsRuntime.EventsEmit(s.eventCtx, "assistant:token", line+"\n")
	}
	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		return fmt.Errorf("%s: output read error: %v", cliID, err)
	}

	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("%s was cancelled", cliID)
		}
		errMsg := stderrBuf.String()
		if errMsg != "" {
			return fmt.Errorf("%s error: %s", cliID, strings.TrimSpace(errMsg))
		}
		return fmt.Errorf("%s error: %v", cliID, err)
	}

	return nil
}
