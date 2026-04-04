package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"facet/app/docs"
	"facet/app/pkg/fctlang/doc"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// mcpState holds editor state shared between the MCP server tools and the app.
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

// startHTTPServer creates and starts a localhost HTTP server on a random port.
// It serves the MCP endpoint at /mcp and the eval endpoint at /eval.
func (a *App) startHTTPServer() (int, error) {
	state := newMCPState()
	a.mcpState = state

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "facet-gui",
		Version: "1.0.0",
	}, nil)

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

		wailsRuntime.EventsEmit(a.ctx, "assistant:replace-code", newCode)

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

		wailsRuntime.EventsEmit(a.ctx, "assistant:replace-code", input.Code)

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

		resp, err := http.Post(fmt.Sprintf("http://127.0.0.1:%d/check", a.mcpPort),
			"application/json",
			strings.NewReader(fmt.Sprintf(`{"source":%q}`, source)))
		if err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: "check failed: " + err.Error()}},
			}, nil, nil
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
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

		libEntries := buildLibraryCatalog()
		if len(libEntries) > 0 {
			sb.WriteString("\n\n## Available Libraries\n\n")
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
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}},
		}, nil, nil
	})

	// Start HTTP listener
	handleMcp := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{Stateless: true})

	mux := http.NewServeMux()
	mux.Handle("/mcp", handleMcp)
	mux.Handle("/eval", a.evalHTTPHandler())
	mux.HandleFunc("/check", handleCheck)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("failed to start HTTP server: %w", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	log.Printf("[http] server listening on http://127.0.0.1:%d (mcp, eval)", port)

	httpServer := &http.Server{Handler: mux}
	go func() {
		if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("[http] server error: %v", err)
		}
	}()

	go func() {
		<-a.ctx.Done()
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer shutCancel()
		httpServer.Shutdown(shutCtx)
	}()

	return port, nil
}
