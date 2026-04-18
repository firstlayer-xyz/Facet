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

	"github.com/modelcontextprotocol/go-sdk/mcp"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// mcpState holds editor state shared between the MCP server tools and the app.
// The assistant sets the current editor code before each run so MCP tools
// (get_editor_code, edit_code, replace_code, check_syntax) can operate on
// it without taking a fresh RPC round-trip to the frontend.
type mcpState struct {
	mu         sync.Mutex
	editorCode string // current editor content, set before each assistant call
}

func newMCPState() *mcpState {
	return &mcpState{}
}

func (s *mcpState) setEditorCode(code string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.editorCode = code
}

func (s *mcpState) getEditorCode() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.editorCode
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
// until Start is called.
func NewMCPService(eval *EvalService) *MCPService {
	return &MCPService{
		eval:  eval,
		state: newMCPState(),
	}
}

// Endpoint returns the port and bearer token for the localhost HTTP server.
// Satisfies AssistantMCPBridge so AssistantService can wire MCP credentials
// into Claude Code without a circular dependency.
func (m *MCPService) Endpoint() (int, string) { return m.port, m.token }

// SetEditorCode updates the editor code that MCP tools see. Satisfies
// AssistantMCPBridge.
func (m *MCPService) SetEditorCode(code string) {
	m.state.setEditorCode(code)
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
		Description: "Apply a search/replace edit to the editor code. The search string must match exactly (verbatim, including whitespace). Returns the updated code on success.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input editCodeInput) (*mcp.CallToolResult, any, error) {
		if input.Search == "" {
			return nil, nil, fmt.Errorf("search string must not be empty")
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
		Description: "Replace the entire editor content with new source code. Use this for new programs or major rewrites. The editor will auto-run the new code.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input replaceCodeInput) (*mcp.CallToolResult, any, error) {
		if input.Code == "" {
			return nil, nil, fmt.Errorf("code must not be empty")
		}

		state.mu.Lock()
		state.editorCode = input.Code
		state.mu.Unlock()

		wailsRuntime.EventsEmit(m.eventCtx, "assistant:replace-code", input.Code)

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "Code replaced. Editor will auto-run."}},
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
