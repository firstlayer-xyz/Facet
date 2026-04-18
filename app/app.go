package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"sync/atomic"

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

// App struct holds the application state and is bound to the frontend via Wails.
type App struct {
	ctx context.Context

	config    *ConfigStore
	logs      *LogCapture
	assistant *AssistantService
	libraries *LibraryManager
	eval      *EvalService
	mcp       *MCPService
}

// NewApp creates a new App application struct.
func NewApp() *App {
	assistant := NewAssistantService()
	eval := NewEvalService()
	return &App{
		config:    NewConfigStore(),
		logs:      NewLogCapture(),
		assistant: assistant,
		libraries: NewLibraryManager(assistant),
		eval:      eval,
		mcp:       NewMCPService(eval),
	}
}

// GetHTTPAuth returns the port + bearer token for the localhost HTTP server.
// The frontend must include `Authorization: Bearer <token>` on every request
// to /eval, /check, or /mcp.  Exposed via Wails binding.
func (a *App) GetHTTPAuth() HTTPAuth {
	return a.mcp.Auth()
}

// GetSettings returns the full settings JSON for the frontend. If the on-disk
// settings file exists but cannot be read or parsed, the error is surfaced so
// the frontend can warn the user rather than silently overwriting it with
// defaults on the next save.
func (a *App) GetSettings() (string, error) {
	return a.config.GetJSON()
}

// PatchSettings merges the provided partial JSON into the existing config.
// See ConfigStore.Patch for semantics.
func (a *App) PatchSettings(jsonStr string) error {
	return a.config.Patch(jsonStr)
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

	// A corrupt or unreadable settings file must not prevent startup — the
	// user still needs to launch the app to fix it. Log loudly so the
	// failure is discoverable; run with defaults for this launch. Subsequent
	// writes (PatchSettings, AddRecentFile, etc.) re-read and refuse to save
	// if the file is still corrupt, so the user's data is not clobbered.
	cfg, err := loadConfig()
	if err != nil {
		log.Printf("[settings] %v — continuing with defaults; writes will refuse until this is resolved", err)
	}
	applyMemoryLimit(cfg.MemoryLimitGB)

	a.libraries.SetContext(ctx)
	a.assistant.SetEventContext(ctx)
	a.assistant.RebuildSystemPrompt()
	a.logs.Start(ctx)

	// Start in-process HTTP server (MCP + eval endpoints)
	if _, _, err := a.mcp.Start(ctx); err != nil {
		log.Printf("[http] failed to start: %v", err)
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
var hasDirtyFiles atomic.Bool

// SetDirtyState is called by the frontend to report whether any files have unsaved changes.
func (a *App) SetDirtyState(dirty bool) {
	hasDirtyFiles.Store(dirty)
}

// beforeClose is called when the user tries to close the window.
// Returns true to prevent closing (user chose to cancel).
func (a *App) beforeClose(ctx context.Context) bool {
	// Emit event so frontend can persist tab state
	wailsRuntime.EventsEmit(ctx, "app:before-close")

	if !hasDirtyFiles.Load() {
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
// If the settings file is corrupt, the update is skipped rather than wiping
// the user's settings.
func (a *App) shutdown(ctx context.Context) {
	err := a.config.Mutate(func(cfg *appConfig) error {
		cfg.MemoryLimitGB = a.GetMemoryLimit()
		return nil
	})
	if err != nil {
		log.Printf("[settings] shutdown: %v", err)
	}
	cleanupScratchFiles()
	a.logs.Close()
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

// GetLogDir returns the path to the logs directory.
func (a *App) GetLogDir() string {
	return logDir()
}

// GetStderrLog returns the current stderr buffer contents.
func (a *App) GetStderrLog() string {
	return a.logs.Stderr()
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
// 0 means use the default (8 GB). The in-memory limit is applied even if we
// cannot persist it to disk — so this run behaves as the user asked; only the
// next launch will forget.
func (a *App) SetMemoryLimit(gb int64) error {
	applyMemoryLimit(gb)
	return a.config.Mutate(func(cfg *appConfig) error {
		cfg.MemoryLimitGB = gb
		return nil
	})
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

// AddRecentFile records path as the most recently opened file. If the
// settings file is corrupt, the update is skipped rather than clobbering it.
func (a *App) AddRecentFile(path string) {
	if path == "" {
		return
	}
	err := a.config.Mutate(func(cfg *appConfig) error {
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
		return nil
	})
	if err != nil {
		log.Printf("[settings] AddRecentFile: %v", err)
	}
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
// If the config dir can't be resolved (e.g. no $HOME), we conservatively say
// "no" — this is only a UI hint (scratch vs saved file) and returning false
// just means the file is treated as saved, which is the safe default.
func (a *App) IsScratchFile(path string) bool {
	dir, err := scratchDir()
	if err != nil {
		return false
	}
	return strings.HasPrefix(path, dir)
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

