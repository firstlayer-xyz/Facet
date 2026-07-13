package main

import (
	"embed"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend/dist
var assets embed.FS

// AutomationConfig is the parsed --automation[=PORT] flag. When Enabled, the
// HTTP server binds the fixed Port and disables bearer-token auth so an
// external driver can connect without discovering a token (see authMiddleware).
type AutomationConfig struct {
	Enabled bool
	Port    int
}

const defaultAutomationPort = 8791

// parseAutomationFlag reads --automation / --automation=PORT from args. An
// unparseable or non-positive port falls back to the default port (the flag is
// still enabled). Absent flag → disabled, zero port.
func parseAutomationFlag(args []string) AutomationConfig {
	for _, a := range args {
		if a == "--automation" {
			return AutomationConfig{Enabled: true, Port: defaultAutomationPort}
		}
		if strings.HasPrefix(a, "--automation=") {
			port := defaultAutomationPort
			if p, err := strconv.Atoi(strings.TrimPrefix(a, "--automation=")); err == nil && p > 0 {
				port = p
			}
			return AutomationConfig{Enabled: true, Port: port}
		}
	}
	return AutomationConfig{}
}

func main() {
	// Create an instance of the app structure
	app := NewApp()
	app.automationCfg = parseAutomationFlag(os.Args[1:])
	if app.automationCfg.Enabled {
		log.Printf("[automation] enabled on http://127.0.0.1:%d (auth disabled)", app.automationCfg.Port)
	}

	// Create application with options
	err := wails.Run(&options.App{
		Title:     "Untitled — Facet",
		Width:     1024,
		Height:    768,
		MinWidth:  400,
		MinHeight: 300,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		LogLevel:         logger.DEBUG,
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 255},
		OnStartup:        app.startup,
		OnBeforeClose:    app.beforeClose,
		OnShutdown:       app.shutdown,
		Menu:             app.buildMenu(),
		Bind: []interface{}{
			app,
		},
		Mac: &mac.Options{
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: true,
				HideTitle:                  true,
				HideTitleBar:               false,
				FullSizeContent:            true,
				UseToolbar:                 false,
				HideToolbarSeparator:       true,
			},
		},
	})

	if err != nil {
		log.Fatal("Error: ", err)
	}
}
