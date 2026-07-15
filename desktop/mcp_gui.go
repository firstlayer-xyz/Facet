//go:build automation

package main

import (
	"context"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// gui_* tool inputs drive the GUI via the automation registry (same commands
// the /control route exposes).
type guiSetCameraInput struct {
	Azimuth   float64 `json:"azimuth" jsonschema:"Camera azimuth in degrees (around the up axis)."`
	Elevation float64 `json:"elevation" jsonschema:"Camera elevation in degrees above the horizontal."`
	Distance  float64 `json:"distance,omitempty" jsonschema:"Distance from the target. Omit to keep the current distance."`
}

type guiRecordStartInput struct {
	Mode   string `json:"mode" jsonschema:"Capture surface: 'canvas' (3D viewer only) or 'page' (full UI)."`
	FPS    int    `json:"fps,omitempty" jsonschema:"Frames per second. Omit for 30."`
	Width  int    `json:"width,omitempty" jsonschema:"Output video width in pixels (page mode only). Omit for the window's native size."`
	Height int    `json:"height,omitempty" jsonschema:"Output video height in pixels (page mode only). Omit for the window's native size."`
	Name   string `json:"name,omitempty" jsonschema:"Optional label used as the video filename prefix (for organizing recordings)."`
}

type guiRecordStopInput struct{}

type guiSetWindowSizeInput struct {
	Width  int `json:"width" jsonschema:"App window width in points."`
	Height int `json:"height" jsonschema:"App window height in points."`
}

type guiLoadCodeInput struct {
	Code string `json:"code" jsonschema:"Facet source to load into the active editor tab; the build runs and the result renders before this returns."`
}

// registerGUITools adds the gui_* tools to the MCP server. They drive the live
// GUI through the shared automation registry (the same commands the /control
// route exposes). Available to the in-app assistant; reachable by external
// drivers only when --automation disables auth. Automation-build only.
func (m *MCPService) registerGUITools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "gui_set_camera",
		Description: "Rotate the 3D viewer camera to an azimuth/elevation (degrees).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input guiSetCameraInput) (*mcp.CallToolResult, any, error) {
		return m.invokeGUI(ctx, "viewer.setCamera", input)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "gui_load_code",
		Description: "Load Facet source into the editor and build it; returns once the result has rendered.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input guiLoadCodeInput) (*mcp.CallToolResult, any, error) {
		return m.invokeGUI(ctx, "editor.loadCode", input)
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

	mcp.AddTool(server, &mcp.Tool{
		Name:        "gui_set_window_size",
		Description: "Resize the app window (points). Lays out the whole UI at that size so recordings frame to it.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input guiSetWindowSizeInput) (*mcp.CallToolResult, any, error) {
		return m.invokeGUI(ctx, "window.setSize", input)
	})
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
