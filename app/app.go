package main

import (
	"bufio"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"

	"facet/app/pkg/fctlang/formatter"
	"facet/app/pkg/fctlang/parser"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// libraryDir returns the OS-specific library directory for user-installed libraries.
func libraryDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine config directory: %w", err)
	}
	return filepath.Join(base, "Facet", "libraries"), nil
}

// scratchDir returns the directory for unsaved scratch files.
func scratchDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine config directory: %w", err)
	}
	return filepath.Join(base, "Facet", "scratch"), nil
}

//go:embed examples
var examplesFS embed.FS

//go:embed "examples/Tutorial.fct"
var defaultSource string

//go:embed docs
var docsFS embed.FS

func (a *App) GetDefaultSource() string {
	return defaultSource
}

// GetExampleList returns the names of all embedded example files.
func (a *App) GetExampleList() []string {
	entries, err := examplesFS.ReadDir("examples")
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

// GetExample returns the source code of the named embedded example.
func (a *App) GetExample(name string) (string, error) {
	if strings.Contains(name, "..") || strings.ContainsAny(name, "/\\") {
		return "", fmt.Errorf("invalid example name: %s", name)
	}
	data, err := examplesFS.ReadFile(path.Join("examples", name))
	if err != nil {
		return "", fmt.Errorf("example not found: %s", name)
	}
	return string(data), nil
}

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

// App struct holds the application state and is bound to the frontend via Wails.
type App struct {
	ctx context.Context

	configMu sync.Mutex

	assistantMu sync.Mutex
	cancelAssistant    context.CancelFunc
	sessionID          string
	assistantConfig    AssistantConfig
	cachedSystemPrompt string

	mcpState *mcpState // MCP server state (tools read/write this)
	mcpPort  int       // port the MCP/eval HTTP server is listening on

	evalMu     sync.Mutex
	cancelEval context.CancelFunc

	stderrBuf *ringBuffer
	logFile   *os.File
}

// NewApp creates a new App application struct.
func NewApp() *App {
	return &App{}
}

// appConfig holds all application settings persisted to disk.
// Frontend-owned sections use json.RawMessage so Go round-trips them without
// needing to duplicate the full type definitions — the frontend owns the schema.
type appConfig struct {
	MemoryLimitGB   int64              `json:"memoryLimitGB"`              // 0 = default (8 GB)
	InstalledLibs   map[string]string  `json:"installedLibs,omitempty"`    // libID → local dir overrides
	RecentFiles     []string           `json:"recentFiles,omitempty"`      // most-recently-opened file paths
	SavedTabs       json.RawMessage    `json:"savedTabs,omitempty"`        // frontend-owned tab state
	ActiveTab       string             `json:"activeTab,omitempty"`        // frontend-owned active tab path
	Appearance      json.RawMessage    `json:"appearance,omitempty"`       // frontend-owned
	Editor          json.RawMessage    `json:"editor,omitempty"`           // frontend-owned
	Assistant       json.RawMessage    `json:"assistant,omitempty"`        // frontend-owned
	Camera          json.RawMessage    `json:"camera,omitempty"`           // frontend-owned
	Slicer          json.RawMessage    `json:"slicer,omitempty"`           // frontend-owned
	LibrarySettings json.RawMessage    `json:"librarySettings,omitempty"`  // frontend-owned (autoPull, etc.)
}

const defaultMemoryLimitGB = 8

// configDir returns the OS-specific Facet config directory.
func configDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		base = os.TempDir()
	}
	return filepath.Join(base, "Facet")
}

// configPath returns the path to the backend settings file.
func configPath() string {
	return filepath.Join(configDir(), "settings.json")
}

func loadConfig() appConfig {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return appConfig{}
	}
	var cfg appConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Printf("loadConfig: unmarshal error: %v", err)
		return appConfig{}
	}
	return cfg
}

func saveConfig(cfg appConfig) {
	dir := configDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("saveConfig: mkdir error: %v", err)
		return
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		log.Printf("saveConfig: marshal error: %v", err)
		return
	}
	if err := os.WriteFile(configPath(), data, 0644); err != nil {
		log.Printf("saveConfig: write error: %v", err)
	}
}

// GetSettings returns the full settings JSON for the frontend.
func (a *App) GetSettings() string {
	cfg := loadConfig()
	data, err := json.Marshal(cfg)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// PatchSettings merges the provided partial JSON into the existing config.
// Only keys present in the patch are updated; missing keys are preserved.
// This is the primary way both frontend and Go code should update settings.
func (a *App) PatchSettings(jsonStr string) {
	a.configMu.Lock()
	defer a.configMu.Unlock()

	// Read existing config as raw JSON
	existing, _ := os.ReadFile(configPath())
	if existing == nil {
		existing = []byte("{}")
	}

	// Parse both into generic maps
	var base map[string]json.RawMessage
	if err := json.Unmarshal(existing, &base); err != nil {
		base = make(map[string]json.RawMessage)
	}

	var patch map[string]json.RawMessage
	if err := json.Unmarshal([]byte(jsonStr), &patch); err != nil {
		fmt.Fprintf(os.Stderr, "PatchSettings: bad JSON: %v\n", err)
		return
	}

	// Merge: patch keys override base keys
	for k, v := range patch {
		base[k] = v
	}

	// Write merged result back through appConfig for validation
	merged, err := json.Marshal(base)
	if err != nil {
		fmt.Fprintf(os.Stderr, "PatchSettings: marshal error: %v\n", err)
		return
	}
	var cfg appConfig
	if err := json.Unmarshal(merged, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "PatchSettings: unmarshal error: %v\n", err)
		return
	}
	saveConfig(cfg)
}

func applyMemoryLimit(gb int64) {
	if gb <= 0 {
		gb = defaultMemoryLimitGB
	}
	debug.SetMemoryLimit(gb * 1024 * 1024 * 1024)
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	cfg := loadConfig()
	applyMemoryLimit(cfg.MemoryLimitGB)

	a.rebuildSystemPrompt()
	a.initStderrCapture()

	// Start in-process HTTP server (MCP + eval endpoints)
	port, err := a.startHTTPServer()
	if err != nil {
		log.Printf("[http] failed to start: %v", err)
	} else {
		a.mcpPort = port
	}

	// Auto-pull libraries on startup if enabled
	if autoPull := parseAutoPull(cfg.LibrarySettings); autoPull {
		go func() {
			fmt.Fprintln(os.Stderr, "auto-pull: pulling all libraries...")
			if err := a.PullAllLibraries(); err != nil {
				fmt.Fprintf(os.Stderr, "auto-pull: %v\n", err)
			} else {
				fmt.Fprintln(os.Stderr, "auto-pull: done")
			}
		}()
	}
}

// parseAutoPull extracts the autoPull boolean from the librarySettings JSON.
func parseAutoPull(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var ls struct {
		AutoPull bool `json:"autoPull"`
	}
	if err := json.Unmarshal(raw, &ls); err != nil {
		return false
	}
	return ls.AutoPull
}

// hasDirtyFiles is set by the frontend whenever the dirty state changes.
// Checked by beforeClose to prompt for unsaved changes.
var hasDirtyFiles bool

// SetDirtyState is called by the frontend to report whether any files have unsaved changes.
func (a *App) SetDirtyState(dirty bool) {
	hasDirtyFiles = dirty
}

// beforeClose is called when the user tries to close the window.
// Returns true to prevent closing (user chose to cancel).
func (a *App) beforeClose(ctx context.Context) bool {
	// Emit event so frontend can persist tab state
	wailsRuntime.EventsEmit(ctx, "app:before-close")

	if !hasDirtyFiles {
		return false // allow close
	}
	result, err := wailsRuntime.MessageDialog(ctx, wailsRuntime.MessageDialogOptions{
		Type:          wailsRuntime.QuestionDialog,
		Title:         "Unsaved Changes",
		Message:       "You have unsaved changes. Quit anyway?",
		DefaultButton: "No",
		Buttons:       []string{"Yes", "No"},
	})
	if err != nil {
		return false // allow close on error
	}
	return result != "Yes" // true = prevent close
}

// shutdown is called when the app is closing. Persists current memory limit.
func (a *App) shutdown(ctx context.Context) {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	cfg := loadConfig()
	cfg.MemoryLimitGB = a.GetMemoryLimit()
	saveConfig(cfg)
	cleanupScratchFiles()
	if a.logFile != nil {
		a.logFile.Close()
	}
}

// cleanupScratchFiles deletes empty (0-byte) scratch files on shutdown.
func cleanupScratchFiles() {
	dir, err := scratchDir()
	if err != nil {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".fct") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.Size() == 0 {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

// rebuildSystemPrompt rebuilds the cached AI system prompt from the language
// spec, curated examples, and library catalog. Safe to call from any goroutine.
func (a *App) rebuildSystemPrompt() {
	catalog := buildLibraryCatalog()
	prompt := buildSystemPrompt(catalog)
	a.assistantMu.Lock()
	a.cachedSystemPrompt = prompt
	a.assistantMu.Unlock()
	log.Printf("[assistant] system prompt rebuilt (%d bytes)", len(prompt))
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

// GetLogDir returns the path to the logs directory.
func (a *App) GetLogDir() string {
	return logDir()
}

// initStderrCapture redirects stderr to a pipe, tees to the original stderr,
// a ring buffer, and a log file. Emits "log:stderr" events for each line.
func (a *App) initStderrCapture() {
	a.stderrBuf = &ringBuffer{max: 256 * 1024} // 256 KB ring buffer

	// Create log directory and rotate old logs
	dir := logDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("initStderrCapture: mkdir error: %v", err)
	}
	rotateOldLogs()

	// Open today's log file (append mode)
	logName := time.Now().Format("2006-01-02") + ".log"
	logFile, fileErr := os.OpenFile(filepath.Join(dir, logName), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if fileErr == nil {
		a.logFile = logFile
	}

	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		return
	}
	os.Stderr = w

	// Tee pipe output to original stderr + ring buffer + log file
	writers := []io.Writer{origStderr, a.stderrBuf}
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
			if a.ctx != nil {
				wailsRuntime.EventsEmit(a.ctx, "log:stderr", line)
			}
		}
	}()

	// Close the write end of the pipe when the app context is cancelled,
	// which causes the scanner goroutine above to exit.
	go func() {
		<-a.ctx.Done()
		w.Close()
	}()
}

// GetStderrLog returns the current stderr buffer contents.
func (a *App) GetStderrLog() string {
	if a.stderrBuf == nil {
		return ""
	}
	return a.stderrBuf.String()
}

// MemStats returns a summary of Go runtime memory usage.
func (a *App) MemStats() map[string]uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return map[string]uint64{
		"heapAlloc":      m.HeapAlloc,      // bytes in use by Go objects
		"heapSys":        m.HeapSys,        // bytes obtained from OS for heap
		"heapIdle":       m.HeapIdle,       // bytes in idle spans (could be released)
		"heapReleased":   m.HeapReleased,   // bytes released to OS
		"sys":            m.Sys,            // total bytes obtained from OS
		"externalMemory": m.ExternalMemory, // bytes tracked via ExternalAlloc (CGo/manifold)
		"numGC":          uint64(m.NumGC),
	}
}

// RunGC triggers a garbage collection cycle.
func (a *App) RunGC() {
	runtime.GC()
}

// SetMemoryLimit sets the Go runtime soft memory limit in GB and persists it.
// 0 means use the default (8 GB).
func (a *App) SetMemoryLimit(gb int64) {
	applyMemoryLimit(gb)
	a.configMu.Lock()
	defer a.configMu.Unlock()
	cfg := loadConfig()
	cfg.MemoryLimitGB = gb
	saveConfig(cfg)
}

// GetMemoryLimit returns the current soft memory limit in GB (0 = default).
func (a *App) GetMemoryLimit() int64 {
	limit := debug.SetMemoryLimit(-1) // -1 reads without changing
	if limit == math.MaxInt64 {
		return 0
	}
	return limit / (1024 * 1024 * 1024)
}

const maxRecentFiles = 10

// AddRecentFile records path as the most recently opened file.
func (a *App) AddRecentFile(path string) {
	if path == "" {
		return
	}
	a.configMu.Lock()
	defer a.configMu.Unlock()
	cfg := loadConfig()
	// Remove existing occurrence, then prepend.
	filtered := make([]string, 0, len(cfg.RecentFiles))
	for _, p := range cfg.RecentFiles {
		if p != path {
			filtered = append(filtered, p)
		}
	}
	cfg.RecentFiles = append([]string{path}, filtered...)
	if len(cfg.RecentFiles) > maxRecentFiles {
		cfg.RecentFiles = cfg.RecentFiles[:maxRecentFiles]
	}
	saveConfig(cfg)
}

// CreateScratchFile creates a new empty .fct file in the scratch directory
// and returns its absolute path. The file persists across crashes for recovery.
func (a *App) CreateScratchFile(name string) (string, error) {
	dir, err := scratchDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, name+".fct")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		return "", err
	}
	return path, nil
}

// IsScratchFile returns true if the path is inside the scratch directory.
func (a *App) IsScratchFile(path string) bool {
	dir, _ := scratchDir()
	return dir != "" && strings.HasPrefix(path, dir)
}

func (a *App) OpenRecentFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return map[string]string{"path": path, "source": string(data)}, nil
}

// SetWindowTitle updates the native window title.
func (a *App) SetWindowTitle(title string) {
	wailsRuntime.WindowSetTitle(a.ctx, title)
}

var facetFilter = wailsRuntime.FileFilter{
	DisplayName: "Facet Files (*.fct)",
	Pattern:     "*.fct",
}

// ConfirmDiscard shows a native Yes/No dialog asking about unsaved changes.
// Returns true if the user chose to discard (Yes) or false to cancel.
func (a *App) ConfirmDiscard() (bool, error) {
	result, err := wailsRuntime.MessageDialog(a.ctx, wailsRuntime.MessageDialogOptions{
		Type:          wailsRuntime.QuestionDialog,
		Title:         "Unsaved Changes",
		Message:       "You have unsaved changes. Discard and continue?",
		DefaultButton: "No",
		Buttons:       []string{"Yes", "No"},
	})
	if err != nil {
		return false, err
	}
	return result == "Yes", nil
}

// OpenFile shows a native file dialog and returns the file contents and path.
func (a *App) OpenFile() (map[string]string, error) {
	path, err := wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Filters: []wailsRuntime.FileFilter{facetFilter},
	})
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, nil // user cancelled
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return map[string]string{"path": path, "source": string(data)}, nil
}

// SaveFile saves source to the given path. If path is empty, shows a save dialog.
func (a *App) SaveFile(source string, path string) (string, error) {
	if path == "" {
		var err error
		path, err = wailsRuntime.SaveFileDialog(a.ctx, wailsRuntime.SaveDialogOptions{
			DefaultFilename: "untitled.fct",
			Filters:         []wailsRuntime.FileFilter{facetFilter},
		})
		if err != nil {
			return "", err
		}
		if path == "" {
			return "", nil // user cancelled
		}
	}
	if err := os.WriteFile(path, []byte(source), 0644); err != nil {
		return "", err
	}
	return path, nil
}

// FormatCode normalizes the indentation of Facet source code.
func (a *App) FormatCode(source string) string {
	src, err := parser.Parse(source, "", parser.SourceUser)
	if err != nil {
		return source
	}
	return formatter.Format(src)
}

// DocGuide is a JSON-serializable guide document for the frontend.
type DocGuide struct {
	Title    string `json:"title"`
	Slug     string `json:"slug"`
	Markdown string `json:"markdown"`
}

// GetDocGuides returns the embedded markdown guide documents.
func (a *App) GetDocGuides() []DocGuide {
	entries, err := docsFS.ReadDir("docs")
	if err != nil {
		return nil
	}
	var guides []DocGuide
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := docsFS.ReadFile("docs/" + e.Name())
		if err != nil {
			continue
		}
		src := string(data)
		slug := strings.TrimSuffix(e.Name(), ".md")
		title := slug
		for _, line := range strings.Split(src, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "# ") {
				title = strings.TrimPrefix(trimmed, "# ")
				break
			}
		}
		guides = append(guides, DocGuide{
			Title:    title,
			Slug:     slug,
			Markdown: src,
		})
	}
	return guides
}

