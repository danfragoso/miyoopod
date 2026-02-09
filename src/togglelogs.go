package main

import "fmt"

// toggleLocalLogs toggles the local logs setting
func (app *MiyooPod) toggleLocalLogs() {
	app.LocalLogsEnabled = !app.LocalLogsEnabled

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
		logMsg(fmt.Sprintf("ERROR: Failed to save log preference: %v", err))
	}
}

// toggleSentry toggles the Sentry (developer logs) setting
func (app *MiyooPod) toggleSentry() {
	app.SentryEnabled = !app.SentryEnabled

	// Update PostHog client state (don't log to avoid circular call)
	if posthogClient != nil {
		posthogClient.Enabled = app.SentryEnabled
	}

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
		logMsg(fmt.Sprintf("ERROR: Failed to save Sentry preference: %v", err))
	}
}
