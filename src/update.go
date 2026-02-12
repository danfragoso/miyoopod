package main

import (
	"encoding/json"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/fogleman/gg"
)

const UPDATE_INFO_PATH = "/mnt/SDCARD/Media/Music/.miyoopod_update.json"
const UPDATE_STATUS_PATH = "/mnt/SDCARD/Media/Music/.miyoopod_update_status"

type UpdateRequest struct {
	Version  string `json:"version"`
	URL      string `json:"url"`
	Checksum string `json:"checksum"`
	Size     int64  `json:"size"`
}

type UpdateStatus struct {
	Success bool   `json:"success"`
	Version string `json:"version"`
	Error   string `json:"error,omitempty"`
}

// showUpdatePrompt sets the update prompt flag and triggers a full redraw.
// Key handling is done by the main loop via handleUpdatePromptKey().
func (app *MiyooPod) showUpdatePrompt() {
	if app.UpdateInfo == nil {
		return
	}

	app.ShowingUpdatePrompt = true
	app.drawCurrentScreen()
}

// drawUpdatePromptOverlay renders the update dialog overlay.
// Uses the same design language as the rest of the app: HeaderBG header bar,
// ProgBG button badges, and flat layout matching drawHeader/drawStatusBar/drawButtonLegend.
func (app *MiyooPod) drawUpdatePromptOverlay() {
	info := app.UpdateInfo
	if info == nil {
		return
	}

	dc := app.DC

	// Full-screen takeover with app background (same as menu/now playing screens)
	dc.SetHexColor(app.CurrentTheme.BG)
	dc.Clear()

	// Header bar (identical style to drawHeader)
	dc.SetHexColor(app.CurrentTheme.HeaderBG)
	dc.DrawRectangle(0, 0, SCREEN_WIDTH, HEADER_HEIGHT)
	dc.Fill()

	dc.SetFontFace(app.FontHeader)
	dc.SetHexColor(app.CurrentTheme.HeaderTxt)
	dc.DrawStringAnchored("Update Available", SCREEN_WIDTH/2, HEADER_HEIGHT/2, 0.5, 0.5)

	// Content area
	contentY := float64(HEADER_HEIGHT) + 24

	// Version transition row - styled like a selected menu item
	dc.SetHexColor(app.CurrentTheme.SelBG)
	dc.DrawRectangle(0, contentY, SCREEN_WIDTH, MENU_ITEM_HEIGHT)
	dc.Fill()

	versionStr := fmt.Sprintf("v%s  →  v%s", APP_VERSION, info.Version)
	dc.SetFontFace(app.FontMenu)
	dc.SetHexColor(app.CurrentTheme.SelTxt)
	dc.DrawStringAnchored(versionStr, SCREEN_WIDTH/2, contentY+MENU_ITEM_HEIGHT/2, 0.5, 0.5)

	contentY += MENU_ITEM_HEIGHT + 20

	// Changelog (if present)
	if info.Changelog != "" {
		dc.SetFontFace(app.FontMenu)
		dc.SetHexColor(app.CurrentTheme.ItemTxt)
		dc.DrawStringWrapped(info.Changelog, SCREEN_WIDTH/2, contentY, 0.5, 0, SCREEN_WIDTH-MENU_LEFT_PAD*4, 1.5, gg.AlignCenter)
		contentY += 60
	}

	// Size info
	if info.Size > 0 {
		dc.SetFontFace(app.FontSmall)
		dc.SetHexColor(app.CurrentTheme.Dim)
		sizeStr := fmt.Sprintf("Download size: %.1f MB", float64(info.Size)/(1024*1024))
		dc.DrawStringAnchored(sizeStr, SCREEN_WIDTH/2, contentY, 0.5, 0.5)
	}

	// Status bar at bottom (identical style to drawStatusBar)
	barY := float64(SCREEN_HEIGHT - STATUS_BAR_HEIGHT)

	dc.SetHexColor(app.CurrentTheme.HeaderBG)
	dc.DrawRectangle(0, barY, SCREEN_WIDTH, STATUS_BAR_HEIGHT)
	dc.Fill()

	// Separator line at top of bar
	dc.SetHexColor(app.CurrentTheme.ProgBG)
	dc.SetLineWidth(1)
	dc.DrawLine(0, barY, SCREEN_WIDTH, barY)
	dc.Stroke()

	// Button legends (same style as drawButtonLegend in status bar)
	centerY := barY + float64(STATUS_BAR_HEIGHT)/2
	dc.SetFontFace(app.FontSmall)
	app.drawButtonLegend(12, centerY, "A", "Update Now")
	app.drawButtonLegend(180, centerY, "B", "Later")

	app.triggerRefresh()
}

// handleUpdatePromptKey handles key presses when the update prompt is showing.
// Returns true if the key was consumed by the prompt.
func (app *MiyooPod) handleUpdatePromptKey(key Key) bool {
	if !app.ShowingUpdatePrompt {
		return false
	}

	switch key {
	case A:
		app.ShowingUpdatePrompt = false
		app.launchUpdater()
		return true
	case B, MENU:
		app.ShowingUpdatePrompt = false
		app.drawCurrentScreen()
		return true
	}

	// Consume all other keys while prompt is showing
	return true
}

// launchUpdater stops playback, writes update info, and execs the updater binary
func (app *MiyooPod) launchUpdater() {
	if app.UpdateInfo == nil {
		return
	}

	// Stop playback
	if app.Playing != nil && app.Playing.State == StatePlaying {
		audioStop()
		app.Playing.State = StateStopped
	}

	// Write update request file
	req := UpdateRequest{
		Version:  app.UpdateInfo.Version,
		URL:      app.UpdateInfo.URL,
		Checksum: app.UpdateInfo.Checksum,
		Size:     app.UpdateInfo.Size,
	}

	data, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		logMsg(fmt.Sprintf("ERROR: Failed to marshal update request: %v", err))
		app.showError("Failed to prepare update")
		return
	}

	if err := os.WriteFile(UPDATE_INFO_PATH, data, 0644); err != nil {
		logMsg(fmt.Sprintf("ERROR: Failed to write update request: %v", err))
		app.showError("Failed to prepare update")
		return
	}

	// Show launching message
	dc := app.DC
	dc.SetHexColor(app.CurrentTheme.BG)
	dc.Clear()
	dc.SetFontFace(app.FontTitle)
	dc.SetHexColor(app.CurrentTheme.HeaderTxt)
	dc.DrawStringAnchored("Launching updater...", SCREEN_WIDTH/2, SCREEN_HEIGHT/2, 0.5, 0.5)
	app.triggerRefresh()
	time.Sleep(500 * time.Millisecond)

	// Track update event
	TrackAppLifecycle("app_closed", map[string]interface{}{
		"reason":         "ota_update",
		"update_version": app.UpdateInfo.Version,
	})

	// Cleanup
	close(app.RefreshChan)
	sdlCleanup()

	// Exec the updater binary (replaces this process)
	updaterPath := "./updater"
	err = syscall.Exec(updaterPath, []string{updaterPath}, os.Environ())
	if err != nil {
		logMsg(fmt.Sprintf("ERROR: Failed to exec updater: %v", err))
		// If exec fails, clean up the update request file
		os.Remove(UPDATE_INFO_PATH)
	}
}

// manualCheckForUpdates triggers a version check from settings and shows result
func (app *MiyooPod) manualCheckForUpdates() {
	dc := app.DC

	// Show checking screen
	dc.SetHexColor(app.CurrentTheme.BG)
	dc.Clear()
	dc.SetFontFace(app.FontMenu)
	dc.SetHexColor(app.CurrentTheme.HeaderTxt)
	dc.DrawStringAnchored("Checking for updates...", SCREEN_WIDTH/2, SCREEN_HEIGHT/2, 0.5, 0.5)
	app.triggerRefresh()

	// Run version check
	status := app.checkVersion()

	if app.UpdateAvailable {
		app.showUpdatePrompt()
		return
	}

	// Show result
	dc.SetHexColor(app.CurrentTheme.BG)
	dc.Clear()
	dc.SetFontFace(app.FontMenu)

	if status == "Up to date" {
		dc.SetHexColor(app.CurrentTheme.Accent)
		dc.DrawStringAnchored(fmt.Sprintf("You're up to date (v%s)", APP_VERSION), SCREEN_WIDTH/2, SCREEN_HEIGHT/2, 0.5, 0.5)
	} else {
		dc.SetHexColor(app.CurrentTheme.Dim)
		dc.DrawStringAnchored("Failed to check for updates", SCREEN_WIDTH/2, SCREEN_HEIGHT/2, 0.5, 0.5)
	}

	app.triggerRefresh()
	time.Sleep(1500 * time.Millisecond)

	// Return to settings menu
	app.drawCurrentScreen()
}

// handleUpdateStatus checks for a status file from a previous OTA update
func (app *MiyooPod) handleUpdateStatus() {
	data, err := os.ReadFile(UPDATE_STATUS_PATH)
	if err != nil {
		return // No status file, nothing to do
	}

	// Remove the status file immediately
	os.Remove(UPDATE_STATUS_PATH)

	var status UpdateStatus
	if err := json.Unmarshal(data, &status); err != nil {
		logMsg(fmt.Sprintf("WARNING: Failed to parse update status: %v", err))
		return
	}

	dc := app.DC

	if status.Success {
		// Show success message using app's standard layout
		dc.SetHexColor(app.CurrentTheme.BG)
		dc.Clear()

		// Header bar
		dc.SetHexColor(app.CurrentTheme.HeaderBG)
		dc.DrawRectangle(0, 0, SCREEN_WIDTH, HEADER_HEIGHT)
		dc.Fill()

		dc.SetFontFace(app.FontHeader)
		dc.SetHexColor(app.CurrentTheme.HeaderTxt)
		dc.DrawStringAnchored("Update Complete", SCREEN_WIDTH/2, HEADER_HEIGHT/2, 0.5, 0.5)

		// Success message centered in content area
		dc.SetFontFace(app.FontTitle)
		dc.SetHexColor(app.CurrentTheme.Accent)
		dc.DrawStringAnchored(fmt.Sprintf("Updated to v%s!", status.Version), SCREEN_WIDTH/2, SCREEN_HEIGHT/2, 0.5, 0.5)

		app.triggerRefresh()
		time.Sleep(2000 * time.Millisecond)
		app.drawCurrentScreen()
	} else {
		// Show failure message
		logMsg(fmt.Sprintf("ERROR: OTA update failed: %s", status.Error))
		app.showError(fmt.Sprintf("Update failed: %s", status.Error))
	}
}

// clearAppData deletes library cache, settings, and artwork, preserving the installation UUID
func (app *MiyooPod) clearAppData() {
	dc := app.DC

	// Full-screen layout matching app design language
	dc.SetHexColor(app.CurrentTheme.BG)
	dc.Clear()

	// Header bar
	dc.SetHexColor(app.CurrentTheme.HeaderBG)
	dc.DrawRectangle(0, 0, SCREEN_WIDTH, HEADER_HEIGHT)
	dc.Fill()

	dc.SetFontFace(app.FontHeader)
	dc.SetHexColor(app.CurrentTheme.HeaderTxt)
	dc.DrawStringAnchored("Clear App Data", SCREEN_WIDTH/2, HEADER_HEIGHT/2, 0.5, 0.5)

	// Warning message centered in content area
	dc.SetFontFace(app.FontMenu)
	dc.SetHexColor(app.CurrentTheme.ItemTxt)
	dc.DrawStringAnchored("Clear all app data?", SCREEN_WIDTH/2, SCREEN_HEIGHT/2-20, 0.5, 0.5)

	dc.SetFontFace(app.FontSmall)
	dc.SetHexColor(app.CurrentTheme.Dim)
	dc.DrawStringAnchored("This will reset settings and rescan your library", SCREEN_WIDTH/2, SCREEN_HEIGHT/2+16, 0.5, 0.5)

	// Status bar at bottom
	barY := float64(SCREEN_HEIGHT - STATUS_BAR_HEIGHT)

	dc.SetHexColor(app.CurrentTheme.HeaderBG)
	dc.DrawRectangle(0, barY, SCREEN_WIDTH, STATUS_BAR_HEIGHT)
	dc.Fill()

	dc.SetHexColor(app.CurrentTheme.ProgBG)
	dc.SetLineWidth(1)
	dc.DrawLine(0, barY, SCREEN_WIDTH, barY)
	dc.Stroke()

	centerY := barY + float64(STATUS_BAR_HEIGHT)/2
	dc.SetFontFace(app.FontSmall)
	app.drawButtonLegend(12, centerY, "A", "Confirm")
	app.drawButtonLegend(150, centerY, "B", "Cancel")

	app.triggerRefresh()

	// Wait for user input
	for app.Running {
		key := Key(C_GetKeyPress())
		if key == NONE {
			time.Sleep(33 * time.Millisecond)
			continue
		}

		switch key {
		case A:
			app.performClearAppData()
			return
		case B, MENU:
			app.drawCurrentScreen()
			return
		}
	}
}

// performClearAppData does the actual deletion and restarts the app
func (app *MiyooPod) performClearAppData() {
	// Preserve installation UUID
	installID := app.InstallationID

	// Show clearing message
	dc := app.DC
	dc.SetHexColor(app.CurrentTheme.BG)
	dc.Clear()
	dc.SetFontFace(app.FontMenu)
	dc.SetHexColor(app.CurrentTheme.HeaderTxt)
	dc.DrawStringAnchored("Clearing app data...", SCREEN_WIDTH/2, SCREEN_HEIGHT/2, 0.5, 0.5)
	app.triggerRefresh()

	// Delete library cache
	if err := os.Remove(LIBRARY_JSON_PATH); err != nil && !os.IsNotExist(err) {
		logMsg(fmt.Sprintf("WARNING: Failed to remove library cache: %v", err))
	}

	// Delete settings
	if err := os.Remove(SETTINGS_PATH); err != nil && !os.IsNotExist(err) {
		logMsg(fmt.Sprintf("WARNING: Failed to remove settings: %v", err))
	}

	// Delete artwork directory
	if err := os.RemoveAll(ARTWORK_DIR); err != nil && !os.IsNotExist(err) {
		logMsg(fmt.Sprintf("WARNING: Failed to remove artwork: %v", err))
	}

	// Write back a minimal settings file with just the UUID
	minimalSettings := Settings{
		InstallationID: installID,
	}
	data, err := json.MarshalIndent(minimalSettings, "", "  ")
	if err == nil {
		os.WriteFile(SETTINGS_PATH, data, 0644)
	}

	logMsg("INFO: App data cleared, restarting...")

	// Stop playback
	if app.Playing != nil && app.Playing.State == StatePlaying {
		audioStop()
	}

	// Cleanup
	close(app.RefreshChan)
	sdlCleanup()

	// Restart the app via launch.sh
	launchPath := "./launch.sh"
	syscall.Exec("/bin/sh", []string{"/bin/sh", launchPath}, os.Environ())
}

// toggleUpdateNotifications toggles the update notifications setting
func (app *MiyooPod) toggleUpdateNotifications() {
	app.UpdateNotifications = !app.UpdateNotifications

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

	if err := app.saveSettings(); err != nil {
		logMsg(fmt.Sprintf("ERROR: Failed to save update notifications preference: %v", err))
	}
}
