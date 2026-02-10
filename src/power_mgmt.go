package main

import (
	"fmt"
	"os"
	"time"
)

// restoreBrightness sets brightness back to a visible level before app exits
func restoreBrightness() {
	// Set to mid-level brightness (duty_cycle range is typically 0-100)
	brightnessPath := "/sys/class/pwm/pwmchip0/pwm0/duty_cycle"
	defaultBrightness := "50"

	if err := os.WriteFile(brightnessPath, []byte(defaultBrightness), 0644); err != nil {
		logMsg(fmt.Sprintf("WARNING: Could not restore brightness: %v", err))
	} else {
		logMsg("Brightness restored to default level")
	}
}

// startInactivityMonitor runs in background and auto-locks screen after period of inactivity
func (app *MiyooPod) startInactivityMonitor() {
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Skip if auto-lock is disabled or already locked
			if app.AutoLockMinutes <= 0 || app.Locked {
				continue
			}

			// Check if inactive for specified duration
			inactiveDuration := time.Since(app.LastActivityTime)
			autoLockDuration := time.Duration(app.AutoLockMinutes) * time.Minute

			if inactiveDuration >= autoLockDuration {
				logMsg(fmt.Sprintf("Auto-lock triggered after %v of inactivity", inactiveDuration))
				app.toggleLock()
			}
		}
	}
}

// monitorPowerButtonHold monitors power button hold duration and forces quit if held 5+ seconds
func (app *MiyooPod) monitorPowerButtonHold() {
	startTime := app.PowerButtonPressTime

	// Check every 100ms
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// If button was released, stop monitoring
			if !app.PowerButtonPressed {
				return
			}

			holdDuration := time.Since(startTime)

			// Force shutdown after 5 seconds
			if holdDuration >= 5*time.Second {
				logMsg("Power button held for 5+ seconds - forcing shutdown")

				// Restore brightness before exiting
				restoreBrightness()

				TrackAction("force_shutdown", map[string]interface{}{
					"hold_duration": holdDuration.Seconds(),
				})

				// Set flag to exit cleanly
				app.Running = false
				return
			}
		}
	}
}

// resetInactivityTimer resets the inactivity timer (called on user interaction)
func (app *MiyooPod) resetInactivityTimer() {
	app.LastActivityTime = time.Now()
}

// peekScreen temporarily shows the screen when locked (3 seconds)
func (app *MiyooPod) peekScreen() {
	if !app.ScreenPeekEnabled {
		return
	}

	// Cancel any existing peek timer
	if app.ScreenPeekTimer != nil {
		app.ScreenPeekTimer.Stop()
	}

	// Restore brightness
	restoreBrightness()
	app.ScreenPeekActive = true

	// Redraw screen to show current state
	app.drawCurrentScreen()

	// Set timer to dim screen after 3 seconds
	app.ScreenPeekTimer = time.AfterFunc(3*time.Second, func() {
		app.dimScreen()
		app.ScreenPeekActive = false
	})
}

// dimScreen reduces brightness to minimum (for locked state)
func (app *MiyooPod) dimScreen() {
	brightnessPath := "/sys/class/pwm/pwmchip0/pwm0/duty_cycle"
	dimBrightness := "0"

	if err := os.WriteFile(brightnessPath, []byte(dimBrightness), 0644); err != nil {
		logMsg(fmt.Sprintf("WARNING: Could not dim screen: %v", err))
	} else {
		logMsg("Screen dimmed (locked)")
	}
}
