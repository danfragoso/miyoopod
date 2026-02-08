package main

import "fmt"

// toggleWriteLogs toggles the write logs setting
func (app *MiyooPod) toggleWriteLogs() {
	app.WriteLogsEnabled = !app.WriteLogsEnabled

	// Rebuild the settings menu to update the label
	app.RootMenu = app.buildRootMenu()
	app.MenuStack = []*MenuScreen{app.RootMenu}

	// Navigate to settings menu
	for _, item := range app.RootMenu.Items {
		if item.Label == "Settings" {
			app.MenuStack = append(app.MenuStack, item.Submenu)
			break
		}
	}

	app.drawCurrentScreen()

	// Save preference to settings file
	if err := app.saveSettings(); err != nil {
		logMsg(fmt.Sprintf("Failed to save log preference: %v", err))
	}
}
