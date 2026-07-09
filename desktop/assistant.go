package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"facet/pkg/fctlang/doc"
	"facet/share/docs"
	"facet/share/examples"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// AssistantConfig holds the user's chosen AI CLI, model, and effort.
type AssistantConfig struct {
	CLI          string `json:"cli"`
	Model        string `json:"model"`
	Effort       string `json:"effort"` // Claude CLI --effort level ("" = CLI default; low/medium/high/xhigh/max)
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
	{ID: "claude", Name: "Claude", Bin: "claude", Models: []string{"sonnet", "opus", "haiku", "fable"}, DefaultModel: "sonnet"},
	{ID: "ollama", Name: "Ollama", Bin: "ollama", Models: []string{"llama3", "codellama", "mistral", "deepseek-coder"}, DefaultModel: "llama3"},
	{ID: "aichat", Name: "AIChat", Bin: "aichat", Models: []string{}, DefaultModel: ""},
	{ID: "llm", Name: "LLM", Bin: "llm", Models: []string{}, DefaultModel: ""},
	{ID: "chatgpt", Name: "ChatGPT", Bin: "chatgpt", Models: []string{"gpt-4o", "gpt-4", "gpt-3.5-turbo"}, DefaultModel: "gpt-4o"},
	{ID: "qwen", Name: "Qwen Code", Bin: "qwen", Models: []string{"qwen3-coder-plus", "qwen3-coder-flash", "qwen-turbo", "qwen-plus", "qwen-max"}, DefaultModel: "qwen3-coder-plus"},
}

// enabledCLIs is the set of provider IDs currently usable. Only Claude is
// enabled today; the rest stay defined in knownCLIs and surface as "coming
// soon" in the settings picker. Re-enable a provider by adding its ID here.
var enabledCLIs = map[string]bool{"claude": true}

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

// DetectAssistantCLIs returns the enabled AI CLIs found on the system. The
// receiver method is the Wails RPC entry point; the testable logic lives in
// detectAssistantCLIs so the filter can be exercised with a fake probe.
func (a *App) DetectAssistantCLIs() []CLIInfo {
	return detectAssistantCLIs(findBinary, queryModels)
}

// detectAssistantCLIs is the package-level implementation of
// DetectAssistantCLIs. probe locates a binary by name (empty = not found);
// listModels returns the model list for a found binary (nil = unknown,
// callers fall back to the hardcoded cliDef list). Only providers in
// enabledCLIs are probed; disabled ones are reported separately by
// comingSoonCLIs.
func detectAssistantCLIs(probe func(string) string, listModels func(string, string) []string) []CLIInfo {
	var result []CLIInfo
	for _, cli := range knownCLIs {
		if !enabledCLIs[cli.ID] {
			continue
		}
		p := probe(cli.Bin)
		if p == "" {
			continue
		}
		models := listModels(cli.ID, p)
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
	return result
}

// ComingSoonCLIs returns the known providers that are not yet enabled, so the
// settings picker can show them as greyed-out "(coming soon)" entries. The
// receiver method is the Wails RPC entry point; the testable logic lives in
// comingSoonCLIs so unit tests don't need a fully-initialised App.
func (a *App) ComingSoonCLIs() []CLIInfo {
	return comingSoonCLIs()
}

// comingSoonCLIs is the package-level implementation of ComingSoonCLIs. Only
// ID and Name are populated — these CLIs are display-only until added to
// enabledCLIs.
func comingSoonCLIs() []CLIInfo {
	var result []CLIInfo
	for _, cli := range knownCLIs {
		if enabledCLIs[cli.ID] {
			continue
		}
		result = append(result, CLIInfo{ID: cli.ID, Name: cli.Name})
	}
	return result
}

// isKnownCLI reports whether id matches an entry in knownCLIs. Used by Send()
// to distinguish a stale-but-known provider (coerce to claude) from a
// genuinely unknown id (surface the binary-lookup error).
func isKnownCLI(id string) bool {
	for _, cli := range knownCLIs {
		if cli.ID == id {
			return true
		}
	}
	return false
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

// effortLevelsCache memoizes the parsed --effort levels: the claude binary
// doesn't change within a session, and re-running `claude --help` on every
// settings/panel open would add needless subprocess latency.
var (
	effortLevelsOnce  sync.Once
	effortLevelsCache []string
)

// GetAssistantEffortLevels returns the reasoning-effort levels the claude CLI
// advertises in its --help (e.g. low/medium/high/xhigh/max), so the UI doesn't
// hardcode them. Returns nil if the binary is missing or the help can't be
// parsed; the frontend then offers only the "Default" (no --effort) choice.
func (a *App) GetAssistantEffortLevels() []string {
	effortLevelsOnce.Do(func() {
		if p := findBinary("claude"); p != "" {
			effortLevelsCache = detectEffortLevels(p)
		}
	})
	return effortLevelsCache
}

// detectEffortLevels runs `claude --help` and parses the --effort flag's
// advertised levels. Separate from parseEffortLevels so the parser is testable
// without spawning the CLI.
func detectEffortLevels(binPath string) []string {
	out, err := exec.Command(binPath, "--help").CombinedOutput()
	if err != nil {
		return nil
	}
	return parseEffortLevels(string(out))
}

// parseEffortLevels extracts the comma-separated levels from the --effort line
// of `claude --help`, e.g. "--effort <level> ... (low, medium, high, xhigh,
// max)" → ["low","medium","high","xhigh","max"]. The list usually wraps onto
// the next help line, so it scans forward from "--effort" for the first
// parenthesised group.
func parseEffortLevels(help string) []string {
	i := strings.Index(help, "--effort")
	if i < 0 {
		return nil
	}
	rest := help[i:]
	open := strings.IndexByte(rest, '(')
	if open < 0 {
		return nil
	}
	closeIdx := strings.IndexByte(rest[open:], ')')
	if closeIdx < 0 {
		return nil
	}
	var levels []string
	for _, p := range strings.Split(rest[open+1:open+closeIdx], ",") {
		if p = strings.TrimSpace(p); p != "" {
			levels = append(levels, p)
		}
	}
	return levels
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
	path, err := wailsRuntime.OpenFileDialog(a.runtimeCtx(), wailsRuntime.OpenDialogOptions{
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
// Current editor code, errors, and active-tab metadata are included as
// context. activeTabPath + activeTabReadOnly are latched by the MCP layer
// for the lifetime of the request so edit_code / replace_code can reject
// edits to read-only files and tab switches mid-run don't leak to the wrong
// file. imagePaths is a list of image file paths to attach (Claude only).
func (a *App) SendAssistantMessage(userMessage, editorCode, errors, activeTabPath string, activeTabReadOnly bool, imagePaths []string) error {
	return a.assistant.Send(userMessage, editorCode, errors, activeTabPath, activeTabReadOnly, imagePaths, a.assistantBridge())
}

// assistantBridge composes what AssistantService needs: the /mcp connection
// details (port/token) from the HTTP server, and the editor-context latch from
// the MCP service. Composed here so neither service depends on the other.
type assistantBridge struct {
	http *HTTPServer
	mcp  *MCPService
}

func (a *App) assistantBridge() assistantBridge { return assistantBridge{http: a.http, mcp: a.mcp} }

func (b assistantBridge) Endpoint() (int, string)               { return b.http.Endpoint() }
func (b assistantBridge) SetContext(code, path string, ro bool) { b.mcp.SetContext(code, path, ro) }

// CancelAssistant cancels any in-flight assistant request.
func (a *App) CancelAssistant() {
	a.assistant.Cancel()
}

// ClearAssistantHistory resets the conversation by clearing the session ID
// and dropping any session-remembered tool permissions so a new conversation
// re-asks before reaching the web or other gated tools.
func (a *App) ClearAssistantHistory() {
	a.assistant.ClearHistory()
	a.mcp.ClearRememberedPermissions()
}

// AnswerAssistantQuestion routes a user's selections from the question
// card back to the in-flight ask_user_question MCP tool call identified
// by id. answers maps each question text to the chosen label; notes maps
// each question to optional free-text (from the "Other" option or a
// notes field). Returns an error if no call is waiting for that id.
func (a *App) AnswerAssistantQuestion(id string, answers map[string]string, notes map[string]string) error {
	return a.mcp.AnswerQuestion(id, answers, notes)
}

// AnswerToolPermission routes a user's Allow/Deny decision from the permission
// card back to the parked permission request in the MCP layer (the
// request_permission CLI bridge or fetch_url's self-gate).
func (a *App) AnswerToolPermission(id string, allow bool, remember bool) error {
	return a.mcp.AnswerPermission(id, allow, remember)
}

// DeliverViewportScreenshot delivers a captured viewport PNG back to
// the parked screenshot_viewport MCP tool call. dataURL is the canvas
// toDataURL output ("data:image/png;base64,..."); pass errMsg
// non-empty (with dataURL empty) to fail the tool when the frontend
// could not capture the frame.
func (a *App) DeliverViewportScreenshot(id, dataURL, errMsg string) error {
	return a.mcp.DeliverScreenshot(id, dataURL, errMsg)
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

	// Section 3b: Auto-generated stdlib API reference from parsed std.fct —
	// authoritative, never drifts from what the evaluator sees.
	if stdlibCatalog := formatStdlibCatalog(doc.BuildDocIndex("", nil)); stdlibCatalog != "" {
		sb.WriteString("\n\n")
		sb.WriteString(stdlibCatalog)
	}

	// Section 4: Curated examples
	sb.WriteString("\n\n## Example Programs\n\n")
	sb.WriteString("Below are working Facet programs demonstrating key features.\n")
	for _, name := range curatedExamples {
		data, err := examples.FS.ReadFile(name)
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

// buildDocumentationResponse assembles the response body for the
// get_documentation MCP tool. `section` narrows to one of
// "language"/"colors"/"stdlib"/"libraries" (empty = all). `query` filters
// stdlib and library entries by case-insensitive substring on Name; when set,
// only matching entries are returned and the non-catalog sections (language,
// colors) are omitted since they aren't searchable by name.
func buildDocumentationResponse(section, query string) string {
	wantAll := section == ""
	wantLang := wantAll || section == "language"
	wantColors := wantAll || section == "colors"
	wantStdlib := wantAll || section == "stdlib"
	wantLibs := wantAll || section == "libraries"

	// A query targets named entries — language/colors have no searchable names.
	if query != "" {
		wantLang = false
		wantColors = false
	}

	var sb strings.Builder
	if wantLang {
		sb.WriteString(docs.LanguageSpec)
	}
	if wantColors {
		if sb.Len() > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(docs.ColorGuide)
	}
	if wantStdlib {
		stdlibEntries := filterByQuery(doc.BuildDocIndex("", nil), query)
		if catalog := formatStdlibCatalog(stdlibEntries); catalog != "" {
			if sb.Len() > 0 {
				sb.WriteString("\n\n")
			}
			sb.WriteString(catalog)
		}
	}
	if wantLibs {
		libEntries := filterByQuery(collectLibDocEntries(), query)
		if catalog := formatLibraryCatalog(libEntries); catalog != "" {
			if sb.Len() > 0 {
				sb.WriteString("\n\n")
			}
			sb.WriteString(catalog)
		}
	}

	if sb.Len() == 0 && query != "" {
		return fmt.Sprintf("No documentation entries matched query %q.", query)
	}
	return sb.String()
}

// filterByQuery returns entries whose Name contains query (case-insensitive).
// Empty query returns the input unchanged.
func filterByQuery(entries []doc.DocEntry, query string) []doc.DocEntry {
	if query == "" {
		return entries
	}
	q := strings.ToLower(query)
	out := make([]doc.DocEntry, 0, len(entries))
	for _, e := range entries {
		if strings.Contains(strings.ToLower(e.Name), q) {
			out = append(out, e)
		}
	}
	return out
}

// formatStdlibCatalog renders stdlib DocEntries (those with Library == "") as a
// markdown API reference section. Functions, methods (grouped by receiver),
// types, and keywords each get their own subsection. Fields are omitted — the
// struct signature already lists them. Entries are included verbatim from the
// parsed stdlib source, so this is always in sync with what the evaluator sees.
func formatStdlibCatalog(entries []doc.DocEntry) string {
	var funcs, types, keywords []doc.DocEntry
	methodsByReceiver := map[string][]doc.DocEntry{}
	var receiverOrder []string
	for _, e := range entries {
		if e.Library != "" {
			continue
		}
		switch e.Kind {
		case "function":
			funcs = append(funcs, e)
		case "method":
			// e.Name is "Receiver.Method"
			recv := e.Name
			if idx := strings.IndexByte(recv, '.'); idx >= 0 {
				recv = recv[:idx]
			}
			if _, ok := methodsByReceiver[recv]; !ok {
				receiverOrder = append(receiverOrder, recv)
			}
			methodsByReceiver[recv] = append(methodsByReceiver[recv], e)
		case "type":
			types = append(types, e)
		case "keyword":
			keywords = append(keywords, e)
		}
	}
	if len(funcs) == 0 && len(types) == 0 && len(keywords) == 0 && len(methodsByReceiver) == 0 {
		return ""
	}

	sortByName := func(xs []doc.DocEntry) {
		sort.Slice(xs, func(i, j int) bool { return xs[i].Name < xs[j].Name })
	}
	sortByName(funcs)
	sortByName(types)
	sortByName(keywords)
	sort.Strings(receiverOrder)
	for _, recv := range receiverOrder {
		sortByName(methodsByReceiver[recv])
	}

	var sb strings.Builder
	sb.WriteString("## Stdlib API Reference (auto-generated)\n\n")
	sb.WriteString("Generated from `std.fct` source comments — always in sync with the evaluator.\n")

	if len(types) > 0 {
		sb.WriteString("\n### Types\n\n")
		for _, e := range types {
			writeDocEntry(&sb, e)
		}
	}
	if len(funcs) > 0 {
		sb.WriteString("\n### Functions\n\n")
		for _, e := range funcs {
			writeDocEntry(&sb, e)
		}
	}
	if len(receiverOrder) > 0 {
		sb.WriteString("\n### Methods\n")
		for _, recv := range receiverOrder {
			sb.WriteString("\n#### ")
			sb.WriteString(recv)
			sb.WriteString("\n\n")
			for _, e := range methodsByReceiver[recv] {
				writeDocEntry(&sb, e)
			}
		}
	}
	if len(keywords) > 0 {
		sb.WriteString("\n### Keywords\n\n")
		for _, e := range keywords {
			writeDocEntry(&sb, e)
		}
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
			writeDocEntry(&sb, e)
		}
	}
	return sb.String()
}

// writeDocEntry renders one doc entry as a Markdown bullet: the signature in
// backticks, then the first line of its doc comment.
func writeDocEntry(sb *strings.Builder, e doc.DocEntry) {
	sb.WriteString("- `")
	sb.WriteString(e.Signature)
	sb.WriteString("`")
	if e.Doc != "" {
		d := e.Doc
		if idx := strings.IndexByte(d, '\n'); idx >= 0 {
			d = d[:idx]
		}
		sb.WriteString(" — ")
		sb.WriteString(d)
	}
	sb.WriteByte('\n')
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
