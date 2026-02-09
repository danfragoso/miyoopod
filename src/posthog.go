package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Lightweight PostHog client using OTLP for logs and error tracking API.
//
// Behavior:
//   - INFO logs → PostHog OTLP (queryable, structured attributes for analytics)
//   - ERROR/WARNING → PostHog Error Tracking ($exception events)
//   - Product analytics → Event tracking API
//
// Automatically tracked events:
//   - app_opened: App startup with library stats (tracks, artists, albums)
//   - app_closed: App shutdown
//   - $pageview: Screen and menu navigation for funnel analysis
//     * Screens: home, artists, albums, playlists, themes, settings, now_playing, queue
//     * Properties: menu_depth, action (e.g., "back")
//   - song_played: Music playback with artist, title, album, duration, shuffle, repeat
//   - Action events (sent as individual event names):
//     * shuffle_enabled, shuffle_disabled
//     * repeat_mode_changed (with repeat_mode property)
//     * theme_changed (with theme_name property)
//     * screen_locked, screen_unlocked
//
// Automatically captures attributes with every log:
//   - action, artist, track, theme, shuffle (from log content)
//   - App version, library stats, playback state (app context)
//   - Installation ID as user identifier (distinct_id)
//   - Device info (miyoo-mini-plus)
//
// Setup:
//   1. Add POSTHOG_TOKEN to .env file
//   2. Run 'make go' to build (token injected at build time)
//   3. Use logMsg() with "ERROR"/"WARNING"/"INFO:" for automatic capture
//   4. Page views, user actions, and lifecycle events are tracked automatically

const POSTHOG_TOKEN = "" // Injected at build time

// OTLP Log structures
type OTLPKeyValue struct {
	Key   string                 `json:"key"`
	Value map[string]interface{} `json:"value"`
}

type OTLPLogRecord struct {
	SeverityNumber int               `json:"severityNumber"`
	SeverityText   string            `json:"severityText"`
	Body           map[string]string `json:"body"`
	Attributes     []OTLPKeyValue    `json:"attributes,omitempty"`
}

type OTLPScopeLogs struct {
	Scope      map[string]string `json:"scope"`
	LogRecords []OTLPLogRecord   `json:"logRecords"`
}

type OTLPResourceLogs struct {
	Resource  map[string][]OTLPKeyValue `json:"resource"`
	ScopeLogs []OTLPScopeLogs           `json:"scopeLogs"`
}

type OTLPLogsPayload struct {
	ResourceLogs []OTLPResourceLogs `json:"resourceLogs"`
}

type PostHogClient struct {
	Token   string
	Enabled bool
}

var posthogClient *PostHogClient

// initSentry initializes the PostHog client (keeping function name for compatibility)
func (app *MiyooPod) initSentry() {
	if POSTHOG_TOKEN == "" {
		return // Silently skip if no token
	}

	posthogClient = &PostHogClient{
		Token:   POSTHOG_TOKEN,
		Enabled: app.SentryEnabled, // Reusing existing setting
	}

	if posthogClient.Enabled {
		logMsg("PostHog client initialized and enabled")
	} else {
		logMsg("PostHog client initialized but disabled")
	}
}

// captureEvent sends logs/events to PostHog (fire and forget)
// INFO → OTLP logs, ERROR/WARNING → Error tracking API
func captureEvent(level, message string, extra map[string]interface{}) {
	if posthogClient == nil || !posthogClient.Enabled || POSTHOG_TOKEN == "" || globalApp == nil {
		return
	}

	// Fire and forget in goroutine
	go func() {
		// Safely recover from any panics in goroutine
		defer func() {
			if r := recover(); r != nil {
				logFile.WriteString(fmt.Sprintf("[POSTHOG] Panic: %v\n", r))
			}
		}()

		if level == "info" {
			// Send INFO logs to OTLP for analytics
			if err := sendOTLPLog(level, message, extra); err != nil {
				if globalApp.LocalLogsEnabled {
					logFile.WriteString(fmt.Sprintf("[POSTHOG] OTLP failed: %v\n", err))
				}
			}
		} else if level == "error" || level == "warning" {
			// Send ERROR/WARNING to error tracking API
			if err := sendErrorToPostHog(level, message, extra); err != nil {
				if globalApp.LocalLogsEnabled {
					logFile.WriteString(fmt.Sprintf("[POSTHOG] Error tracking failed: %v\n", err))
				}
			}
		}
	}()
}

// sendOTLPLog sends a log record via OTLP to PostHog
func sendOTLPLog(level, message string, extra map[string]interface{}) error {
	// Map severity levels
	severityMap := map[string]int{
		"info":    9,  // INFO
		"warning": 13, // WARN
		"error":   17, // ERROR
	}
	severityNumber := severityMap[level]
	if severityNumber == 0 {
		severityNumber = 9
	}

	// Build attributes from extra
	attributes := []OTLPKeyValue{}
	if extra != nil {
		for k, v := range extra {
			var attrValue map[string]interface{}
			switch val := v.(type) {
			case string:
				attrValue = map[string]interface{}{"stringValue": val}
			case int:
				attrValue = map[string]interface{}{"intValue": val}
			case bool:
				attrValue = map[string]interface{}{"boolValue": val}
			default:
				attrValue = map[string]interface{}{"stringValue": fmt.Sprintf("%v", val)}
			}
			attributes = append(attributes, OTLPKeyValue{Key: k, Value: attrValue})
		}
	}

	// Add user ID attribute
	attributes = append(attributes, OTLPKeyValue{
		Key:   "user.id",
		Value: map[string]interface{}{"stringValue": globalApp.InstallationID},
	})

	// Build resource attributes
	resourceAttrs := []OTLPKeyValue{
		{Key: "service.name", Value: map[string]interface{}{"stringValue": "miyoopod"}},
		{Key: "service.version", Value: map[string]interface{}{"stringValue": APP_VERSION}},
		{Key: "device.model", Value: map[string]interface{}{"stringValue": "miyoo-mini-plus"}},
	}

	// Build log record (omit timestamp - let PostHog use ingestion time due to device's 1970 clock)
	logRecord := OTLPLogRecord{
		SeverityNumber: severityNumber,
		SeverityText:   strings.ToUpper(level),
		Body:           map[string]string{"stringValue": message},
		Attributes:     attributes,
	}

	payload := OTLPLogsPayload{
		ResourceLogs: []OTLPResourceLogs{
			{
				Resource: map[string][]OTLPKeyValue{"attributes": resourceAttrs},
				ScopeLogs: []OTLPScopeLogs{
					{
						Scope:      map[string]string{"name": "miyoopod"},
						LogRecords: []OTLPLogRecord{logRecord},
					},
				},
			},
		},
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Send to PostHog OTLP endpoint
	url := "https://us.i.posthog.com/i/v1/logs"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadJSON))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", POSTHOG_TOKEN))

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("PostHog OTLP returned status %d", resp.StatusCode)
	}

	return nil
}

// sendErrorToPostHog sends an error/warning to PostHog error tracking API
func sendErrorToPostHog(level, message string, extra map[string]interface{}) error {
	// Build exception list
	exceptionList := []map[string]interface{}{
		{
			"type":  level,
			"value": message,
			"mechanism": map[string]interface{}{
				"handled":   true,
				"synthetic": false,
			},
			"stacktrace": map[string]interface{}{
				"type": "raw",
				"frames": []map[string]interface{}{
					{
						"platform": "custom",
						"lang":     "go",
						"function": "logMsg",
						"module":   "miyoopod",
					},
				},
			},
		},
	}

	// Build event properties
	properties := map[string]interface{}{
		"distinct_id":        globalApp.InstallationID,
		"$exception_list":    exceptionList,
		"$exception_message": message,
		"$exception_level":   level,
		"version":            APP_VERSION,
		"device":             "miyoo-mini-plus",
	}

	// Add extra attributes
	if extra != nil {
		for k, v := range extra {
			properties[k] = v
		}
	}

	// Build PostHog event
	event := map[string]interface{}{
		"api_key":    POSTHOG_TOKEN,
		"event":      "$exception",
		"properties": properties,
	}

	eventJSON, err := json.Marshal(event)
	if err != nil {
		return err
	}

	// Send to PostHog events API
	url := "https://us.i.posthog.com/i/v0/e/"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(eventJSON))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("PostHog error tracking returned status %d", resp.StatusCode)
	}

	return nil
}

// Helper functions for logging different levels
func CaptureError(message string, extra map[string]interface{}) {
	captureEvent("error", message, extra)
}

func CaptureWarning(message string, extra map[string]interface{}) {
	captureEvent("warning", message, extra)
}

func CaptureInfo(message string, extra map[string]interface{}) {
	captureEvent("info", message, extra)
}

// --- Product Analytics Event Tracking ---

// trackEvent sends a custom event to PostHog for product analytics
func trackEvent(eventName string, properties map[string]interface{}) error {
	if posthogClient == nil || !posthogClient.Enabled || POSTHOG_TOKEN == "" || globalApp == nil {
		return nil
	}

	// Ensure properties map exists
	if properties == nil {
		properties = make(map[string]interface{})
	}

	// Add standard properties
	properties["distinct_id"] = globalApp.InstallationID
	properties["version"] = APP_VERSION
	properties["device"] = "miyoo-mini-plus"

	// Build PostHog event
	event := map[string]interface{}{
		"api_key":    POSTHOG_TOKEN,
		"event":      eventName,
		"properties": properties,
	}

	eventJSON, err := json.Marshal(event)
	if err != nil {
		return err
	}

	// Send to PostHog events API
	url := "https://us.i.posthog.com/i/v0/e/"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(eventJSON))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("PostHog event tracking returned status %d", resp.StatusCode)
	}

	return nil
}

// TrackPageView tracks screen navigation for funnel analysis
// Screens: menu, now_playing, queue
// For menu screen, pass the menu title (e.g., "Artists", "Albums", "Playlists")
func TrackPageView(screenName string, properties map[string]interface{}) {
	if posthogClient == nil || !posthogClient.Enabled {
		return
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logFile.WriteString(fmt.Sprintf("[POSTHOG] TrackPageView panic: %v\n", r))
			}
		}()

		if properties == nil {
			properties = make(map[string]interface{})
		}

		properties["$current_url"] = "/" + screenName
		properties["screen_name"] = screenName

		if err := trackEvent("$pageview", properties); err != nil {
			if globalApp.LocalLogsEnabled {
				logFile.WriteString(fmt.Sprintf("[POSTHOG] TrackPageView failed: %v\n", err))
			}
		}
	}()
}

// TrackSongPlayed tracks music playback with full metadata
func TrackSongPlayed(track *Track) {
	if posthogClient == nil || !posthogClient.Enabled || track == nil {
		return
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logFile.WriteString(fmt.Sprintf("[POSTHOG] TrackSongPlayed panic: %v\n", r))
			}
		}()

		properties := map[string]interface{}{
			"artist":    track.Artist,
			"title":     track.Title,
			"album":     track.Album,
			"duration":  track.Duration,
			"file_path": track.Path,
		}

		// Add playback context if available
		if globalApp != nil {
			properties["shuffle_enabled"] = globalApp.Queue.Shuffle
			properties["repeat_mode"] = globalApp.Queue.Repeat.String()
			properties["queue_size"] = len(globalApp.Queue.Tracks)
		}

		if err := trackEvent("song_played", properties); err != nil {
			if globalApp.LocalLogsEnabled {
				logFile.WriteString(fmt.Sprintf("[POSTHOG] TrackSongPlayed failed: %v\n", err))
			}
		}
	}()
}

// TrackAction tracks user actions like theme changes, shuffle toggles, etc.
// The action becomes the event name (e.g., "theme_changed", "screen_locked")
func TrackAction(action string, properties map[string]interface{}) {
	if posthogClient == nil || !posthogClient.Enabled {
		return
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logFile.WriteString(fmt.Sprintf("[POSTHOG] TrackAction panic: %v\n", r))
			}
		}()

		if properties == nil {
			properties = make(map[string]interface{})
		}

		if err := trackEvent(action, properties); err != nil {
			if globalApp.LocalLogsEnabled {
				logFile.WriteString(fmt.Sprintf("[POSTHOG] TrackAction failed: %v\n", err))
			}
		}
	}()
}

// TrackAppLifecycle tracks app opened and closed events
func TrackAppLifecycle(event string, properties map[string]interface{}) {
	if posthogClient == nil || !posthogClient.Enabled {
		return
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logFile.WriteString(fmt.Sprintf("[POSTHOG] TrackAppLifecycle panic: %v\n", r))
			}
		}()

		if properties == nil {
			properties = make(map[string]interface{})
		}

		// Add library stats if available
		if globalApp != nil && globalApp.Library != nil {
			properties["library_tracks"] = len(globalApp.Library.Tracks)
			properties["library_artists"] = len(globalApp.Library.Artists)
			properties["library_albums"] = len(globalApp.Library.Albums)
		}

		if err := trackEvent(event, properties); err != nil {
			if globalApp.LocalLogsEnabled {
				logFile.WriteString(fmt.Sprintf("[POSTHOG] TrackAppLifecycle failed: %v\n", err))
			}
		}
	}()
}
