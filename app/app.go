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
	"strings"
	"sync"
	"time"

	"facet/app/pkg/fctlang/checker"
	"facet/app/pkg/fctlang/doc"
	"facet/app/pkg/fctlang/evaluator"
	"facet/app/pkg/fctlang/formatter"
	"facet/app/pkg/fctlang/loader"
	"facet/app/pkg/fctlang/parser"
	"facet/app/pkg/manifold"
	"facet/app/pkg/runner"

	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// runResultPayload is the single event payload sent to the frontend on every run.
// Check data is always present. Eval data is present when an entry point was provided.
type runResultPayload struct {
	// Check data (always present)
	Errors       []parser.SourceError `json:"errors,omitempty"`
	VarTypes     checker.VarTypeMap   `json:"varTypes,omitempty"`
	Declarations *checker.DeclResult  `json:"declarations,omitempty"`
	EntryPoints  []runner.EntryPoint   `json:"entryPoints,omitempty"`
	DocIndex     []doc.DocEntry       `json:"docIndex,omitempty"`

	// Eval data (present when entry point was provided and eval succeeded)
	Mesh   *manifold.DisplayMesh `json:"mesh,omitempty"`
	Stats  *evaluator.ModelStats `json:"stats,omitempty"`
	Time   float64               `json:"time,omitempty"`
	PosMap []evaluator.PosEntry  `json:"posMap,omitempty"`

	// Debug data (present for debug runs)
	DebugFinal []*manifold.DisplayMesh `json:"debugFinal,omitempty"`
	DebugSteps []evaluator.DebugStep   `json:"debugSteps,omitempty"`
}

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

	runner   *runner.ProgramRunner
	configMu sync.Mutex

	assistantMu sync.Mutex
	cancelAssistant    context.CancelFunc
	sessionID          string
	assistantConfig    AssistantConfig
	cachedSystemPrompt string

	mcpState *mcpState // MCP server state (tools read/write this)
	mcpPort  int       // port the MCP HTTP server is listening on

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
	LastFile        string             `json:"lastFile,omitempty"`         // path of last opened file; "" = tutorial
	Appearance      json.RawMessage    `json:"appearance,omitempty"`       // frontend-owned
	Editor          json.RawMessage    `json:"editor,omitempty"`           // frontend-owned
	Assistant       json.RawMessage    `json:"assistant,omitempty"`        // frontend-owned
	Camera          json.RawMessage    `json:"camera,omitempty"`           // frontend-owned
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
	libDir, _ := libraryDir()
	a.runner = runner.New(a.ctx, runner.Callbacks{
		OnStart: func() {
			wailsRuntime.EventsEmit(a.ctx, "run:start")
		},
		OnIdle: func() {
			wailsRuntime.EventsEmit(a.ctx, "run:idle")
		},
		OnResult: func(result *runner.RunResult) {
			a.emitRunResult(result)
		},
	}, runner.Config{
		LibDir: libDir,
		ResolveOpts: func() *loader.Options {
			cfg := loadConfig()
			opts := &loader.Options{}
			if len(cfg.InstalledLibs) > 0 {
				opts.InstalledLibs = cfg.InstalledLibs
			}
			return opts
		},
	})
	a.initStderrCapture()

	// Start in-process MCP HTTP server for AI assistant tool use
	port, err := a.startMCPServer()
	if err != nil {
		log.Printf("[mcp] failed to start: %v", err)
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

// shutdown is called when the app is closing. Persists current memory limit.
func (a *App) shutdown(ctx context.Context) {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	cfg := loadConfig()
	cfg.MemoryLimitGB = a.GetMemoryLimit()
	saveConfig(cfg)
	if a.logFile != nil {
		a.logFile.Close()
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

// GetRecentFiles returns the list of recently opened file paths.
func (a *App) GetRecentFiles() []string {
	cfg := loadConfig()
	return cfg.RecentFiles
}

// GetLastFile returns the path of the last opened file ("" = tutorial/first-run).
func (a *App) GetLastFile() string {
	cfg := loadConfig()
	return cfg.LastFile
}

// SetLastFile persists the path of the currently open file.
func (a *App) SetLastFile(path string) {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	cfg := loadConfig()
	cfg.LastFile = path
	saveConfig(cfg)
}

// OpenRecentFile reads and returns the contents of path without a file dialog.
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

// DeleteScratchFile removes a scratch file from disk.
func (a *App) DeleteScratchFile(path string) error {
	dir, _ := scratchDir()
	// Safety: only delete files inside the scratch directory
	if dir == "" || !strings.HasPrefix(path, dir) {
		return fmt.Errorf("not a scratch file: %s", path)
	}
	return os.Remove(path)
}

// IsScratchFile returns true if the path is inside the scratch directory.
func (a *App) IsScratchFile(path string) bool {
	dir, _ := scratchDir()
	return dir != "" && strings.HasPrefix(path, dir)
}

// RekeySource moves a source entry in the runner's program from oldKey to newKey.
// Used after saving a scratch file to a real path.
func (a *App) RekeySource(oldKey, newKey string) {
	a.runner.RekeySource(oldKey, newKey)
}

func (a *App) OpenRecentFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return map[string]string{"path": path, "source": string(data)}, nil
}

// OpenLibraryDir reads the .fct file from a library directory and returns its path and source.
func (a *App) OpenLibraryDir(dir string) (map[string]string, error) {
	name := filepath.Base(dir)
	fctPath := filepath.Join(dir, name+".fct")
	data, err := os.ReadFile(fctPath)
	if err != nil {
		return nil, err
	}
	return map[string]string{"path": fctPath, "source": string(data)}, nil
}

// buildMenu creates the native application menu bar.
func (a *App) buildMenu() *menu.Menu {
	appMenu := menu.NewMenu()

	// macOS app menu (Facet → About, Quit, etc.)
	appMenu.Append(menu.AppMenu())

	// File menu
	fileMenu := appMenu.AddSubmenu("File")
	fileMenu.AddText("New", keys.CmdOrCtrl("n"), func(_ *menu.CallbackData) {
		wailsRuntime.EventsEmit(a.ctx, "menu:new")
	})
	fileMenu.AddText("New Library...", nil, func(_ *menu.CallbackData) {
		wailsRuntime.EventsEmit(a.ctx, "menu:new-library")
	})
	fileMenu.AddText("Open...", keys.CmdOrCtrl("o"), func(_ *menu.CallbackData) {
		wailsRuntime.EventsEmit(a.ctx, "menu:open")
	})

	// Open Recent submenu
	recentMenu := fileMenu.AddSubmenu("Open Recent")
	recentFiles := loadConfig().RecentFiles
	if len(recentFiles) == 0 {
		recentMenu.AddText("No Recent Files", nil, nil)
	} else {
		for _, p := range recentFiles {
			label := filepath.Base(p)
			recentMenu.AddText(label, nil, func(_ *menu.CallbackData) {
				wailsRuntime.EventsEmit(a.ctx, "menu:open-recent", p)
			})
		}
	}

	// Open Example submenu
	demoMenu := fileMenu.AddSubmenu("Open Example")
	for _, name := range a.GetExampleList() {
		label := strings.TrimSuffix(name, ".fct")
		demoMenu.AddText(label, nil, func(_ *menu.CallbackData) {
			wailsRuntime.EventsEmit(a.ctx, "menu:open-demo", name)
		})
	}

	// Open Library submenu — 2 levels deep: first segment → submenu, rest → label
	libMenu := fileMenu.AddSubmenu("Open Library")
	libs, _ := a.ListLocalLibraries()
	if len(libs) == 0 {
		libMenu.AddText("No Libraries Installed", nil, nil)
	} else {
		// Group by first path segment
		groups := make(map[string][]LibraryInfo)
		var groupOrder []string
		for _, lib := range libs {
			parts := strings.SplitN(lib.ID, "/", 2)
			group := parts[0]
			if _, exists := groups[group]; !exists {
				groupOrder = append(groupOrder, group)
			}
			groups[group] = append(groups[group], lib)
		}
		for _, group := range groupOrder {
			groupLibs := groups[group]
			sub := libMenu.AddSubmenu(group)
			for _, lib := range groupLibs {
				parts := strings.SplitN(lib.ID, "/", 2)
				label := group
				if len(parts) > 1 {
					label = parts[1]
				}
				libPath := lib.Path
				sub.AddText(label, nil, func(_ *menu.CallbackData) {
					wailsRuntime.EventsEmit(a.ctx, "menu:open-library", libPath)
				})
			}
		}
	}

	fileMenu.AddSeparator()
	fileMenu.AddText("Save", keys.CmdOrCtrl("s"), func(_ *menu.CallbackData) {
		wailsRuntime.EventsEmit(a.ctx, "menu:save")
	})
	fileMenu.AddText("Save As...", keys.Combo("s", keys.CmdOrCtrlKey, keys.ShiftKey), func(_ *menu.CallbackData) {
		wailsRuntime.EventsEmit(a.ctx, "menu:save-as")
	})
	fileMenu.AddSeparator()
	exportMenu := fileMenu.AddSubmenu("Export")
	exportMenu.AddText("Export 3MF...", keys.CmdOrCtrl("e"), func(_ *menu.CallbackData) {
		wailsRuntime.EventsEmit(a.ctx, "menu:export", "3mf")
	})
	exportMenu.AddText("Export STL...", nil, func(_ *menu.CallbackData) {
		wailsRuntime.EventsEmit(a.ctx, "menu:export", "stl")
	})
	exportMenu.AddText("Export OBJ...", nil, func(_ *menu.CallbackData) {
		wailsRuntime.EventsEmit(a.ctx, "menu:export", "obj")
	})

	// Edit menu (standard cut/copy/paste/undo/redo)
	appMenu.Append(menu.EditMenu())

	// Run menu
	runMenu := appMenu.AddSubmenu("Run")
	runMenu.AddText("Run", keys.CmdOrCtrl("r"), func(_ *menu.CallbackData) {
		wailsRuntime.EventsEmit(a.ctx, "menu:run")
	})
	runMenu.AddText("Debug", keys.Combo("r", keys.CmdOrCtrlKey, keys.ShiftKey), func(_ *menu.CallbackData) {
		wailsRuntime.EventsEmit(a.ctx, "menu:debug")
	})

	// View menu
	viewMenu := appMenu.AddSubmenu("View")
	viewMenu.AddText("Full Code View", keys.Combo("f", keys.CmdOrCtrlKey, keys.ShiftKey), func(_ *menu.CallbackData) {
		wailsRuntime.EventsEmit(a.ctx, "menu:fullcode")
	})
	viewMenu.AddSeparator()
	viewMenu.AddText("Toggle Grid", nil, func(_ *menu.CallbackData) {
		wailsRuntime.EventsEmit(a.ctx, "menu:toggle-grid")
	})
	viewMenu.AddText("Toggle Axes", nil, func(_ *menu.CallbackData) {
		wailsRuntime.EventsEmit(a.ctx, "menu:toggle-axes")
	})
	viewMenu.AddSeparator()
	viewMenu.AddText("Docs", keys.Combo("d", keys.CmdOrCtrlKey, keys.ShiftKey), func(_ *menu.CallbackData) {
		wailsRuntime.EventsEmit(a.ctx, "menu:docs")
	})

	// Model menu
	modelMenu := appMenu.AddSubmenu("Model")
	modelMenu.AddText("Parameters", keys.Combo("p", keys.CmdOrCtrlKey, keys.ShiftKey), func(_ *menu.CallbackData) {
		wailsRuntime.EventsEmit(a.ctx, "menu:params")
	})
	modelMenu.AddText("AI Assistant", keys.Combo("a", keys.CmdOrCtrlKey, keys.ShiftKey), func(_ *menu.CallbackData) {
		wailsRuntime.EventsEmit(a.ctx, "menu:assistant")
	})
	modelMenu.AddSeparator()
	slicers := detectSlicers()
	if len(slicers) == 0 {
		modelMenu.AddText("Send to Slicer", nil, func(_ *menu.CallbackData) {
			wailsRuntime.EventsEmit(a.ctx, "menu:slicer")
		})
	} else {
		slicerMenu := modelMenu.AddSubmenu("Send to Slicer")
		for _, s := range slicers {
			slicerMenu.AddText(s.Name, nil, func(_ *menu.CallbackData) {
				wailsRuntime.EventsEmit(a.ctx, "menu:slicer-id", s.ID)
			})
		}
	}

	// Window menu (macOS standard + Settings)
	windowMenu := appMenu.AddSubmenu("Window")
	windowMenu.AddText("Settings", keys.CmdOrCtrl(","), func(_ *menu.CallbackData) {
		wailsRuntime.EventsEmit(a.ctx, "menu:settings")
	})

	return appMenu
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

// Stop cancels the current build without starting a new one.
func (a *App) Stop() {
	a.runner.Stop()
}

// ResetRunner clears all runner state: cached program, entry point, and library cache.
func (a *App) ResetRunner() {
	a.runner.Reset()
}

// UpdateSource parses the given source and updates the runner's program state.
// key is the disk path of the source file.
func (a *App) UpdateSource(key string, source string) {
	a.runner.UpdateSource(key, source)
}

// Run triggers an immediate evaluation from the given source key.
func (a *App) Run(key string, entryPoint string, overrides map[string]interface{}) {
	a.runner.Run(key, entryPoint, overrides)
}

// Debug triggers an immediate debug evaluation from the given source key.
func (a *App) Debug(key string, entryPoint string, overrides map[string]interface{}) {
	a.runner.Debug(key, entryPoint, overrides)
}

// emitRunResult sends a single run:result event with all data.
func (a *App) emitRunResult(result *runner.RunResult) {
	payload := runResultPayload{
		Errors:       result.Errors,
		VarTypes:     result.VarTypes,
		Declarations: result.Declarations,
		EntryPoints:  result.EntryPoints,
		DocIndex:     result.DocIndex,
		Mesh:         result.Mesh,
		Stats:        result.Stats,
		Time:         result.Time,
		PosMap:       result.PosMap,
	}
	if result.DebugResult != nil {
		payload.DebugFinal = result.DebugResult.Final
		payload.DebugSteps = result.DebugResult.Steps
	}
	wailsRuntime.EventsEmit(a.ctx, "run:result", payload)
}

// FormatCode normalizes the indentation of Facet source code.
func (a *App) FormatCode(source string) string {
	src, err := parser.Parse(source)
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

// GetDebugStepMeshes lazily extracts meshes for a single debug step.
// Called by the frontend when the user navigates to a step.
func (a *App) GetDebugStepMeshes(index int) ([]evaluator.DebugMesh, error) {
	r := a.runner.LastResult()
	if r == nil || r.DebugResult == nil {
		return nil, fmt.Errorf("no debug result")
	}
	if index < 0 || index >= len(r.DebugResult.Steps) {
		return nil, fmt.Errorf("step index out of range")
	}
	meshes := r.DebugResult.ResolveMeshes(index)
	if meshes == nil {
		return []evaluator.DebugMesh{}, nil
	}
	return meshes, nil
}

// ExportMesh exports the last evaluated model in the given format.
// Uses Manifold's Assimp-backed I/O for export.
// Shows a native save dialog to choose the output path.
func (a *App) ExportMesh(format string) error {
	r := a.runner.LastResult()
	var solids []*manifold.Solid
	if r != nil {
		solids = r.Solids
	}
	if len(solids) == 0 {
		return fmt.Errorf("no mesh to export — run your code first")
	}

	var filter wailsRuntime.FileFilter
	var defaultName string
	switch format {
	case "3mf":
		filter = wailsRuntime.FileFilter{DisplayName: "3MF Files (*.3mf)", Pattern: "*.3mf"}
		defaultName = "export.3mf"
	case "stl":
		filter = wailsRuntime.FileFilter{DisplayName: "STL Files (*.stl)", Pattern: "*.stl"}
		defaultName = "export.stl"
	case "obj":
		filter = wailsRuntime.FileFilter{DisplayName: "OBJ Files (*.obj)", Pattern: "*.obj"}
		defaultName = "export.obj"
	case "glb":
		filter = wailsRuntime.FileFilter{DisplayName: "GLB Files (*.glb)", Pattern: "*.glb"}
		defaultName = "export.glb"
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}

	path, err := wailsRuntime.SaveFileDialog(a.ctx, wailsRuntime.SaveDialogOptions{
		DefaultFilename: defaultName,
		Filters:         []wailsRuntime.FileFilter{filter},
	})
	if err != nil {
		return err
	}
	if path == "" {
		return nil // user cancelled
	}

	// Use Go-native writers for 3MF/STL/OBJ; assimp for other formats.
	switch format {
	case "3mf":
		return manifold.Export3MFMulti(solids, path)
	case "stl":
		return manifold.ExportSTLMulti(solids, path)
	case "obj":
		return manifold.ExportOBJMulti(solids, path)
	default:
		return manifold.ExportMeshes(solids, path)
	}
}

// DetectSlicers returns the list of slicer applications found on the system.
func (a *App) DetectSlicers() []SlicerInfo {
	return detectSlicers()
}

// SendToSlicer exports the last evaluated model as .3mf to a stable temp file
// and opens it in the specified slicer application. Uses a fixed path per slicer
// so repeated sends reuse the already-open file in the slicer.
func (a *App) SendToSlicer(slicerID string) error {
	r := a.runner.LastResult()
	var solids []*manifold.Solid
	if r != nil {
		solids = r.Solids
	}
	if len(solids) == 0 {
		return fmt.Errorf("no mesh to export — run your code first")
	}

	// Stable path per instance: reuse the same file so the slicer can detect
	// updates, but include PID so multiple app instances don't collide.
	path := filepath.Join(os.TempDir(), fmt.Sprintf("facet-slicer-%s-%d.3mf", slicerID, os.Getpid()))
	if err := manifold.Export3MFMulti(solids, path); err != nil {
		return err
	}
	return launchSlicer(slicerID, path)
}

// ---------------------------------------------------------------------------
// Library management
// ---------------------------------------------------------------------------

// LibraryInfo describes an installed/cached library for the settings UI.
type LibraryInfo struct {
	ID   string `json:"id"`   // "github.com/user/repo" (full path for operations)
	Name string `json:"name"` // "user/repo" (display name)
	Ref  string `json:"ref"`  // "v1.0" or "main"
	Path string `json:"path"` // local filesystem path
}

// ListLibraries scans the git cache directory for cloned libraries.
func (a *App) ListLibraries() ([]LibraryInfo, error) {
	cacheDir := loader.DefaultGitCacheDir()
	var libs []LibraryInfo

	// Walk <cacheDir>/<host>/<user>/<repo>/<ref>/
	hosts, _ := os.ReadDir(cacheDir)
	for _, h := range hosts {
		if !h.IsDir() || strings.HasPrefix(h.Name(), ".") {
			continue
		}
		users, _ := os.ReadDir(filepath.Join(cacheDir, h.Name()))
		for _, u := range users {
			if !u.IsDir() {
				continue
			}
			repos, _ := os.ReadDir(filepath.Join(cacheDir, h.Name(), u.Name()))
			for _, r := range repos {
				if !r.IsDir() {
					continue
				}
				refs, _ := os.ReadDir(filepath.Join(cacheDir, h.Name(), u.Name(), r.Name()))
				for _, ref := range refs {
					if !ref.IsDir() {
						continue
					}
					libs = append(libs, LibraryInfo{
						ID:   fmt.Sprintf("%s/%s/%s", h.Name(), u.Name(), r.Name()),
						Name: fmt.Sprintf("%s/%s", u.Name(), r.Name()),
						Ref:  ref.Name(),
						Path: filepath.Join(cacheDir, h.Name(), u.Name(), r.Name(), ref.Name()),
					})
				}
			}
		}
	}
	return libs, nil
}

// ListLocalLibraries scans the local libraries directory recursively for libraries.
// A library is any directory containing a <dirname>.fct file.
func (a *App) ListLocalLibraries() ([]LibraryInfo, error) {
	libDir, err := libraryDir()
	if err != nil {
		return nil, err
	}
	var libs []LibraryInfo
	var walk func(dir, rel string)
	walk = func(dir, rel string) {
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			// Skip the embedded "facet" namespace at top level
			if rel == "" && e.Name() == "facet" {
				continue
			}
			childDir := filepath.Join(dir, e.Name())
			childRel := e.Name()
			if rel != "" {
				childRel = rel + "/" + e.Name()
			}
			// Check if this directory is a library (contains <name>.fct)
			fctFile := filepath.Join(childDir, e.Name()+".fct")
			if _, err := os.Stat(fctFile); err == nil {
				libs = append(libs, LibraryInfo{
					ID:   childRel,
					Path: childDir,
				})
			}
			// Recurse deeper
			walk(childDir, childRel)
		}
	}
	walk(libDir, "")
	return libs, nil
}

// ClearLibCache removes all cached (git-cloned) libraries.
func (a *App) ClearLibCache() error {
	cacheDir := loader.DefaultGitCacheDir()
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return nil // nothing to clear
	}
	for _, e := range entries {
		os.RemoveAll(filepath.Join(cacheDir, e.Name()))
	}

	return nil
}

// validateLibName checks that a name contains only safe characters for library paths.
func validateLibName(name string) error {
	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			return fmt.Errorf("name may only contain letters, digits, hyphens, and underscores")
		}
	}
	return nil
}

// CreateLibraryFolder creates a new top-level library folder.
// Import paths for libraries inside will be "<folder>/<name>".
func (a *App) CreateLibraryFolder(folder string) error {
	if err := validateLibName(folder); err != nil {
		return err
	}
	libDir, err := libraryDir()
	if err != nil {
		return err
	}
	return os.MkdirAll(filepath.Join(libDir, folder), 0o755)
}

// CreateLocalLibrary creates a new library inside a folder with a starter template.
// The library is created at <libDir>/<folder>/<name>/<name>.fct.
// Import path: lib "<folder>/<name>"
func (a *App) CreateLocalLibrary(folder, name string) (string, error) {
	if err := validateLibName(folder); err != nil {
		return "", fmt.Errorf("folder: %w", err)
	}
	if err := validateLibName(name); err != nil {
		return "", fmt.Errorf("library: %w", err)
	}

	libDir, err := libraryDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(libDir, folder, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	filePath := filepath.Join(dir, name+".fct")
	if _, err := os.Stat(filePath); err == nil {
		return filePath, nil // already exists — just open it
	}

	template := fmt.Sprintf("# %s/%s library\n\n", folder, name)
	if err := os.WriteFile(filePath, []byte(template), 0o644); err != nil {
		return "", err
	}
	return filePath, nil
}

// ListLibraryFolders returns the top-level library folder names.
func (a *App) ListLibraryFolders() ([]string, error) {
	libDir, err := libraryDir()
	if err != nil {
		return nil, err
	}
	entries, _ := os.ReadDir(libDir)
	var folders []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") && e.Name() != "facet" {
			folders = append(folders, e.Name())
		}
	}
	return folders, nil
}

// GetLibraryDir returns the path to the local libraries root directory.
func (a *App) GetLibraryDir() (string, error) {
	return libraryDir()
}

// RemoveLocalLibrary removes a local library directory.
func (a *App) RemoveLocalLibrary(id string) error {
	libDir, err := libraryDir()
	if err != nil {
		return err
	}
	target := filepath.Join(libDir, id)
	// Safety: ensure target is inside libDir
	abs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	absLib, err := filepath.Abs(libDir)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(abs, absLib+string(filepath.Separator)) {
		return fmt.Errorf("invalid library path")
	}
	return os.RemoveAll(target)
}

// InstallLibrary clones a remote library into the git cache.
func (a *App) InstallLibrary(url, ref string) error {
	if ref == "" {
		ref = "main"
	}
	// Parse the URL to determine cache path
	// Expected format: github.com/user/repo or https://github.com/user/repo.git
	rawPath := strings.TrimPrefix(url, "https://")
	rawPath = strings.TrimPrefix(rawPath, "http://")
	rawPath = strings.TrimSuffix(rawPath, ".git")
	rawPath = strings.TrimSuffix(rawPath, "/")

	segments := strings.Split(rawPath, "/")
	if len(segments) < 3 {
		return fmt.Errorf("invalid library URL: need host/user/repo")
	}

	cacheDir := loader.DefaultGitCacheDir()
	dest := filepath.Join(cacheDir, segments[0], segments[1], segments[2], ref)

	cloneURL := fmt.Sprintf("https://%s/%s/%s.git", segments[0], segments[1], segments[2])
	if err := loader.CloneRepo(a.ctx, cloneURL, ref, dest); err != nil {
		return err
	}

	a.rebuildSystemPrompt()
	return nil
}

// RemoveLibrary removes a cached library from the git cache.
func (a *App) RemoveLibrary(id, ref string) error {
	cacheDir := loader.DefaultGitCacheDir()
	target := filepath.Join(cacheDir, id, ref)
	// Safety: ensure target is inside cacheDir
	abs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	absCache, err := filepath.Abs(cacheDir)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(abs, absCache+string(filepath.Separator)) {
		return fmt.Errorf("invalid library path")
	}
	if err := os.RemoveAll(target); err != nil {
		return err
	}

	return nil
}

// UpdateLibrary runs git pull in a cached library directory to fetch updates.
func (a *App) UpdateLibrary(id, ref string) error {
	cacheDir := loader.DefaultGitCacheDir()
	dir := filepath.Join(cacheDir, id, ref)
	if err := loader.PullRepo(a.ctx, dir); err != nil {
		fmt.Fprintf(os.Stderr, "UpdateLibrary %s@%s: %v\n", id, ref, err)
		return err
	}

	return nil
}

// PullAllLibraries pulls all cached libraries, fetching updates and tags.
func (a *App) PullAllLibraries() error {
	libs, err := a.ListLibraries()
	if err != nil {
		return err
	}
	var errs []string
	for _, lib := range libs {
		if err := loader.PullRepo(a.ctx, lib.Path); err != nil {
			msg := fmt.Sprintf("%s@%s: %v", lib.Name, lib.Ref, err)
			fmt.Fprintf(os.Stderr, "PullAllLibraries: %s\n", msg)
			errs = append(errs, msg)
		}
	}
	a.rebuildSystemPrompt()
	if len(errs) > 0 {
		return fmt.Errorf("pull errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// ForkLibrary copies a cached library to the local libraries directory,
// skipping hidden files/directories (like .git).
func (a *App) ForkLibrary(id, ref string) error {
	cacheDir := loader.DefaultGitCacheDir()
	src := filepath.Join(cacheDir, id, ref)
	// Strip host from id (github.com/user/repo -> user/repo) for local path.
	// Only strip the first segment if it looks like a hostname (contains a dot).
	localID := id
	if parts := strings.SplitN(id, "/", 2); len(parts) > 1 && strings.Contains(parts[0], ".") {
		localID = parts[1]
	}
	libDirPath, err := libraryDir()
	if err != nil {
		return err
	}
	dst := filepath.Join(libDirPath, localID)

	if err := filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Skip hidden files/directories
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0644)
	}); err != nil {
		return err
	}
	a.rebuildSystemPrompt()
	return nil
}

// IsReadOnlyPath returns true if the given path is inside the git cache
// or the built-in library directory, meaning it should not be user-edited.
// User-created local libraries (under <libDir>/local/) are writable.
func (a *App) IsReadOnlyPath(p string) bool {
	if p == "" {
		return false
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return false
	}
	// Git cache is always read-only.
	if d, err := filepath.Abs(loader.DefaultGitCacheDir()); err == nil {
		if strings.HasPrefix(abs, d+string(filepath.Separator)) || abs == d {
			return true
		}
	}
	// Library directory is writable — it contains only user-managed files
	// (forked libraries and locally created ones).
	if ld, ldErr := libraryDir(); ldErr == nil {
		if d, err := filepath.Abs(ld); err == nil {
			if strings.HasPrefix(abs, d+string(filepath.Separator)) || abs == d {
				return false
			}
		}
	}
	return false
}

// GetLibraryFilePath resolves a library import path to its disk path
// using the current program's import mapping. Returns empty string if not found.
func (a *App) GetLibraryFilePath(importPath string) string {
	prog := a.runner.Prog()
	return prog.Resolve(importPath)
}

// RevealInFileManager opens the given path in the OS file manager and brings it to front.
func (a *App) RevealInFileManager(path string) error {
	return revealInFileManager(path)
}
