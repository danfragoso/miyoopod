package main

import (
	"encoding/json"
	"fmt"
	"os"
)

const SETTINGS_PATH = "/mnt/SDCARD/Media/Music/.miyoopod_settings.json"

type Settings struct {
	Theme            string `json:"theme,omitempty"`
	LockKey          string `json:"lock_key,omitempty"`
	WriteLogsEnabled bool   `json:"write_logs_enabled,omitempty"`
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

	// Restore theme
	if settings.Theme != "" {
		for _, theme := range AllThemes() {
			if theme.Name == settings.Theme {
				app.setTheme(theme)
				logMsg(fmt.Sprintf("Restored theme: %s", settings.Theme))
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
		logMsg(fmt.Sprintf("Restored lock key: %s", settings.LockKey))
	}

	// Restore log writing preference
	app.WriteLogsEnabled = settings.WriteLogsEnabled
	if app.WriteLogsEnabled {
		logMsg("Log writing enabled")
	} else {
		logMsg("Log writing disabled")
	}

	return nil
}

// saveSettings saves current theme and lock key preferences
func (app *MiyooPod) saveSettings() error {
	settings := Settings{
		Theme:            app.CurrentTheme.Name,
		LockKey:          app.getLockKeyName(),
		WriteLogsEnabled: app.WriteLogsEnabled,
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(SETTINGS_PATH, data, 0644)
}
