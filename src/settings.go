package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/google/uuid"
)

const SETTINGS_PATH = "/mnt/SDCARD/Media/Music/.miyoopod_settings.json"

type Settings struct {
	InstallationID   string `json:"installation_id,omitempty"`
	Theme            string `json:"theme,omitempty"`
	LockKey          string `json:"lock_key,omitempty"`
	LocalLogsEnabled bool   `json:"local_logs_enabled,omitempty"`
	SentryEnabled    bool   `json:"sentry_enabled,omitempty"`
	AutoLockMinutes  *int   `json:"auto_lock_minutes,omitempty"`
	ScreenPeekEnabled *bool `json:"screen_peek_enabled,omitempty"`
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
		logMsg(fmt.Sprintf("Generated new installation ID: %s", settings.InstallationID))
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

	// Restore lock key
	if settings.LockKey != "" {
		switch settings.LockKey {
		case "Y":
			app.LockKey = Y
		case "X":
			app.LockKey = X
		case "SELECT":
			app.LockKey = SELECT
		}
		logMsg(fmt.Sprintf("INFO: Restored lock key: %s", settings.LockKey))
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

	// Restore screen peek preference (default to true if not set)
	if settings.ScreenPeekEnabled != nil {
		app.ScreenPeekEnabled = *settings.ScreenPeekEnabled
		if app.ScreenPeekEnabled {
			logMsg("Screen peek enabled")
		} else {
			logMsg("Screen peek disabled")
		}
	}

	return nil
}

// saveSettings saves current theme and lock key preferences
func (app *MiyooPod) saveSettings() error {
	settings := Settings{
		InstallationID:   app.InstallationID,
		Theme:            app.CurrentTheme.Name,
		LockKey:          app.getLockKeyName(),
		LocalLogsEnabled: app.LocalLogsEnabled,
		SentryEnabled:    app.SentryEnabled,
		AutoLockMinutes:   &app.AutoLockMinutes,
		ScreenPeekEnabled: &app.ScreenPeekEnabled,
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(SETTINGS_PATH, data, 0644)
}
