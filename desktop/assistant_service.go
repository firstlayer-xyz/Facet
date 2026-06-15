package main

import (
	"context"
	"fmt"
	"log"
	"sync"
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

// AssistantService coordinates the AI assistant panel: it owns the chosen CLI
// config and cached system prompt, selects the matching Assistant backend, and
// delegates each turn to it. The backend is constructed lazily on the first
// Send and reused across turns; it is swapped only when the selected CLI
// changes.
type AssistantService struct {
	mu                 sync.Mutex
	current            Assistant
	currentCLI         string
	config             AssistantConfig
	cachedSystemPrompt string

	// eventCtx is the app-lifetime context used for emitting assistant:*
	// events to the frontend. Set via SetEventContext at startup.
	eventCtx context.Context

	// Test seams (default to the real implementations).
	newAssistant  func(cliID, binPath string, emit EventEmitter, mcp AssistantMCPBridge) Assistant
	resolveBinary func(name string) string
}

// NewAssistantService creates an unstarted service. Call SetEventContext at
// startup before any Send/RebuildSystemPrompt call that emits events.
func NewAssistantService() *AssistantService {
	return &AssistantService{newAssistant: defaultNewAssistant, resolveBinary: findBinary}
}

// defaultNewAssistant constructs the real backend for a resolved CLI id: the
// persistent Claude stream-json process for "claude", a one-shot subprocess
// runner for every other (enabled) CLI.
func defaultNewAssistant(cliID, binPath string, emit EventEmitter, mcp AssistantMCPBridge) Assistant {
	if cliID == "claude" {
		return newClaudeAssistant(emit, mcp, binPath)
	}
	return newGenericCLIAssistant(emit, cliID, binPath)
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

// Send resolves the configured CLI, constructs (or reuses) the matching
// Assistant backend, builds the per-turn config and Turn, and delegates. mcp
// may be nil when MCP is unavailable; Claude then runs in no-tools mode.
func (s *AssistantService) Send(userMessage, editorCode, errorsText, activeTabPath string, activeTabReadOnly bool, imagePaths []string, mcp AssistantMCPBridge) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cliID := resolveCLIID(s.config.CLI)
	binName, err := binNameFor(cliID)
	if err != nil {
		return err
	}
	binPath := s.resolveBinary(binName)
	if binPath == "" {
		return fmt.Errorf("%s CLI not found", binName)
	}
	if s.current == nil || s.currentCLI != cliID {
		if s.current != nil {
			s.current.Close()
		}
		s.current = s.newAssistant(cliID, binPath, newWailsEmitter(s.eventCtx), mcp)
		s.currentCLI = cliID
	}
	cfg := SessionConfig{
		Model:        s.config.Model,
		Effort:       s.config.Effort,
		MaxTurns:     orDefault(s.config.MaxTurns, 10),
		SystemPrompt: s.resolveSystemPromptLocked(cliID, mcp),
	}
	turn := Turn{
		UserMessage:       userMessage,
		EditorCode:        editorCode,
		ErrorsText:        errorsText,
		ActiveTabPath:     activeTabPath,
		ActiveTabReadOnly: activeTabReadOnly,
		ImagePaths:        imagePaths,
	}
	return s.current.Send(turn, cfg)
}

// Cancel stops the in-flight turn (Claude keeps its warm process; one-shot CLIs
// are killed).
func (s *AssistantService) Cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current != nil {
		s.current.Interrupt()
	}
}

// ClearHistory resets the conversation so the next Send starts fresh.
func (s *AssistantService) ClearHistory() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current != nil {
		s.current.Reset()
	}
}

// Shutdown terminates the live backend (call from the app's shutdown hook).
func (s *AssistantService) Shutdown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current != nil {
		s.current.Close()
		s.current = nil
		s.currentCLI = ""
	}
}

// resolveCLIID coerces an empty or disabled CLI selection to Claude (the only
// enabled provider). An unknown id is returned as-is so binNameFor surfaces an
// explicit error.
func resolveCLIID(cli string) string {
	if cli == "" {
		return "claude"
	}
	if isKnownCLI(cli) && !enabledCLIs[cli] {
		return "claude"
	}
	return cli
}

// binNameFor returns the binary name for an enabled CLI, or an error.
func binNameFor(cliID string) (string, error) {
	for _, cli := range knownCLIs {
		if cli.ID == cliID && enabledCLIs[cli.ID] {
			return cli.Bin, nil
		}
	}
	return "", fmt.Errorf("unknown CLI: %s", cliID)
}

func orDefault(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}

// resolveSystemPromptLocked picks the system prompt: an explicit user override
// wins; else the short MCP prompt when Claude has an MCP endpoint (docs fetched
// on demand via tools); else the full inlined prompt. Caller holds s.mu.
func (s *AssistantService) resolveSystemPromptLocked(cliID string, mcp AssistantMCPBridge) string {
	if s.config.SystemPrompt != "" {
		return s.config.SystemPrompt
	}
	mcpPort := 0
	if mcp != nil {
		mcpPort, _ = mcp.Endpoint()
	}
	if cliID == "claude" && mcpPort > 0 {
		return buildMCPSystemPrompt()
	}
	if s.cachedSystemPrompt != "" {
		return s.cachedSystemPrompt
	}
	return buildFullSystemPrompt(nil)
}
