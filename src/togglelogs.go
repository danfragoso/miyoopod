package main

import (
	"fmt"
	"time"
)

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

// cycleAutoLock cycles through auto-lock options: 1min -> 3min -> 5min -> 10min -> Off -> 1min...
func (app *MiyooPod) cycleAutoLock() {
	switch app.AutoLockMinutes {
	case 1:
		app.AutoLockMinutes = 3
	case 3:
		app.AutoLockMinutes = 5
	case 5:
		app.AutoLockMinutes = 10
	case 10:
		app.AutoLockMinutes = 0
	default:
		app.AutoLockMinutes = 1
	}

	// Reset activity timer so the new timeout starts fresh
	app.LastActivityTime = time.Now()

	// Rebuild the settings menu to update the label
	app.RootMenu = app.buildRootMenu()
	app.MenuStack = []*MenuScreen{app.RootMenu}

	for _, item := range app.RootMenu.Items {
		if item.Label == "Settings" {
			app.MenuStack = append(app.MenuStack, item.Submenu)
			break
		}
	}

	app.drawCurrentScreen()

	if err := app.saveSettings(); err != nil {
		logMsg(fmt.Sprintf("ERROR: Failed to save auto-lock preference: %v", err))
	}
}

// toggleScreenPeek toggles the screen peek while locked setting
func (app *MiyooPod) toggleScreenPeek() {
	app.ScreenPeekEnabled = !app.ScreenPeekEnabled

	// Rebuild the settings menu to update the label
	app.RootMenu = app.buildRootMenu()
	app.MenuStack = []*MenuScreen{app.RootMenu}

	for _, item := range app.RootMenu.Items {
		if item.Label == "Settings" {
			app.MenuStack = append(app.MenuStack, item.Submenu)
			break
		}
	}

	app.drawCurrentScreen()

	if err := app.saveSettings(); err != nil {
		logMsg(fmt.Sprintf("ERROR: Failed to save screen peek preference: %v", err))
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
