package main

import (
	"fmt"
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
	a.assistant.SetConfig(config)
}

// GetDefaultSystemPrompt returns the dynamically assembled system prompt.
func (a *App) GetDefaultSystemPrompt() string {
	return a.assistant.GetDefaultSystemPrompt()
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
	return a.assistant.Send(userMessage, editorCode, errors, imagePaths, a.mcp)
}

// CancelAssistant cancels any in-flight assistant request.
func (a *App) CancelAssistant() {
	a.assistant.Cancel()
}

// ClearAssistantHistory resets the conversation by clearing the session ID.
func (a *App) ClearAssistantHistory() {
	a.assistant.ClearHistory()
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

