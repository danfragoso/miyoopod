package main

import "fmt"

// setTheme changes the current theme and refreshes the screen
func (app *MiyooPod) setTheme(theme Theme) {
	app.CurrentTheme = theme
	app.NPCacheDirty = true // Force Now Playing screen to re-render
	app.drawCurrentScreen()

	// Save theme preference to library
	if err := app.saveLibraryJSON(); err != nil {
		logMsg(fmt.Sprintf("Failed to save theme preference: %v", err))
	}
}
