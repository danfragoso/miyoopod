package main

import (
	"fmt"
	"os"
	"strings"
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
				logMsg(fmt.Sprintf("INFO: Auto-lock triggered after %v of inactivity", inactiveDuration))
				app.toggleLock()
			}
		}
	}
}

// monitorPowerButtonHold is now event-driven via a timer set in handlePowerButtonPress.
// This function is kept as the timer callback for the 5-second force shutdown.
func (app *MiyooPod) monitorPowerButtonHold() {
	// If button was released before the timer fired, do nothing
	if !app.PowerButtonPressed {
		return
	}

	holdDuration := time.Since(app.PowerButtonPressTime)
	logMsg("INFO: Power button held for 5+ seconds - forcing shutdown")

	// Restore brightness and CPU governor before exiting
	restoreBrightness()
	app.restoreCPUGovernor()

	TrackAction("force_shutdown", map[string]interface{}{
		"hold_duration": holdDuration.Seconds(),
	})

	// Set flag to exit cleanly
	app.Running = false
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

// --- CPU governor management ---

// cpuGovernorPaths returns the sysfs paths for all CPU cores' scaling_governor
func cpuGovernorPaths() []string {
	var paths []string
	for i := 0; i < 2; i++ { // Dual-core Cortex-A7
		paths = append(paths, fmt.Sprintf("/sys/devices/system/cpu/cpu%d/cpufreq/scaling_governor", i))
	}
	return paths
}

// readCPUGovernor reads the current governor from cpu0
func readCPUGovernor() string {
	data, err := os.ReadFile("/sys/devices/system/cpu/cpu0/cpufreq/scaling_governor")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// writeCPUGovernor writes the governor to all CPU cores
func writeCPUGovernor(governor string) error {
	for _, path := range cpuGovernorPaths() {
		if err := os.WriteFile(path, []byte(governor), 0644); err != nil {
			return err
		}
	}
	return nil
}

// initCPUGovernor saves the original CPU governor for later restoration on exit
func (app *MiyooPod) initCPUGovernor() {
	app.OriginalCPUGovernor = readCPUGovernor()
	if app.OriginalCPUGovernor != "" {
		logMsg(fmt.Sprintf("INFO: Original CPU governor: %s", app.OriginalCPUGovernor))
	}
}

// setCPUGovernorLocked switches to powersave (screen off, audio-only — max battery savings)
func (app *MiyooPod) setCPUGovernorLocked() {
	if err := writeCPUGovernor("powersave"); err != nil {
		logMsg(fmt.Sprintf("WARNING: Could not set CPU governor to powersave: %v", err))
	}
}

// setCPUGovernorUnlocked switches to performance (full speed for responsive UI)
func (app *MiyooPod) setCPUGovernorUnlocked() {
	if err := writeCPUGovernor("performance"); err != nil {
		logMsg(fmt.Sprintf("WARNING: Could not set CPU governor to performance: %v", err))
	}
}

// restoreCPUGovernor restores the original CPU governor on app exit
func (app *MiyooPod) restoreCPUGovernor() {
	if app.OriginalCPUGovernor == "" {
		return
	}
	if err := writeCPUGovernor(app.OriginalCPUGovernor); err != nil {
		logMsg(fmt.Sprintf("WARNING: Could not restore CPU governor to %s: %v", app.OriginalCPUGovernor, err))
	} else {
		logMsg(fmt.Sprintf("INFO: CPU governor restored to %s", app.OriginalCPUGovernor))
	}
}
