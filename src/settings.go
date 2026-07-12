package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/google/uuid"
)

const SETTINGS_PATH = "/mnt/SDCARD/Media/Music/.miyoopod_settings.json"

type Settings struct {
	InstallationID        string `json:"installation_id,omitempty"`
	Theme                 string `json:"theme,omitempty"`
	LocalLogsEnabled      bool   `json:"local_logs_enabled,omitempty"`
	SentryEnabled         bool   `json:"sentry_enabled,omitempty"`
	AutoLockMinutes       *int   `json:"auto_lock_minutes,omitempty"`
	UpdateNotifications   *bool  `json:"update_notifications,omitempty"`
	Volume                *int   `json:"volume,omitempty"`
	Brightness            *int   `json:"brightness,omitempty"`
	FontSize              *int   `json:"font_size,omitempty"`
	KeepPerformanceOnLock *bool  `json:"keep_performance_on_lock,omitempty"`
}

// loadSettings loads theme and lock key preferences from a lightweight JSON file
func (app *MiyooPod) loadSettings() error {
	data, err := os.ReadFile(SETTINGS_PATH)
	if err != nil {
		return err
	}

	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return err
	}

	// Generate installation ID if it doesn't exist
	if settings.InstallationID == "" {
		settings.InstallationID = uuid.New().String()
		logMsg(fmt.Sprintf("INFO: Generated new installation ID: %s", settings.InstallationID))
		// Save immediately
		app.InstallationID = settings.InstallationID
		app.saveSettings()
	} else {
		app.InstallationID = settings.InstallationID
		logMsg(fmt.Sprintf("Loaded installation ID: %s", settings.InstallationID))
	}

	// Restore theme
	if settings.Theme != "" {
		for _, theme := range AllThemes() {
			if theme.Name == settings.Theme {
				app.setTheme(theme)
				logMsg(fmt.Sprintf("INFO: Restored theme: %s", settings.Theme))
				break
			}
		}
	}

	// Restore log writing preference
	app.LocalLogsEnabled = settings.LocalLogsEnabled
	if app.LocalLogsEnabled {
		logMsg("Local logs enabled")
	} else {
		logMsg("Local logs disabled")
	}

	// Restore Sentry preference
	app.SentryEnabled = settings.SentryEnabled
	if app.SentryEnabled {
		logMsg("Developer logs (Sentry) enabled")
	} else {
		logMsg("Developer logs (Sentry) disabled")
	}

	// Restore auto-lock minutes (default to 3 if not set)
	if settings.AutoLockMinutes != nil {
		app.AutoLockMinutes = *settings.AutoLockMinutes
		if *settings.AutoLockMinutes == 0 {
			logMsg("Auto-lock disabled")
		} else {
			logMsg(fmt.Sprintf("Auto-lock: %d minutes", *settings.AutoLockMinutes))
		}
	}
	// else keep default value (3 minutes)

	// Restore update notifications preference (default to true if not set)
	if settings.UpdateNotifications != nil {
		app.UpdateNotifications = *settings.UpdateNotifications
		if app.UpdateNotifications {
			logMsg("Update notifications enabled")
		} else {
			logMsg("Update notifications disabled")
		}
	}

	// Restore volume (default 50 if not set)
	if settings.Volume != nil {
		app.SystemVolume = *settings.Volume
		setMiAOVolume(app.SystemVolume)
		logMsg(fmt.Sprintf("INFO: Restored volume: %d%%", app.SystemVolume))
	}

	// Restore brightness (default to current if not set)
	if settings.Brightness != nil {
		app.SystemBrightness = *settings.Brightness
		setBrightness(app.SystemBrightness)
		logMsg(fmt.Sprintf("INFO: Restored brightness: %d%%", app.SystemBrightness))
	}

	// Restore font size (default Medium if not set)
	if settings.FontSize != nil && *settings.FontSize != app.FontSize {
		if *settings.FontSize >= FontSizeSmall && *settings.FontSize <= FontSizeLarge {
			app.FontSize = *settings.FontSize
			app.applyFontSize()
			logMsg(fmt.Sprintf("INFO: Restored font size: %s", fontSizeName(app.FontSize)))
		}
	}

	// Restore keep-performance-on-lock preference (default false = drop to powersave)
	if settings.KeepPerformanceOnLock != nil {
		app.KeepPerformanceOnLock = *settings.KeepPerformanceOnLock
		if app.KeepPerformanceOnLock {
			logMsg("Keep performance while locked: enabled")
		} else {
			logMsg("Keep performance while locked: disabled")
		}
	}

	return nil
}

// saveSettings saves current theme and lock key preferences
func (app *MiyooPod) saveSettings() error {
	settings := Settings{
		InstallationID:        app.InstallationID,
		Theme:                 app.CurrentTheme.Name,
		LocalLogsEnabled:      app.LocalLogsEnabled,
		SentryEnabled:         app.SentryEnabled,
		AutoLockMinutes:       &app.AutoLockMinutes,
		UpdateNotifications:   &app.UpdateNotifications,
		Volume:                &app.SystemVolume,
		Brightness:            &app.SystemBrightness,
		FontSize:              &app.FontSize,
		KeepPerformanceOnLock: &app.KeepPerformanceOnLock,
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(SETTINGS_PATH, data, 0644)
}
