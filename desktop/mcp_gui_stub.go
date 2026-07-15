//go:build !automation

package main

import "github.com/modelcontextprotocol/go-sdk/mcp"

// registerGUITools is a no-op in non-automation builds: the demo remote-control
// bus is compiled out, so the shipped app registers none of the gui_* tools.
func (m *MCPService) registerGUITools(server *mcp.Server) {}
