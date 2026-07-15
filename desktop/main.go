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

// parseAutomationFlag lives in automation_flag.go (automation builds) and
// automation_flag_stub.go (all other builds, where it always returns a disabled
// config so the --automation flag is inert and can never drop the HTTP auth).

// parseWindowSize reads --window-size=WIDTHxHEIGHT (points) from args, returning
// (0,0) when absent or malformed. Pre-sizing the window at launch means a
// recording never captures a runtime resize.
func parseWindowSize(args []string) (w, h int) {
	const prefix = "--window-size="
	for _, a := range args {
		if !strings.HasPrefix(a, prefix) {
			continue
		}
		parts := strings.SplitN(strings.TrimPrefix(a, prefix), "x", 2)
		if len(parts) != 2 {
			return 0, 0
		}
		pw, err1 := strconv.Atoi(parts[0])
		ph, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil || pw <= 0 || ph <= 0 {
			return 0, 0
		}
		return pw, ph
	}
	return 0, 0
}

func main() {
	// Create an instance of the app structure
	app := NewApp()
	app.automationCfg = parseAutomationFlag(os.Args[1:])
	if app.automationCfg.Enabled {
		log.Printf("[automation] enabled on http://127.0.0.1:%d (auth disabled)", app.automationCfg.Port)
	}

	// Initial window size: --window-size=WxH overrides the default. Sizing at
	// launch keeps a runtime resize out of any recording.
	width, height := 1024, 768
	if w, h := parseWindowSize(os.Args[1:]); w > 0 && h > 0 {
		width, height = w, h
		log.Printf("[window] initial size %dx%d (from --window-size)", width, height)
	}

	// Create application with options
	err := wails.Run(&options.App{
		Title:     "Untitled — Facet",
		Width:     width,
		Height:    height,
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
