package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"facet/app/docs"
	"facet/app/pkg/fctlang/checker"
	"facet/app/pkg/fctlang/doc"
	"facet/app/pkg/fctlang/evaluator"
	"facet/app/pkg/fctlang/loader"
	"facet/app/pkg/fctlang/parser"
	"facet/app/pkg/manifold"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type EvalInput struct {
	Source     string `json:"source" jsonschema:"Facet source code"`
	OutputPath string `json:"outputPath" jsonschema:"Export file path (e.g. .stl, .obj, .3mf, .glb)"`
}

type CheckInput struct {
	Source string `json:"source" jsonschema:"Facet source code to check"`
}

type DocInput struct {
	Source string `json:"source,omitempty" jsonschema:"Optional Facet source code to include user-defined docs"`
}

func main() {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "facet",
		Version: "1.0.0",
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "evaluate",
		Description: "Evaluate Facet source code and export the resulting 3D model (STL, OBJ, 3MF, GLB, etc.)",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input EvalInput) (*mcp.CallToolResult, any, error) {
		ext := filepath.Ext(input.OutputPath)
		if ext == "" {
			return nil, nil, fmt.Errorf("output file must have an extension (e.g. .stl, .obj, .3mf, .glb)")
		}
		// Sanitize output path: reject path traversal, require relative path
		cleaned := filepath.Clean(input.OutputPath)
		if strings.Contains(cleaned, "..") {
			return nil, nil, fmt.Errorf("output path must not contain '..'")
		}
		if filepath.IsAbs(cleaned) {
			return nil, nil, fmt.Errorf("output path must be relative, not absolute")
		}
		input.OutputPath = cleaned

		src, err := parser.Parse(input.Source)
		if err != nil {
			return nil, nil, fmt.Errorf("parse error: %v", err)
		}
		prog := loader.Program{Sources: map[string]*parser.Source{"mcp-input": src}, Imports: make(map[string]string)}

		if errs := checker.Check(prog).Errors; len(errs) > 0 {
			return nil, nil, fmt.Errorf("type error: %v", errs[0])
		}

		result, err := evaluator.Eval(ctx, prog, "mcp-input", nil, "Main")
		if err != nil {
			return nil, nil, fmt.Errorf("eval error: %v", err)
		}

		if err := manifold.ExportMeshes(result.Solids, input.OutputPath); err != nil {
			return nil, nil, fmt.Errorf("export error: %v", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: input.OutputPath},
			},
		}, nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "check_syntax",
		Description: "Parse and type-check Facet source code without evaluating. Returns validation errors or confirms the code is valid.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input CheckInput) (*mcp.CallToolResult, any, error) {
		src, err := parser.Parse(input.Source)
		if err != nil {
			data, _ := json.Marshal(map[string]interface{}{
				"valid":  false,
				"errors": []map[string]interface{}{{"line": 0, "col": 0, "message": err.Error()}},
			})
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
			}, nil, nil
		}
		prog := loader.Program{Sources: map[string]*parser.Source{"mcp-input": src}, Imports: make(map[string]string)}
		errs := checker.Check(prog).Errors
		if len(errs) == 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: `{"valid":true,"errors":[]}`}},
			}, nil, nil
		}
		data, _ := json.Marshal(map[string]interface{}{"valid": false, "errors": errs})
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
		}, nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_documentation",
		Description: "Return the Facet language API documentation index including all stdlib functions, methods, types, and keywords. Optionally pass source code to include user-defined function docs.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input DocInput) (*mcp.CallToolResult, any, error) {
		entries := doc.BuildDocIndex(input.Source, nil)
		data, err := json.Marshal(entries)
		if err != nil {
			return nil, nil, err
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
		}, nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_language_spec",
		Description: "Return the complete Facet language specification (grammar, types, built-in functions) as Markdown",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input struct{}) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: docs.LanguageSpec}},
		}, nil, nil
	})

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
