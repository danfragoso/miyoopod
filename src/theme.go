package main

import "fmt"

// setTheme changes the current theme and refreshes the screen
func (app *MiyooPod) setTheme(theme Theme) {
	app.CurrentTheme = theme
	app.NPCacheDirty = true // Force Now Playing screen to re-render
	app.drawCurrentScreen()

	// Generate icon PNG with new theme colors
	if err := app.generateIconPNG(); err != nil {
		logMsg(fmt.Sprintf("Failed to generate themed icon: %v", err))
	}

	// Save theme preference to settings file (fast)
	if err := app.saveSettings(); err != nil {
		logMsg(fmt.Sprintf("Failed to save theme preference: %v", err))
	}
}
