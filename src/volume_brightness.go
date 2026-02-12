package main

import (
	"fmt"
	"time"
)

// adjustVolume changes system volume and shows overlay
func (app *MiyooPod) adjustVolume(delta int) {
	newVolume := clamp(app.SystemVolume+delta, 0, 100)

	// Always max SDL2_mixer volume
	audioSetVolume(100)

	// Set MI_AO system volume
	setMiAOVolume(newVolume)
	app.SystemVolume = newVolume

	app.showOverlay("volume", newVolume)

	// Persist to settings
	go app.saveSettings()

	logMsg(fmt.Sprintf("Volume: %d%%", newVolume))
}

// adjustBrightness changes screen brightness and shows overlay
func (app *MiyooPod) adjustBrightness(delta int) {
	newBrightness := clamp(app.SystemBrightness+delta, 10, 100) // Min 10% so screen stays visible

	setBrightness(newBrightness)
	app.SystemBrightness = newBrightness

	app.showOverlay("brightness", newBrightness)

	// Persist to settings
	go app.saveSettings()

	logMsg(fmt.Sprintf("Brightness: %d%%", newBrightness))
}

// showOverlay displays the volume/brightness overlay for 2 seconds
func (app *MiyooPod) showOverlay(overlayType string, value int) {
	// Cancel existing timer
	if app.OverlayTimer != nil {
		app.OverlayTimer.Stop()
	}

	app.OverlayType = overlayType
	app.OverlayValue = value
	app.OverlayVisible = true

	// Signal main loop to redraw (non-blocking to avoid deadlock)
	app.requestRedraw()

	// Hide after 2 seconds
	app.OverlayTimer = time.AfterFunc(2*time.Second, func() {
		app.OverlayVisible = false
		app.requestRedraw()
	})
}

func clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
