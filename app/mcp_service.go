package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"facet/app/docs"
	"facet/app/pkg/fctlang/parser"

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

type getDocumentationInput struct{}

type newFileInput struct {
	Name string `json:"name" jsonschema:"Label for the new file (e.g. 'plate-with-holes'). '.fct' is appended if missing."`
	Code string `json:"code" jsonschema:"Initial source code for the new file. Must be a complete Facet program if the user expects it to render."`
}

type getLastRunInput struct{}

// HTTPAuth is the payload returned to the frontend so it can authenticate
// requests to the localhost HTTP server.
type HTTPAuth struct {
	Port  int    `json:"port"`
	Token string `json:"token"`
}

// MCPService owns the localhost HTTP server that exposes the MCP endpoint
// (/mcp), the eval endpoint (/eval), and the syntax-check endpoint
// (/check). It also holds the bearer token shared across those endpoints
// and the editor-code state that MCP tools read and write.
//
// Start wires the /eval route through an EvalService so this service is
// decoupled from eval pipeline details — it only needs an http.Handler.
type MCPService struct {
	eval     *EvalService
	state    *mcpState
	port     int
	token    string
	eventCtx context.Context
}

// NewMCPService creates a new MCP service. The HTTP server is not started
// until Start is called. The MCPService registers itself as the run recorder
// on the EvalService so every /eval response updates the lastRun slot.
func NewMCPService(eval *EvalService) *MCPService {
	m := &MCPService{
		eval:  eval,
		state: newMCPState(),
	}
	eval.SetRunRecorder(m.RecordRun)
	return m
}

// Endpoint returns the port and bearer token for the localhost HTTP server.
// Satisfies AssistantMCPBridge so AssistantService can wire MCP credentials
// into Claude Code without a circular dependency.
func (m *MCPService) Endpoint() (int, string) { return m.port, m.token }

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

// Auth returns the port + bearer token as an HTTPAuth payload for the
// frontend.
func (m *MCPService) Auth() HTTPAuth {
	return HTTPAuth{Port: m.port, Token: m.token}
}

// Start creates the HTTP listener, registers the MCP tools and routes,
// and launches the server goroutine. The server is shut down when ctx is
// cancelled. Returns the port and token on success.
func (m *MCPService) Start(ctx context.Context) (int, string, error) {
	m.eventCtx = ctx

	token, err := generateToken()
	if err != nil {
		return 0, "", err
	}

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

		state.mu.Lock()
		code := state.editorCode
		state.mu.Unlock()

		idx := strings.Index(code, input.Search)
		if idx < 0 {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: "Search text not found in editor. Make sure it matches the code exactly, including whitespace and newlines."}},
			}, nil, nil
		}

		newCode := code[:idx] + input.Replace + code[idx+len(input.Search):]

		state.mu.Lock()
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
		Description: "Return the Facet language specification, color guide, and available library catalog. Call this when you need to look up syntax, functions, types, or library APIs.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input getDocumentationInput) (*mcp.CallToolResult, any, error) {
		var sb strings.Builder
		sb.WriteString(docs.LanguageSpec)
		sb.WriteString("\n\n")
		sb.WriteString(docs.ColorGuide)

		if catalog := formatLibraryCatalog(collectLibDocEntries()); catalog != "" {
			sb.WriteString("\n\n")
			sb.WriteString(catalog)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}},
		}, nil, nil
	})

	// --- Tool: get_last_run ---
	mcp.AddTool(server, &mcp.Tool{
		Name: "get_last_run",
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

	// Start HTTP listener first so we know the port before wiring middleware.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, "", fmt.Errorf("failed to start HTTP server: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	handleMcp := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{Stateless: true})

	mux := http.NewServeMux()
	mux.Handle("/mcp", handleMcp)
	mux.Handle("/eval", m.eval.HTTPHandler())
	mux.HandleFunc("/check", handleCheck)

	log.Printf("[http] server listening on http://127.0.0.1:%d (mcp, eval)", port)

	httpServer := &http.Server{Handler: authMiddleware(token, port, mux)}
	go func() {
		if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("[http] server error: %v", err)
		}
	}()

	go func() {
		<-ctx.Done()
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer shutCancel()
		httpServer.Shutdown(shutCtx)
	}()

	m.port = port
	m.token = token
	return port, token, nil
}
