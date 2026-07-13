package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"facet/pkg/fctlang/formatter"
	"facet/pkg/fctlang/parser"
	"facet/share/examples"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// mcpState holds editor state shared between the MCP server tools and the app.
// The assistant sets the current editor code and active-tab metadata before
// each run so MCP tools (get_editor_code, edit_code, replace_code, new_file,
// check_syntax) can operate on it without taking a fresh RPC round-trip to
// the frontend. The active tab is latched at run start: if the user switches
// tabs mid-run, MCP tools continue operating on the tab the assistant was
// invoked against — avoids silent cross-tab edits.
//
// lastRun holds a small summary of the most recent /eval run so the
// get_last_run tool can report triangles/vertices/bbox/errors without
// re-evaluating. It is overwritten on every eval (user-triggered or
// assistant-triggered).
type mcpState struct {
	mu            sync.Mutex
	editorCode    string // current editor content, set before each assistant call
	activeTabPath string // path of the tab the assistant was invoked against
	readOnly      bool   // true if the active tab cannot be modified (stdlib, library, example)
	lastRun       *runSummary
}

// runSummary is a small JSON-serializable snapshot of an /eval run. It is
// updated by handleEval via MCPService.RecordRun and returned verbatim by
// the get_last_run MCP tool.
//
// Per-object bounding boxes and piece counts let the assistant verify
// positioning and printability ("every object should be exactly 1 piece").
type runSummary struct {
	Ok          bool                 `json:"ok"`
	Errors      []parser.SourceError `json:"errors,omitempty"`
	Triangles   int                  `json:"triangles,omitempty"`
	Vertices    int                  `json:"vertices,omitempty"`
	Volume      float64              `json:"volume,omitempty"`
	SurfaceArea float64              `json:"surfaceArea,omitempty"`
	BBoxMin     [3]float64           `json:"bboxMin,omitempty"`
	BBoxMax     [3]float64           `json:"bboxMax,omitempty"`
	Objects     []objectSummary      `json:"objects,omitempty"`
	TimeSec     float64              `json:"timeSec,omitempty"`
	Entry       string               `json:"entry,omitempty"`
	Key         string               `json:"key,omitempty"`
	// Sources is the user-authored source code that was evaluated, keyed by
	// tab path. Library / stdlib code is excluded — the assistant only needs
	// to see what is editable. Lets the assistant detect mid-turn user edits
	// by comparing against what it wrote.
	Sources map[string]string `json:"sources,omitempty"`
	RanAt   time.Time         `json:"ranAt"`
}

// objectSummary describes one top-level solid returned by Main():
// its bounding box and the number of disconnected pieces it contains.
// Piece count > 1 means the object has floating disconnected geometry —
// not 3D-printable as a single part.
type objectSummary struct {
	BBoxMin [3]float64 `json:"bboxMin"`
	BBoxMax [3]float64 `json:"bboxMax"`
	Pieces  int        `json:"pieces"`
}

func newMCPState() *mcpState {
	return &mcpState{}
}

// setContext latches the per-run editor state. Call once before each assistant
// Send — the latched values persist for the lifetime of the request even if
// the user switches tabs in the UI.
func (s *mcpState) setContext(code, activeTabPath string, readOnly bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.editorCode = code
	s.activeTabPath = activeTabPath
	s.readOnly = readOnly
}

func (s *mcpState) getEditorCode() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.editorCode
}

func (s *mcpState) isReadOnly() (bool, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readOnly, s.activeTabPath
}

// setLastRun stores the most recent eval summary. Overwrites the previous
// slot unconditionally — the assistant and the user share one view of
// "the last thing that ran."
func (s *mcpState) setLastRun(r runSummary) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r.RanAt = time.Now()
	s.lastRun = &r
}

// getLastRun returns a copy of the last run summary, or nil if no run has
// completed in this session.
func (s *mcpState) getLastRun() *runSummary {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lastRun == nil {
		return nil
	}
	copy := *s.lastRun
	return &copy
}

// listExamplesResponse renders the built-in examples as a markdown list,
// pairing each filename with the first comment line in the file (used as a
// short summary when present). Sorted alphabetically so output is stable.
func listExamplesResponse() string {
	entries, err := examples.FS.ReadDir(".")
	if err != nil {
		return "(no examples available)"
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	var sb strings.Builder
	sb.WriteString("Built-in Facet examples. Fetch source with get_example(name).\n\n")
	for _, name := range names {
		sb.WriteString("- `")
		sb.WriteString(name)
		sb.WriteString("`")
		if summary := firstCommentLine(name); summary != "" {
			sb.WriteString(" — ")
			sb.WriteString(summary)
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

// firstCommentLine reads the named example and returns the first `# ...`
// comment line, stripped of its leading "# " prefix. Returns "" if the file
// has no leading comment.
func firstCommentLine(name string) string {
	data, err := examples.FS.ReadFile(name)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
		}
		return "" // first non-blank line is code, not a comment
	}
	return ""
}

// --- MCP tool input types ---

type getEditorCodeInput struct{}

type editCodeInput struct {
	Search  string `json:"search" jsonschema:"Exact text to find in the editor (must match verbatim)"`
	Replace string `json:"replace" jsonschema:"Text to replace the search match with"`
}

type replaceCodeInput struct {
	Code string `json:"code" jsonschema:"Complete new source code for the editor"`
}

type checkSyntaxInput struct {
	Source string `json:"source,omitempty" jsonschema:"Source code to check (omit to use current editor code)"`
}

type getDocumentationInput struct {
	Section string `json:"section,omitempty" jsonschema:"Limit the response to one section. One of: 'language' (grammar/semantics), 'colors' (color guide), 'stdlib' (auto-generated function/method catalog), 'libraries' (user/cached lib catalog). Omit to return all sections."`
	Query   string `json:"query,omitempty" jsonschema:"Case-insensitive substring to filter stdlib and library entries by name (e.g. 'Cylinder', 'Align'). Applies only to the 'stdlib' and 'libraries' sections; other sections are omitted when a query is set."`
}

type newFileInput struct {
	Name string `json:"name" jsonschema:"Label for the new file (e.g. 'plate-with-holes'). '.fct' is appended if missing."`
	Code string `json:"code" jsonschema:"Initial source code for the new file. Must be a complete Facet program if the user expects it to render."`
}

type getLastRunInput struct{}

type listExamplesInput struct{}

type getExampleInput struct {
	Name string `json:"name" jsonschema:"Example filename as returned by list_examples (e.g. 'Chess Pawn.fct'). Case-sensitive."`
}

type formatCodeInput struct {
	Source string `json:"source,omitempty" jsonschema:"Source code to format. Omit to format the current editor code."`
}

// gui_* tool inputs drive the GUI via the automation registry (same commands
// the /control route exposes).
type guiSetCameraInput struct {
	Azimuth   float64 `json:"azimuth" jsonschema:"Camera azimuth in degrees (around the up axis)."`
	Elevation float64 `json:"elevation" jsonschema:"Camera elevation in degrees above the horizontal."`
	Distance  float64 `json:"distance,omitempty" jsonschema:"Distance from the target. Omit to keep the current distance."`
}

type guiRecordStartInput struct {
	Mode string `json:"mode" jsonschema:"Capture surface: 'canvas' (3D viewer only) or 'page' (full UI)."`
	FPS  int    `json:"fps,omitempty" jsonschema:"Frames per second. Omit for 30."`
}

type guiRecordStopInput struct{}

// requestPermissionInput is the payload the Claude CLI sends to the tool named
// by --permission-prompt-tool whenever a tool use is not pre-approved.
// CONTRACT (verified against claude 2.1.x): the CLI sends the proposed tool
// name and its input object, and expects back a single text content block
// whose text is JSON {"behavior":"allow","updatedInput":...} or
// {"behavior":"deny","message":...}. Extra fields (tool_use_id,
// permission_suggestions) are ignored.
type requestPermissionInput struct {
	ToolName string         `json:"tool_name"`
	Input    map[string]any `json:"input"`
}

// permissionSummary renders a one-line human description of a pending tool use
// for the Allow/Deny card.
func permissionSummary(toolName string, input map[string]any) string {
	switch toolName {
	case "WebSearch":
		if q, ok := input["query"].(string); ok {
			return "Search the web for: " + q
		}
		return "Search the web"
	case "WebFetch":
		if u, ok := input["url"].(string); ok {
			return "Fetch web page: " + u
		}
		return "Fetch a web page"
	case "Bash":
		if c, ok := input["command"].(string); ok {
			return "Run shell command: " + c
		}
		return "Run a shell command"
	default:
		return "Use tool: " + toolName
	}
}

// permissionRememberKey returns the session-remember key for a tool use. For
// network tools it includes the target host so "remember" is scoped per site;
// otherwise it is the tool name.
func permissionRememberKey(toolName string, input map[string]any) string {
	if toolName == "WebFetch" {
		if u, ok := input["url"].(string); ok {
			if parsed, err := url.Parse(u); err == nil && parsed.Host != "" {
				return "WebFetch:" + parsed.Host
			}
		}
	}
	return toolName
}

// fetchURLInput is the argument to the fetch_url tool.
type fetchURLInput struct {
	URL string `json:"url" jsonschema:"The absolute http(s) URL to fetch."`
}

// askUserQuestionInput mirrors the schema of Claude Code's built-in
// AskUserQuestion tool so the model uses this one the same way: a list of
// 1-4 self-contained questions, each with 2-4 mutually-exclusive options.
// The tool handler returns the user's selections (one label per question,
// or several when MultiSelect is true) as JSON.
type askUserQuestionInput struct {
	Questions []askQuestion `json:"questions" jsonschema:"List of 1-4 questions to ask the user. Each question is self-contained; do not assume the user remembers context from earlier ones."`
}

type askQuestion struct {
	Question    string      `json:"question" jsonschema:"The full question text. End with a question mark."`
	Header      string      `json:"header" jsonschema:"Very short label shown as a chip/tag (max ~12 characters), e.g. 'Auth method', 'Library', 'Approach'."`
	Options     []askOption `json:"options" jsonschema:"2-4 distinct, mutually-exclusive options. No 'Other' — the UI adds it automatically so the user can supply free text."`
	MultiSelect bool        `json:"multiSelect,omitempty" jsonschema:"Set true to let the user pick multiple options. Default false (single-select)."`
}

type askOption struct {
	Label       string `json:"label" jsonschema:"Concise label shown on the option button (1-5 words)."`
	Description string `json:"description,omitempty" jsonschema:"One-line explanation of what selecting this option means."`
}

// screenshotViewportInput optionally specifies a camera pose. Omit all
// of azimuth/elevation/distance to capture the user's live view; supply
// all three (and optionally target) to render an off-screen view from a
// chosen angle without disturbing the user's camera.
type screenshotViewportInput struct {
	Azimuth   *float64    `json:"azimuth,omitempty" jsonschema:"Degrees around the bed's up axis, counterclockwise viewed from above. 0° is the bed's front-view direction. Optional — omit to use the user's current view."`
	Elevation *float64    `json:"elevation,omitempty" jsonschema:"Degrees above the horizontal plane. 0° is on the horizon, 90° is overhead (top-down), -90° is from underneath. Optional — omit to use the user's current view."`
	Distance  *float64    `json:"distance,omitempty" jsonschema:"Distance from target to camera, in millimetres. Optional — omit to use the user's current view."`
	Target    *vec3Coords `json:"target,omitempty" jsonschema:"World-space point the camera looks at, in millimetres. Defaults to the model bounding-sphere centre. Only used when azimuth/elevation/distance are supplied."`
}

// vec3Coords is a {x,y,z} point in world-space millimetres.
type vec3Coords struct {
	X float64 `json:"x" jsonschema:"X coordinate in millimetres."`
	Y float64 `json:"y" jsonschema:"Y coordinate in millimetres."`
	Z float64 `json:"z" jsonschema:"Z coordinate in millimetres."`
}

// updateTaskPlanInput renders the model's working task list in the
// assistant panel so the user can see step-by-step progress on a
// multi-turn build. Pass the FULL list each call — the panel replaces
// its state, it does not merge or append.
type updateTaskPlanInput struct {
	Tasks []taskItem `json:"tasks" jsonschema:"Full task list (replaces any previous list). One entry per discrete step the model intends to do; mark each with its current status."`
}

type taskItem struct {
	Content string `json:"content" jsonschema:"Short imperative description of the step, e.g. 'Model the pawn base'."`
	Status  string `json:"status" jsonschema:"One of: 'pending' (not started), 'in_progress' (current step), 'completed' (finished)."`
}

// HTTPAuth is the payload returned to the frontend so it can authenticate
// requests to the localhost HTTP server.
type HTTPAuth struct {
	Port  int    `json:"port"`
	Token string `json:"token"`
}

// MCPService builds the MCP server and registers the assistant tools that read
// and write the editor's code/context and answer questions/permissions. The
// HTTPServer hosts the resulting handler at /mcp (alongside /eval and /check);
// this service no longer owns the listener or the bearer token.
type MCPService struct {
	state    *mcpState
	eventCtx context.Context

	// In-flight ask_user_question calls. Each tool invocation parks a
	// goroutine on its channel until the frontend reports the answer via
	// the AssistantAnswerQuestion Wails binding (which calls AnswerQuestion
	// below). Keyed by a per-call ID emitted to the frontend in the
	// assistant:question event payload — the frontend echoes it back on
	// submit so we can route the answer to the right pending call.
	questionsMu sync.Mutex
	questions   map[string]chan questionAnswer

	// In-flight screenshot_viewport calls. Same pattern as `questions`
	// above: tool emits an assistant:screenshot-request event with an id,
	// blocks on the channel, and the frontend echoes the captured PNG
	// back via DeliverViewportScreenshot. Raw image bytes (not base64)
	// land in the channel so the MCP layer can hand them straight to
	// ImageContent, which re-encodes for the wire.
	screenshotsMu sync.Mutex
	screenshots   map[string]chan screenshotResult

	// In-flight permission requests. The request_permission MCP tool (and
	// fetch_url's self-gate) park here until the frontend answers via the
	// AnswerToolPermission Wails binding. Same id→chan pattern as questions.
	permissionsMu sync.Mutex
	permissions   map[string]chan permissionDecision

	// Session-remembered approvals: keys the user chose to "remember for
	// this session". Cleared on ClearHistory so a new conversation re-asks.
	rememberedMu sync.Mutex
	remembered   map[string]struct{}

	// automation drives the GUI for the gui_* tools. Shared with the /control
	// route so the assistant and external drivers use one command registry.
	automation *AutomationController
}

// screenshotResult carries the captured viewport PNG (raw bytes) back
// to the parked tool handler, or an error string if the capture failed
// (e.g. WebGL context lost).
type screenshotResult struct {
	PNG []byte
	Err string
}

// questionAnswer carries the user's selections (and any "Other" / notes
// text) back from the frontend to the parked MCP tool handler. The shape
// mirrors Claude Code's built-in AskUserQuestion tool so the model can
// interpret the result without further training.
type questionAnswer struct {
	Answers map[string]string `json:"answers"`
	Notes   map[string]string `json:"notes,omitempty"`
}

// permissionDecision carries the user's allow/deny choice back from the
// frontend to a parked permission request.
type permissionDecision struct {
	Allow    bool
	Remember bool
}

// NewMCPService creates a new MCP service. It registers itself as the run
// recorder on the EvalService so every /eval response updates the lastRun slot
// that the get_last_run tool reports. The MCP server itself is built lazily by
// buildServer when the HTTPServer mounts /mcp.
func NewMCPService(eval *EvalService, automation *AutomationController) *MCPService {
	m := &MCPService{
		state:       newMCPState(),
		questions:   make(map[string]chan questionAnswer),
		screenshots: make(map[string]chan screenshotResult),
		permissions: make(map[string]chan permissionDecision),
		remembered:  make(map[string]struct{}),
		automation:  automation,
	}
	eval.SetRunRecorder(m.RecordRun)
	return m
}

// takeResolution atomically fetches and removes the channel parked under id, so
// at most one caller — a single answerer, or the handler's cancel branch — ever
// owns it. The map entry is thus the single resolution token for a parked call.
// ok is false when the entry is already gone (someone else resolved it).
func takeResolution[T any](mu *sync.Mutex, m map[string]chan T, id string) (chan T, bool) {
	mu.Lock()
	defer mu.Unlock()
	ch, ok := m[id]
	if ok {
		delete(m, id)
	}
	return ch, ok
}

// awaitResolution parks a tool handler on ch until an answer arrives or ctx is
// cancelled. On cancel it take-and-removes its own entry: if it wins (the entry
// is still present) it reports cancellation (ok=false); if an answerer already
// removed the entry, that answerer is guaranteed to send exactly one value into
// the cap-1 buffer, so it drains and returns that value. This makes the answerer
// and the handler agree on exactly one resolution — an answer sent as the
// handler cancels is honored, never dropped into a readerless buffer.
func awaitResolution[T any](ctx context.Context, mu *sync.Mutex, m map[string]chan T, id string, ch chan T) (T, bool) {
	select {
	case v := <-ch:
		return v, true
	case <-ctx.Done():
		if _, mine := takeResolution(mu, m, id); mine {
			var zero T
			return zero, false
		}
		return <-ch, true
	}
}

// AnswerQuestion resolves the pending ask_user_question call identified by
// id with the user's selections. Returns an error if no call is waiting
// (the call may have been cancelled, already answered, or the id may be
// bogus). Safe to call from any goroutine.
func (m *MCPService) AnswerQuestion(id string, answers, notes map[string]string) error {
	ch, ok := takeResolution(&m.questionsMu, m.questions, id)
	if !ok {
		return fmt.Errorf("no pending question with id %q", id)
	}
	// Guaranteed non-blocking: take-and-remove makes this the sole owner of the
	// cap-1 channel, and the handler either reads it or (on cancel) drains it.
	ch <- questionAnswer{Answers: answers, Notes: notes}
	return nil
}

// DeliverScreenshot resolves the pending screenshot_viewport call with
// the captured PNG. dataURL may be either a raw base64 string or a
// "data:image/png;base64,..." URL — both are normalised. Pass errMsg
// non-empty (with png nil) to fail the tool when the frontend can't
// capture.
func (m *MCPService) DeliverScreenshot(id, dataURL, errMsg string) error {
	// Build the result BEFORE taking the resolution token: a bad-base64 delivery
	// must fail without removing the parked entry, or the handler's cancel-drain
	// would block forever on a channel that never receives a value.
	var res screenshotResult
	if errMsg != "" {
		res.Err = errMsg
	} else {
		// Accept both "data:image/png;base64,..." and bare base64. The
		// frontend's toDataURL produces the former; strip the prefix so
		// the bytes we hand to ImageContent are the raw PNG.
		raw := dataURL
		if i := strings.Index(raw, ","); i >= 0 && strings.HasPrefix(raw, "data:") {
			raw = raw[i+1:]
		}
		decoded, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return fmt.Errorf("decode screenshot for %q: %w", id, err)
		}
		res.PNG = decoded
	}

	ch, ok := takeResolution(&m.screenshotsMu, m.screenshots, id)
	if !ok {
		return fmt.Errorf("no pending screenshot with id %q", id)
	}
	ch <- res
	return nil
}

// requestPermission surfaces an Allow/Deny card in the assistant panel for a
// tool the model wants to use and blocks until the user decides (or ctx is
// cancelled). rememberKey identifies the tool+target for "remember for this
// session"; pass "" to disable remembering. summary is the human-readable line
// shown on the card. On ctx cancellation it returns a deny.
func (m *MCPService) requestPermission(ctx context.Context, toolName, summary, rememberKey string) permissionDecision {
	if rememberKey != "" {
		m.rememberedMu.Lock()
		_, ok := m.remembered[rememberKey]
		m.rememberedMu.Unlock()
		if ok {
			return permissionDecision{Allow: true, Remember: true}
		}
	}

	// If already cancelled, don't prompt. Also keeps requestPermission
	// unit-testable without a live Wails event context.
	select {
	case <-ctx.Done():
		return permissionDecision{Allow: false}
	default:
	}

	id, err := generateToken()
	if err != nil {
		// A local token failure is a hard error; deny rather than allow.
		return permissionDecision{Allow: false}
	}
	ch := make(chan permissionDecision, 1)
	m.permissionsMu.Lock()
	m.permissions[id] = ch
	m.permissionsMu.Unlock()
	defer func() {
		m.permissionsMu.Lock()
		delete(m.permissions, id)
		m.permissionsMu.Unlock()
	}()

	wailsRuntime.EventsEmit(m.eventCtx, "assistant:permission-request", map[string]any{
		"id":       id,
		"toolName": toolName,
		"summary":  summary,
	})

	d, ok := awaitResolution(ctx, &m.permissionsMu, m.permissions, id, ch)
	if !ok {
		return permissionDecision{Allow: false}
	}
	if d.Allow && d.Remember && rememberKey != "" {
		m.rememberedMu.Lock()
		m.remembered[rememberKey] = struct{}{}
		m.rememberedMu.Unlock()
	}
	return d
}

// AnswerPermission resolves the pending permission request identified by id
// with the user's decision. Returns an error if no request is waiting (the
// call may have been cancelled, already answered, or the id may be bogus).
func (m *MCPService) AnswerPermission(id string, allow, remember bool) error {
	ch, ok := takeResolution(&m.permissionsMu, m.permissions, id)
	if !ok {
		return fmt.Errorf("no pending permission request with id %q", id)
	}
	ch <- permissionDecision{Allow: allow, Remember: remember}
	return nil
}

// ClearRememberedPermissions drops all session-remembered approvals so a new
// conversation re-prompts. Called from ClearHistory.
func (m *MCPService) ClearRememberedPermissions() {
	m.rememberedMu.Lock()
	m.remembered = make(map[string]struct{})
	m.rememberedMu.Unlock()
}

// SetContext latches the per-run editor state for MCP tools: current code,
// the active tab's path, and whether that tab is read-only. Satisfies
// AssistantMCPBridge.
func (m *MCPService) SetContext(code, activeTabPath string, readOnly bool) {
	m.state.setContext(code, activeTabPath, readOnly)
}

// RecordRun stores the most recent /eval summary so the get_last_run MCP
// tool can report it. Called by handleEval on both success and error paths.
func (m *MCPService) RecordRun(r runSummary) {
	m.state.setLastRun(r)
}

// buildServer constructs the MCP server and registers all assistant tools
// (which read/write the editor context and answer questions/permissions). The
// HTTPServer mounts the returned server at /mcp and owns the listener + token.
func (m *MCPService) buildServer(ctx context.Context) *mcp.Server {
	m.eventCtx = ctx

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "facet-gui",
		Version: "1.0.0",
	}, nil)

	state := m.state

	// --- Tool: get_editor_code ---
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_editor_code",
		Description: "Return the current source code in the Facet editor.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input getEditorCodeInput) (*mcp.CallToolResult, any, error) {
		code := state.getEditorCode()
		if code == "" {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "(editor is empty)"}},
			}, nil, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: code}},
		}, nil, nil
	})

	// --- Tool: edit_code ---
	mcp.AddTool(server, &mcp.Tool{
		Name:        "edit_code",
		Description: "Apply a search/replace edit to the editor code. The search string must match exactly (verbatim, including whitespace). Returns the updated code on success. Fails if the current file is read-only — in that case, use new_file to create an editable copy.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input editCodeInput) (*mcp.CallToolResult, any, error) {
		if input.Search == "" {
			return nil, nil, fmt.Errorf("search string must not be empty")
		}

		if ro, path := state.isReadOnly(); ro {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("The current file (%s) is read-only (stdlib, library, or example). Use new_file to create an editable copy, then edit that.", path)}},
			}, nil, nil
		}

		// Hold the lock across the whole read-modify-write so a concurrent edit
		// between the read and the write can't be silently clobbered (lost update).
		state.mu.Lock()
		code := state.editorCode
		idx := strings.Index(code, input.Search)
		if idx < 0 {
			state.mu.Unlock()
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: "Search text not found in editor. Make sure it matches the code exactly, including whitespace and newlines."}},
			}, nil, nil
		}
		newCode := code[:idx] + input.Replace + code[idx+len(input.Search):]
		state.editorCode = newCode
		state.mu.Unlock()

		wailsRuntime.EventsEmit(m.eventCtx, "assistant:replace-code", newCode)

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "Edit applied successfully."}},
		}, nil, nil
	})

	// --- Tool: replace_code ---
	mcp.AddTool(server, &mcp.Tool{
		Name:        "replace_code",
		Description: "Replace the entire editor content with new source code. Use this for new programs or major rewrites. The editor will auto-run the new code. Fails if the current file is read-only — in that case, use new_file instead.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input replaceCodeInput) (*mcp.CallToolResult, any, error) {
		if input.Code == "" {
			return nil, nil, fmt.Errorf("code must not be empty")
		}

		if ro, path := state.isReadOnly(); ro {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("The current file (%s) is read-only (stdlib, library, or example). Use new_file to create an editable copy instead.", path)}},
			}, nil, nil
		}

		state.mu.Lock()
		state.editorCode = input.Code
		state.mu.Unlock()

		wailsRuntime.EventsEmit(m.eventCtx, "assistant:replace-code", input.Code)

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "Code replaced. Editor will auto-run."}},
		}, nil, nil
	})

	// --- Tool: new_file ---
	mcp.AddTool(server, &mcp.Tool{
		Name:        "new_file",
		Description: "Create a new editable file in the editor and switch to it. Use this when the current file is read-only (stdlib, library, example) or when the user wants their changes in a separate file rather than overwriting the current one. The new file becomes the active tab and the editor auto-runs it.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input newFileInput) (*mcp.CallToolResult, any, error) {
		if input.Code == "" {
			return nil, nil, fmt.Errorf("code must not be empty")
		}
		name := strings.TrimSpace(input.Name)
		if name == "" {
			name = "Untitled"
		}
		if !strings.HasSuffix(name, ".fct") {
			name += ".fct"
		}

		// Latch the new file as the active tab so subsequent edit_code /
		// replace_code calls in the same turn target it (not the previous,
		// possibly read-only tab).
		state.mu.Lock()
		state.editorCode = input.Code
		state.readOnly = false
		// activeTabPath is updated by the frontend via the event round-trip;
		// leaving it as-is is fine because readOnly=false is the only thing
		// guards check. The frontend will refresh it on the next Send.
		state.mu.Unlock()

		wailsRuntime.EventsEmit(m.eventCtx, "assistant:new-file", map[string]string{
			"name": name,
			"code": input.Code,
		})

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "Created new file " + name + ". It is now the active editable tab."}},
		}, nil, nil
	})

	// --- Tool: check_syntax ---
	mcp.AddTool(server, &mcp.Tool{
		Name:        "check_syntax",
		Description: "Parse and type-check Facet source code without running it. Returns validation errors or confirms the code is valid. If source is omitted, checks the current editor code.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input checkSyntaxInput) (*mcp.CallToolResult, any, error) {
		source := input.Source
		if source == "" {
			source = state.getEditorCode()
		}
		if source == "" {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: `{"valid":false,"errors":[{"message":"no source code"}]}`}},
			}, nil, nil
		}
		result := checkSource(ctx, source, "check.fct")
		body, err := json.Marshal(result)
		if err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: "check failed: " + err.Error()}},
			}, nil, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(body)}},
		}, nil, nil
	})

	// --- Tool: get_documentation ---
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_documentation",
		Description: "Return Facet documentation. By default returns everything (language spec, color guide, stdlib API reference, library catalog) — roughly 40-60 KB. Pass `section` to fetch one section. Pass `query` to filter stdlib/library entries by name (e.g. query='Cylinder' returns just that function and its overloads). Prefer a targeted query when you know the name you're looking up.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input getDocumentationInput) (*mcp.CallToolResult, any, error) {
		result := buildDocumentationResponse(input.Section, input.Query)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: result}},
		}, nil, nil
	})

	// --- Tool: list_examples ---
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_examples",
		Description: "Return the list of built-in Facet example programs with a one-line summary for each (pulled from the first comment in the file). Use this to discover example names; fetch full source with get_example.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input listExamplesInput) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: listExamplesResponse()}},
		}, nil, nil
	})

	// --- Tool: get_example ---
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_example",
		Description: "Return the full source code of a built-in example by filename. Names come from list_examples. Examples are read-only — use replace_code or new_file to modify their contents into the editor.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input getExampleInput) (*mcp.CallToolResult, any, error) {
		if input.Name == "" {
			return nil, nil, fmt.Errorf("name must not be empty")
		}
		if !validExampleName(input.Name) {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("invalid example name: %q", input.Name)}},
			}, nil, nil
		}
		data, err := examples.FS.ReadFile(input.Name)
		if err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("example not found: %q (call list_examples for available names)", input.Name)}},
			}, nil, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
		}, nil, nil
	})

	// --- Tool: format_code ---
	mcp.AddTool(server, &mcp.Tool{
		Name:        "format_code",
		Description: "Format Facet source code with the canonical formatter (4-space indent, 80-column width). Returns the formatted source. If source is omitted, formats the current editor code. Fails with a parse error when the input is not valid Facet — formatting does not fix syntax errors.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input formatCodeInput) (*mcp.CallToolResult, any, error) {
		source := input.Source
		if source == "" {
			source = state.getEditorCode()
		}
		if source == "" {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: "no source code to format"}},
			}, nil, nil
		}
		src, err := parser.Parse(source, "", parser.SourceUser)
		if err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("parse failed — cannot format invalid source:\n%s", err.Error())}},
			}, nil, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: formatter.Format(src)}},
		}, nil, nil
	})

	// --- Tool: ask_user_question ---
	// Mirrors Claude Code's built-in AskUserQuestion. The built-in is
	// auto-denied when the CLI runs with `-p` (no interactive TTY), so
	// this tool gives the model a working alternative that surfaces the
	// question in the assistant panel and blocks until the user answers.
	mcp.AddTool(server, &mcp.Tool{
		Name:        "ask_user_question",
		Description: "Use this tool only when you are blocked on a decision that is genuinely the user's to make — one you cannot resolve from the request, the code, or sensible defaults. Each question gets 2-4 mutually-exclusive options; users will always be able to pick 'Other' to provide custom text. Set multiSelect=true when the options are not mutually exclusive. Returns JSON {answers, notes} keyed by the question text.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input askUserQuestionInput) (*mcp.CallToolResult, any, error) {
		if len(input.Questions) == 0 {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: "must supply at least one question"}},
			}, nil, nil
		}
		if len(input.Questions) > 4 {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: "max 4 questions per call"}},
			}, nil, nil
		}
		id, err := generateToken()
		if err != nil {
			return nil, nil, fmt.Errorf("generate question id: %w", err)
		}
		// Buffered cap 1 so AnswerQuestion can send without coordinating
		// with this goroutine's select. Removing the entry from m.questions
		// before reading guarantees only one send ever lands.
		ch := make(chan questionAnswer, 1)
		m.questionsMu.Lock()
		m.questions[id] = ch
		m.questionsMu.Unlock()
		defer func() {
			m.questionsMu.Lock()
			delete(m.questions, id)
			m.questionsMu.Unlock()
		}()

		wailsRuntime.EventsEmit(m.eventCtx, "assistant:question", map[string]any{
			"id":        id,
			"questions": input.Questions,
		})

		ans, ok := awaitResolution(ctx, &m.questionsMu, m.questions, id, ch)
		if !ok {
			// Cancellation propagates up through the assistant stream;
			// surface a brief tool error so the model knows the user
			// didn't decide.
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: "user cancelled before answering"}},
			}, nil, nil
		}
		body, err := json.Marshal(ans)
		if err != nil {
			return nil, nil, fmt.Errorf("marshal answer: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(body)}},
		}, nil, nil
	})

	// --- Tool: screenshot_viewport ---
	// Returns the live 3D viewport as a PNG so the model can SEE what
	// the user has rendered. Same channel-block pattern as
	// ask_user_question: emit an event, park on a buffered channel
	// until the frontend echoes the captured PNG back via the
	// DeliverViewportScreenshot Wails binding. The PNG bytes ride
	// through ImageContent which encodes them base64 on the wire.
	mcp.AddTool(server, &mcp.Tool{
		Name:        "screenshot_viewport",
		Description: "Capture the 3D viewport as a PNG. By default returns the user's live view; supply azimuth/elevation/distance (and optional target) to render off-screen from a chosen pose without moving the user's camera — useful for inspecting the back or underside. Use this to verify how your edit actually looks (not just stats); call after meaningful edits, not constantly.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input screenshotViewportInput) (*mcp.CallToolResult, any, error) {
		id, err := generateToken()
		if err != nil {
			return nil, nil, fmt.Errorf("generate screenshot id: %w", err)
		}
		ch := make(chan screenshotResult, 1)
		m.screenshotsMu.Lock()
		m.screenshots[id] = ch
		m.screenshotsMu.Unlock()
		defer func() {
			m.screenshotsMu.Lock()
			delete(m.screenshots, id)
			m.screenshotsMu.Unlock()
		}()

		payload := map[string]any{"id": id}
		if input.Azimuth != nil {
			payload["azimuth"] = *input.Azimuth
		}
		if input.Elevation != nil {
			payload["elevation"] = *input.Elevation
		}
		if input.Distance != nil {
			payload["distance"] = *input.Distance
		}
		if input.Target != nil {
			payload["target"] = map[string]float64{"x": input.Target.X, "y": input.Target.Y, "z": input.Target.Z}
		}
		wailsRuntime.EventsEmit(m.eventCtx, "assistant:screenshot-request", payload)

		res, ok := awaitResolution(ctx, &m.screenshotsMu, m.screenshots, id, ch)
		if !ok {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: "screenshot cancelled before capture"}},
			}, nil, nil
		}
		if res.Err != "" {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: "viewport capture failed: " + res.Err}},
			}, nil, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.ImageContent{Data: res.PNG, MIMEType: "image/png"},
			},
		}, nil, nil
	})

	// --- Tool: update_task_plan ---
	// One-way: the model posts its working task list and the assistant
	// panel renders/updates a checklist. No channel block — the model
	// can keep working immediately. Each call REPLACES the previous
	// list; the model is expected to send the complete current state.
	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_task_plan",
		Description: "Post or update a checklist of steps you intend to do, shown live to the user. Use for multi-step builds (3+ discrete steps) so the user can see progress. Each call REPLACES the list — send the full current state, not a delta. Statuses: 'pending', 'in_progress' (exactly one at a time), 'completed'. Don't use for single-step requests.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input updateTaskPlanInput) (*mcp.CallToolResult, any, error) {
		// Mild validation: each item needs a status the frontend knows
		// how to render. Unknown values would silently render as plain
		// text — better to reject so the model fixes its call.
		for i, t := range input.Tasks {
			switch t.Status {
			case "pending", "in_progress", "completed":
			default:
				return &mcp.CallToolResult{
					IsError: true,
					Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("task %d has invalid status %q (want pending/in_progress/completed)", i, t.Status)}},
				}, nil, nil
			}
		}
		wailsRuntime.EventsEmit(m.eventCtx, "assistant:task-plan", map[string]any{"tasks": input.Tasks})
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("task plan updated (%d items)", len(input.Tasks))}},
		}, nil, nil
	})

	// --- Tool: get_last_run ---
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_last_run",
		Description: "Return a summary of the most recent Facet evaluation: triangle/vertex counts, bounding box, entry point, errors, and a ranAt timestamp. Use this after edit_code / replace_code / new_file to verify what actually rendered. Note: this reports the LAST evaluation, which may reflect a user edit made after your change, or may still show the previous run if the editor has not finished re-evaluating. Check the ranAt timestamp to judge freshness. Returns null if no evaluation has completed this session.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input getLastRunInput) (*mcp.CallToolResult, any, error) {
		summary := state.getLastRun()
		if summary == nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "null"}},
			}, nil, nil
		}
		body, err := json.Marshal(summary)
		if err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: "marshal failed: " + err.Error()}},
			}, nil, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(body)}},
		}, nil, nil
	})

	// --- Tool: request_permission ---
	// Registered as the CLI's --permission-prompt-tool. The CLI calls this for
	// every tool use that is NOT pre-approved by --allowedTools (i.e. built-ins
	// like WebSearch/WebFetch/Bash). It surfaces an Allow/Deny card and returns
	// the CLI's required permission verdict JSON.
	mcp.AddTool(server, &mcp.Tool{
		Name:        "request_permission",
		Description: "Internal permission gate invoked by the CLI when a tool needs user approval. Not for direct use by the model.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input requestPermissionInput) (*mcp.CallToolResult, any, error) {
		d := m.requestPermission(ctx,
			input.ToolName,
			permissionSummary(input.ToolName, input.Input),
			permissionRememberKey(input.ToolName, input.Input),
		)
		var verdict map[string]any
		if d.Allow {
			verdict = map[string]any{"behavior": "allow", "updatedInput": input.Input}
		} else {
			verdict = map[string]any{"behavior": "deny", "message": "User denied permission for " + input.ToolName}
		}
		body, err := json.Marshal(verdict)
		if err != nil {
			return nil, nil, fmt.Errorf("marshal permission verdict: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(body)}},
		}, nil, nil
	})

	// --- Tool: fetch_url ---
	// Fetch a URL and return its content: images (png/jpeg/gif/webp) come back
	// as a viewable image (vision); svg/text/json/xml/js come back as text.
	// Network egress is gated through the same permission card as the CLI
	// bridge (per-host, remember-for-session). Hard-errors on unsupported
	// content types, oversize images, bad scheme, SSRF-blocked hosts, non-2xx.
	mcp.AddTool(server, &mcp.Tool{
		Name:        "fetch_url",
		Description: "Fetch a URL and return its contents. Images (png/jpeg/gif/webp) are returned as an image you can SEE; text/JSON/XML/SVG are returned as text. Use this to look at an image from the web or read page content. The user is asked to approve network access the first time per site.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input fetchURLInput) (*mcp.CallToolResult, any, error) {
		parsed, err := validateFetchURL(input.URL)
		if err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			}, nil, nil
		}

		// Self-gate: surface an Allow/Deny card before egress.
		d := m.requestPermission(ctx, "fetch_url", "Fetch from the web: "+input.URL, "fetch_url:"+parsed.Host)
		if !d.Allow {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: "User denied network access to " + parsed.Host}},
			}, nil, nil
		}

		res, err := fetchContent(ctx, newFetchClient(), input.URL)
		if err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			}, nil, nil
		}

		if res.IsImage {
			header := fmt.Sprintf("Fetched %s (HTTP %d, %s, %d bytes)", res.FinalURL, res.Status, res.MIME, len(res.Data))
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: header},
					&mcp.ImageContent{Data: res.Data, MIMEType: res.MIME},
				},
			}, nil, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: res.Text}},
		}, nil, nil
	})

	// --- GUI automation tools ---
	// These drive the live GUI through the shared automation registry (the same
	// commands the /control route exposes). Available to the in-app assistant;
	// reachable by external drivers only when --automation disables auth.
	mcp.AddTool(server, &mcp.Tool{
		Name:        "gui_set_camera",
		Description: "Rotate the 3D viewer camera to an azimuth/elevation (degrees).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input guiSetCameraInput) (*mcp.CallToolResult, any, error) {
		return m.invokeGUI(ctx, "viewer.setCamera", input)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "gui_record_start",
		Description: "Start recording the app to a video file. mode is 'canvas' (3D viewer) or 'page' (full UI).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input guiRecordStartInput) (*mcp.CallToolResult, any, error) {
		return m.invokeGUI(ctx, "record.start", input)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "gui_record_stop",
		Description: "Stop the current recording and return the saved video file path.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input guiRecordStopInput) (*mcp.CallToolResult, any, error) {
		return m.invokeGUI(ctx, "record.stop", input)
	})

	return server
}

// invokeGUI marshals a typed tool input to JSON params and drives the named GUI
// command through the automation controller. A command failure becomes an
// IsError tool result (a Go error would read as a protocol fault to the SDK).
// The command's JSON return value, when present, is echoed as the result text
// so gui_record_stop can hand back the saved path.
func (m *MCPService) invokeGUI(ctx context.Context, name string, input any) (*mcp.CallToolResult, any, error) {
	params, err := json.Marshal(input)
	if err != nil {
		return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}}}, nil, nil
	}
	value, err := m.automation.Invoke(ctx, name, params)
	if err != nil {
		return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}}}, nil, nil
	}
	text := name + " ok"
	if len(value) > 0 && string(value) != "null" {
		text = string(value)
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
}
