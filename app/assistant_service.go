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
// and a way to publish the current editor code so MCP tools can read it. A
// nil bridge, or one whose Endpoint returns port=0, means MCP is unavailable
// — Claude falls back to a no-tools mode and generic CLIs are unaffected.
type AssistantMCPBridge interface {
	Endpoint() (port int, token string)
	SetEditorCode(code string)
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
func (s *AssistantService) Send(userMessage, editorCode, errorsText string, imagePaths []string, mcp AssistantMCPBridge) error {
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

	// Store editor code for MCP tools to read
	if mcp != nil {
		mcp.SetEditorCode(editorCode)
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
			if err == nil && result.sessionID != "" {
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

	// Stream stderr in the background so we see errors immediately
	var stderrBuf strings.Builder
	go func() {
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

		if eventType == "assistant" {
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
			continue
		}

		// type="user": tool results returned — Claude will think about next step
		if eventType == "user" {
			wailsRuntime.EventsEmit(s.eventCtx, "assistant:thinking", toolCallCount)
			continue
		}

		// Result event: session_id and, when the stream produced no
		// intermediate tokens, the result text itself — otherwise the
		// user would see an empty assistant turn for completions that
		// only emit a final summary (e.g. tool-only runs).
		if _, hasResult := event["result"]; hasResult {
			if sid, ok := event["session_id"].(string); ok && sid != "" {
				result.sessionID = sid
				log.Printf("[assistant] session_id: %s", sid)
			}
			if !emittedAny {
				if text, ok := event["result"].(string); ok && text != "" {
					wailsRuntime.EventsEmit(s.eventCtx, "assistant:token", text)
				}
			}
		}
	}
	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		return result, fmt.Errorf("claude: output read error: %v", err)
	}

	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return result, fmt.Errorf("claude was cancelled")
		}
		errMsg := stderrBuf.String()
		if errMsg != "" {
			return result, fmt.Errorf("claude error: %s", strings.TrimSpace(errMsg))
		}
		return result, fmt.Errorf("claude error: %v", err)
	}

	return result, nil
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
