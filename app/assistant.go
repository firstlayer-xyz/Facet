package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"facet/app/docs"
	"facet/app/pkg/fctlang/doc"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// AssistantConfig holds the user's chosen AI CLI and model.
type AssistantConfig struct {
	CLI          string `json:"cli"`
	Model        string `json:"model"`
	SystemPrompt string `json:"systemPrompt"`
	MaxTurns     int    `json:"maxTurns"` // max tool-use turns for Claude CLI (0 = default 10)
}

// CLIInfo describes a detected AI CLI for the frontend settings UI.
type CLIInfo struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Models       []string `json:"models"`
	DefaultModel string   `json:"defaultModel"`
}

type cliDef struct {
	ID           string
	Name         string
	Bin          string
	Models       []string
	DefaultModel string
}

var knownCLIs = []cliDef{
	{ID: "claude", Name: "Claude", Bin: "claude", Models: []string{"sonnet", "opus", "haiku"}, DefaultModel: "sonnet"},
	{ID: "ollama", Name: "Ollama", Bin: "ollama", Models: []string{"llama3", "codellama", "mistral", "deepseek-coder"}, DefaultModel: "llama3"},
	{ID: "aichat", Name: "AIChat", Bin: "aichat", Models: []string{}, DefaultModel: ""},
	{ID: "llm", Name: "LLM", Bin: "llm", Models: []string{}, DefaultModel: ""},
	{ID: "chatgpt", Name: "ChatGPT", Bin: "chatgpt", Models: []string{"gpt-4o", "gpt-4", "gpt-3.5-turbo"}, DefaultModel: "gpt-4o"},
}

// extraSearchDirs returns common binary directories not always in PATH.
func extraSearchDirs() []string {
	home, _ := os.UserHomeDir()
	var dirs []string

	switch runtime.GOOS {
	case "darwin":
		dirs = []string{
			"/usr/local/bin",
			"/opt/homebrew/bin",
			filepath.Join(home, ".local", "bin"),
			filepath.Join(home, ".cargo", "bin"),
			filepath.Join(home, "go", "bin"),
		}
		// npm global — check common locations
		if npmPrefix, err := exec.Command("npm", "prefix", "-g").Output(); err == nil {
			dirs = append(dirs, filepath.Join(strings.TrimSpace(string(npmPrefix)), "bin"))
		}
	case "linux":
		dirs = []string{
			"/usr/local/bin",
			"/snap/bin",
			filepath.Join(home, ".local", "bin"),
			filepath.Join(home, ".cargo", "bin"),
			filepath.Join(home, "go", "bin"),
		}
		if npmPrefix, err := exec.Command("npm", "prefix", "-g").Output(); err == nil {
			dirs = append(dirs, filepath.Join(strings.TrimSpace(string(npmPrefix)), "bin"))
		}
	case "windows":
		appData := os.Getenv("APPDATA")
		localAppData := os.Getenv("LOCALAPPDATA")
		dirs = []string{
			filepath.Join(home, ".cargo", "bin"),
			filepath.Join(home, "go", "bin"),
			filepath.Join(home, ".local", "bin"),
		}
		if appData != "" {
			dirs = append(dirs, filepath.Join(appData, "npm"))
		}
		if localAppData != "" {
			dirs = append(dirs, filepath.Join(localAppData, "Programs"))
		}
	}

	return dirs
}

// findBinary searches for a binary by name, first via PATH then in common directories.
// Returns the full path if found, empty string otherwise.
func findBinary(name string) string {
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	// Check common directories not in PATH
	for _, dir := range extraSearchDirs() {
		candidate := filepath.Join(dir, name)
		if runtime.GOOS == "windows" {
			// Try .exe extension on Windows
			for _, ext := range []string{".exe", ".cmd", ".bat"} {
				p := candidate + ext
				if info, err := os.Stat(p); err == nil && !info.IsDir() {
					return p
				}
			}
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

// DetectAssistantCLIs returns the list of AI CLIs found on the system.
func (a *App) DetectAssistantCLIs() []CLIInfo {
	var result []CLIInfo
	for _, cli := range knownCLIs {
		if p := findBinary(cli.Bin); p != "" {
			models := queryModels(cli.ID, p)
			if len(models) == 0 {
				models = cli.Models
			}
			result = append(result, CLIInfo{
				ID:           cli.ID,
				Name:         cli.Name,
				Models:       models,
				DefaultModel: cli.DefaultModel,
			})
		}
	}
	return result
}

// queryModels attempts to list available models for a CLI.
// Returns nil on failure (caller falls back to hardcoded list).
func queryModels(cliID, binPath string) []string {
	var cmd *exec.Cmd
	switch cliID {
	case "ollama":
		cmd = exec.Command(binPath, "list")
	case "aichat":
		cmd = exec.Command(binPath, "--list-models")
	case "llm":
		cmd = exec.Command(binPath, "models")
	case "chatgpt":
		cmd = exec.Command(binPath, "--list-models")
	default:
		return nil
	}

	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var models []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		switch cliID {
		case "ollama":
			// Skip header line; format: "NAME ID SIZE MODIFIED"
			if strings.HasPrefix(line, "NAME") {
				continue
			}
			// First field is the model name (may include :tag)
			if name := strings.Fields(line)[0]; name != "" {
				models = append(models, name)
			}
		case "llm":
			// Format: "OpenAI Chat: gpt-4o (aliases: 4o)"
			// Extract model ID between ": " and " ("
			if idx := strings.Index(line, ": "); idx >= 0 {
				rest := line[idx+2:]
				if end := strings.Index(rest, " ("); end >= 0 {
					rest = rest[:end]
				}
				if rest != "" {
					models = append(models, strings.TrimSpace(rest))
				}
			}
		default:
			// aichat, chatgpt: one model per line
			models = append(models, line)
		}
	}
	return models
}

// SetAssistantConfig stores the assistant configuration.
func (a *App) SetAssistantConfig(config AssistantConfig) {
	a.assistantMu.Lock()
	defer a.assistantMu.Unlock()
	a.assistantConfig = config
}

// GetDefaultSystemPrompt returns the dynamically assembled system prompt.
func (a *App) GetDefaultSystemPrompt() string {
	a.assistantMu.Lock()
	defer a.assistantMu.Unlock()
	if a.cachedSystemPrompt != "" {
		return a.cachedSystemPrompt
	}
	// Fallback: build on demand if not yet cached
	return buildFullSystemPrompt(nil)
}

// PickImageFile opens a native file dialog for selecting an image file
// and returns the chosen path (empty string if cancelled).
func (a *App) PickImageFile() (string, error) {
	path, err := wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Attach Image",
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "Images (*.png, *.jpg, *.jpeg, *.gif, *.webp)", Pattern: "*.png;*.jpg;*.jpeg;*.gif;*.webp"},
		},
	})
	if err != nil {
		return "", err
	}
	return path, nil
}

// SendAssistantMessage sends a user message via the configured AI CLI.
// Current editor code and errors are included as context.
// imagePaths is a list of image file paths to attach (Claude only).
func (a *App) SendAssistantMessage(userMessage string, editorCode string, errors string, imagePaths []string) error {
	a.assistantMu.Lock()
	if a.cancelAssistant != nil {
		a.cancelAssistant()
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.cancelAssistant = cancel
	config := a.assistantConfig
	sessionID := a.sessionID
	a.assistantMu.Unlock()

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

	sysPrompt := config.SystemPrompt
	if sysPrompt == "" {
		if cliID == "claude" && a.mcpPort > 0 {
			// MCP-enabled: short prompt, Claude fetches docs on-demand via tools
			sysPrompt = buildMCPSystemPrompt()
		} else {
			// Generic CLIs: full prompt with all docs inlined
			sysPrompt = a.GetDefaultSystemPrompt()
		}
	}

	// Store editor code for MCP tools to read
	if a.mcpState != nil {
		a.mcpState.setEditorCode(editorCode)
	}

	fullPrompt := buildPrompt(userMessage, editorCode, errors, imagePaths)

	log.Printf("[assistant] starting %s CLI, model=%q, images=%d, prompt length=%d",
		cliID, config.Model, len(imagePaths), len(fullPrompt))

	go func() {
		defer cancel()
		var err error
		if cliID == "claude" {
			var result streamResult
			result, err = a.runClaudeStream(ctx, binPath, fullPrompt, sessionID, imagePaths, config.Model, sysPrompt)
			if err == nil && result.sessionID != "" {
				a.assistantMu.Lock()
				a.sessionID = result.sessionID
				a.assistantMu.Unlock()
			}
		} else {
			err = a.runGenericCLIStream(ctx, cliID, binPath, fullPrompt, config.Model, sysPrompt)
		}
		if err != nil {
			log.Printf("[assistant] error: %v", err)
			if ctx.Err() != nil {
				return
			}
			wailsRuntime.EventsEmit(a.ctx, "assistant:error", err.Error())
			return
		}
		wailsRuntime.EventsEmit(a.ctx, "assistant:done", "")
	}()

	return nil
}

// CancelAssistant cancels any in-flight assistant request.
func (a *App) CancelAssistant() {
	a.assistantMu.Lock()
	defer a.assistantMu.Unlock()
	if a.cancelAssistant != nil {
		a.cancelAssistant()
		a.cancelAssistant = nil
	}
}

// ClearAssistantHistory resets the conversation by clearing the session ID.
func (a *App) ClearAssistantHistory() {
	a.assistantMu.Lock()
	defer a.assistantMu.Unlock()
	a.sessionID = ""
}

// curatedExamples lists the example filenames included in the AI system prompt.
// Selected for broad feature coverage: text/extrusion, constrained vars,
// CSG composition, symmetry/mirroring, mesh manipulation, and procedural generation.
var curatedExamples = []string{
	"Spiral Text.fct",
	"Bolt And Nut.fct",
	"Chess Pawn.fct",
	"Shark.fct",
}


// buildMCPSystemPrompt returns the short system prompt for MCP-enabled CLIs.
// Claude fetches docs on-demand via get_documentation.
func buildMCPSystemPrompt() string {
	return docs.AIPrompt
}

// buildFullSystemPrompt returns the full system prompt with all docs inlined,
// for generic CLIs that don't have MCP tool access.
func buildFullSystemPrompt(libEntries []doc.DocEntry) string {
	var sb strings.Builder

	// Section 1: Role, format instructions, and modeling strategies
	sb.WriteString(docs.AIPrompt)

	// Section 2: Color guide
	sb.WriteString("\n\n")
	sb.WriteString(docs.ColorGuide)

	// Section 3: Full language reference
	sb.WriteString("\n\n")
	sb.WriteString(docs.LanguageSpec)

	// Section 4: Curated examples
	sb.WriteString("\n\n## Example Programs\n\n")
	sb.WriteString("Below are working Facet programs demonstrating key features.\n")
	for _, name := range curatedExamples {
		data, err := examplesFS.ReadFile("examples/" + name)
		if err != nil {
			continue
		}
		title := strings.TrimSuffix(name, ".fct")
		sb.WriteString("\n### ")
		sb.WriteString(title)
		sb.WriteString("\n```facet\n")
		sb.Write(data)
		if len(data) > 0 && data[len(data)-1] != '\n' {
			sb.WriteByte('\n')
		}
		sb.WriteString("```\n")
	}

	// Section 5: Library catalog
	if catalog := formatLibraryCatalog(libEntries); catalog != "" {
		sb.WriteString("\n")
		sb.WriteString(catalog)
	}

	return sb.String()
}

// formatLibraryCatalog renders a list of doc entries as a markdown library catalog.
// Returns an empty string if there are no entries.
func formatLibraryCatalog(libEntries []doc.DocEntry) string {
	if len(libEntries) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Available Libraries\n\n")
	sb.WriteString("Users can import these libraries with `var X = lib \"<import path>\";` then call `X.Function(...)`.\n")

	type libGroup struct {
		importPath string
		entries    []doc.DocEntry
	}
	orderKeys := []string{}
	groups := map[string]*libGroup{}
	for _, e := range libEntries {
		if e.Library == "" {
			continue
		}
		g, ok := groups[e.Library]
		if !ok {
			g = &libGroup{importPath: gitCacheNSToImportPath(e.Library)}
			groups[e.Library] = g
			orderKeys = append(orderKeys, e.Library)
		}
		g.entries = append(g.entries, e)
	}
	for _, ns := range orderKeys {
		g := groups[ns]
		displayName := ns
		if idx := strings.LastIndex(ns, "/"); idx >= 0 {
			displayName = ns[idx+1:]
		}
		sb.WriteString("\n### ")
		sb.WriteString(displayName)
		sb.WriteByte('\n')
		if g.importPath != "" {
			sb.WriteString("Import: `var X = lib \"")
			sb.WriteString(g.importPath)
			sb.WriteString("\";`\n")
		}
		for _, e := range g.entries {
			sb.WriteString("- `")
			sb.WriteString(e.Signature)
			sb.WriteString("`")
			if e.Doc != "" {
				sb.WriteString(" — ")
				d := e.Doc
				if idx := strings.IndexByte(d, '\n'); idx >= 0 {
					d = d[:idx]
				}
				sb.WriteString(d)
			}
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

// gitCacheNSToImportPath converts a library namespace derived from the git cache
// directory layout (e.g. "github.com/user/repo/branch/libname") into a user-facing
// import path (e.g. "github.com/user/repo/libname@branch").
// For built-in library namespaces (e.g. "facet/gears") it returns the path unchanged.
func gitCacheNSToImportPath(ns string) string {
	parts := strings.Split(ns, "/")
	// Built-in libs: "facet/gears" → no @ needed
	if len(parts) < 4 || !strings.Contains(parts[0], ".") {
		return ns
	}
	// Cache layout: host/user/repo/branch[/subpath...]
	host := parts[0]
	user := parts[1]
	repo := parts[2]
	branch := parts[3]
	if len(parts) > 4 {
		subpath := strings.Join(parts[4:], "/")
		return fmt.Sprintf("%s/%s/%s/%s@%s", host, user, repo, subpath, branch)
	}
	return fmt.Sprintf("%s/%s/%s@%s", host, user, repo, branch)
}

func buildPrompt(userMessage, editorCode, errors string, imagePaths []string) string {
	var sb strings.Builder
	sb.WriteString(userMessage)

	if editorCode != "" {
		sb.WriteString("\n\n---\nCurrent editor code:\n```facet\n")
		sb.WriteString(editorCode)
		sb.WriteString("\n```")
	}

	if errors != "" {
		sb.WriteString("\n\nCurrent errors:\n```\n")
		sb.WriteString(errors)
		sb.WriteString("\n```")
	}

	if len(imagePaths) > 0 {
		sb.WriteString("\n\nAttached: viewport screenshot of the current rendered result.")
	}

	return sb.String()
}

type streamResult struct {
	sessionID string
}

// extractTextDelta tries to find a text delta in a stream-json event.
// Handles both nested (.event.delta.text) and flat (.delta.text) structures.
func extractTextDelta(event map[string]interface{}) string {
	// Try .event.delta (nested stream_event wrapper)
	if inner, ok := event["event"].(map[string]interface{}); ok {
		if delta, ok := inner["delta"].(map[string]interface{}); ok {
			if dt, _ := delta["type"].(string); dt == "text_delta" {
				if text, ok := delta["text"].(string); ok {
					return text
				}
			}
		}
	}
	// Try .delta directly (flat structure)
	if delta, ok := event["delta"].(map[string]interface{}); ok {
		if dt, _ := delta["type"].(string); dt == "text_delta" {
			if text, ok := delta["text"].(string); ok {
				return text
			}
		}
	}
	return ""
}

// filterEnv returns a copy of env with any entries whose key matches one of
// the given keys removed. Keys are matched case-sensitively against the
// portion before the first '='.
func filterEnv(env []string, keys ...string) []string {
	out := make([]string, 0, len(env))
	for _, e := range env {
		key := e
		if i := strings.IndexByte(e, '='); i >= 0 {
			key = e[:i]
		}
		skip := false
		for _, k := range keys {
			if key == k {
				skip = true
				break
			}
		}
		if !skip {
			out = append(out, e)
		}
	}
	return out
}

// runClaudeStream runs the claude CLI with --output-format stream-json and
// parses the NDJSON output line by line, emitting text deltas as they arrive.
func (a *App) runClaudeStream(ctx context.Context, binPath, prompt, sessionID string, imagePaths []string, model, sysPrompt string) (streamResult, error) {
	maxTurns := a.assistantConfig.MaxTurns
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

	// Connect to the in-process MCP server for tool use
	if a.mcpPort > 0 {
		mcpConfig := fmt.Sprintf(`{"mcpServers":{"facet":{"type":"http","url":"http://127.0.0.1:%d/mcp"}}}`, a.mcpPort)
		args = append(args, "--mcp-config", mcpConfig, "--strict-mcp-config", "--allowedTools", "mcp__facet__*")
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
		s := bufio.NewScanner(stderrR)
		for s.Scan() {
			line := s.Text()
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
								wailsRuntime.EventsEmit(a.ctx, "assistant:token", text)
							}
						case "tool_use":
							toolCallCount++
							if toolName, ok := cb["name"].(string); ok {
								wailsRuntime.EventsEmit(a.ctx, "assistant:tool-use", toolName, toolCallCount)
							}
						}
					}
				}
			}
			continue
		}

		// type="user": tool results returned — Claude will think about next step
		if eventType == "user" {
			wailsRuntime.EventsEmit(a.ctx, "assistant:thinking", toolCallCount)
			continue
		}

		// --- Legacy streaming delta format (older Claude CLI versions) ---
		if eventType == "content_block_start" {
			if cb, ok := event["content_block"].(map[string]interface{}); ok {
				if cb["type"] == "tool_use" {
					toolCallCount++
					if toolName, ok := cb["name"].(string); ok {
						wailsRuntime.EventsEmit(a.ctx, "assistant:tool-use", toolName, toolCallCount)
					}
				}
			}
		}
		if text := extractTextDelta(event); text != "" {
			emittedAny = true
			wailsRuntime.EventsEmit(a.ctx, "assistant:token", text)
			continue
		}

		// --- Result event: session_id and fallback text ---
		if _, hasResult := event["result"]; hasResult {
			if sid, ok := event["session_id"].(string); ok && sid != "" {
				result.sessionID = sid
				log.Printf("[assistant] session_id: %s", sid)
			}
			if !emittedAny {
				if text, ok := event["result"].(string); ok && text != "" {
					wailsRuntime.EventsEmit(a.ctx, "assistant:token", text)
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
func (a *App) runGenericCLIStream(ctx context.Context, cliID, binPath, prompt, model, sysPrompt string) error {
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
		wailsRuntime.EventsEmit(a.ctx, "assistant:token", line+"\n")
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
