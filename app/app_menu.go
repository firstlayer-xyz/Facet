package main

import (
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

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
	fileMenu.AddSeparator()
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

	// Open Example submenu
	demoMenu := fileMenu.AddSubmenu("Open Example")
	for _, name := range a.GetExampleList() {
		label := strings.TrimSuffix(name, ".fct")
		demoMenu.AddText(label, nil, func(_ *menu.CallbackData) {
			wailsRuntime.EventsEmit(a.ctx, "menu:open-demo", name)
		})
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
