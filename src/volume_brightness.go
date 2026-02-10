package main

import (
	"fmt"
	"time"
)

// adjustVolume changes system volume and shows overlay
func (app *MiyooPod) adjustVolume(delta int) {
	currentVolume := app.OverlayValue // Use overlay value for system volume
	newVolume := clamp(currentVolume+delta, 0, 100)

	// Always max SDL2_mixer volume
	audioSetVolume(100)

	// Set MI_AO system volume
	setMiAOVolume(newVolume)

	app.showOverlay("volume", newVolume)

	logMsg(fmt.Sprintf("Volume: %d%%", newVolume))
}

// adjustBrightness changes screen brightness and shows overlay
func (app *MiyooPod) adjustBrightness(delta int) {
	currentBrightness := getBrightness()
	newBrightness := clamp(currentBrightness+delta, 10, 100) // Min 10% so screen stays visible

	setBrightness(newBrightness)
	app.showOverlay("brightness", newBrightness)

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

	// Redraw screen with overlay
	app.drawCurrentScreen()

	// Hide after 2 seconds
	app.OverlayTimer = time.AfterFunc(2*time.Second, func() {
		app.OverlayVisible = false
		app.drawCurrentScreen()
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
