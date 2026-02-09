package main

/*
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"os"
	"strings"
	"time"
)

const LOG_PATH = "./miyoopod.log"

var logFile *os.File
var globalApp *MiyooPod // Global reference for checking LocalLogsEnabled

func init() {
	var err error
	logFile, err = os.OpenFile(LOG_PATH, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
}

// logMsg writes to local log file and automatically captures errors/warnings to PostHog.
// Local logs and developer logs are independent:
//   - LocalLogsEnabled controls writing to miyoopod.log file
//   - SentryEnabled controls sending to PostHog for observability
//
// Examples:
//
//	logMsg("ERROR: Failed to load track") -> sent to PostHog as error (if enabled), written to file (if enabled)
//	logMsg("WARNING: Low battery") -> sent to PostHog as warning (if enabled), written to file (if enabled)
//	logMsg("INFO: Playback started") -> sent to PostHog as info (if enabled), written to file (if enabled)
func logMsg(message string) {
	// Write to local log file if enabled
	if globalApp != nil && globalApp.LocalLogsEnabled {
		logFile.WriteString(time.Now().Format("2006-01-02 15:04:05.999") + " - " + message + "\n")
	}

	// Capture to PostHog independently (if developer logs enabled)
	attrs := extractLogAttributes(message)
	if strings.Contains(strings.ToUpper(message), "ERROR") || strings.Contains(strings.ToUpper(message), "FATAL") {
		CaptureError(message, attrs)
	} else if strings.Contains(strings.ToUpper(message), "WARNING") {
		CaptureWarning(message, attrs)
	} else if strings.Contains(strings.ToUpper(message), "INFO:") {
		CaptureInfo(message, attrs)
	}
}

// extractLogAttributes parses log messages to extract structured data
func extractLogAttributes(message string) map[string]interface{} {
	attrs := make(map[string]interface{})

	// Extract action from message patterns
	if strings.Contains(message, "Playing:") {
		attrs["action"] = "play"
		// Extract artist and track: "Playing: Artist - Track"
		if parts := strings.Split(message, "Playing: "); len(parts) > 1 {
			if trackInfo := strings.Split(parts[1], " - "); len(trackInfo) >= 2 {
				attrs["artist"] = strings.TrimSpace(trackInfo[0])
				attrs["track"] = strings.TrimSpace(strings.Join(trackInfo[1:], " - "))
			}
		}
	} else if strings.Contains(message, "Screen locked") {
		attrs["action"] = "screen_lock"
	} else if strings.Contains(message, "Screen unlocked") {
		attrs["action"] = "screen_unlock"
	} else if strings.Contains(message, "Library loaded") {
		attrs["action"] = "library_load"
		// Extract stats: "474 tracks, 37 albums, 33 artists"
		if parts := strings.Split(message, ": "); len(parts) > 1 {
			stats := parts[1]
			if strings.Contains(stats, "tracks") {
				var tracks, albums, artists int
				fmt.Sscanf(stats, "%d tracks, %d albums, %d artists", &tracks, &albums, &artists)
				attrs["track_count"] = tracks
				attrs["album_count"] = albums
				attrs["artist_count"] = artists
			}
		}
	}

	// Add current app context
	if globalApp != nil {
		if globalApp.CurrentTheme.Name != "" {
			attrs["theme"] = globalApp.CurrentTheme.Name
		}
		if globalApp.Queue != nil {
			if globalApp.Queue.Shuffle {
				attrs["shuffle"] = "enabled"
			} else {
				attrs["shuffle"] = "disabled"
			}
		}
	}

	return attrs
}

//export GoLogMsg
func GoLogMsg(cMsg *C.char) {
	msg := C.GoString(cMsg)
	logMsg(msg)
}

//export DetectDevice
func DetectDevice(width, height C.int) {
	if globalApp == nil {
		return
	}

	w := int(width)
	h := int(height)

	globalApp.DisplayWidth = w
	globalApp.DisplayHeight = h

	// Determine device model based on resolution
	if w == 640 && h == 480 {
		globalApp.DeviceModel = "miyoo-mini-plus"
	} else if w == 750 && h == 560 {
		// Both v4 and flip have the same resolution
		// We'll label them as v4 by default, could be either
		globalApp.DeviceModel = "miyoo-mini-v4"
	} else {
		globalApp.DeviceModel = fmt.Sprintf("unknown-%dx%d", w, h)
	}

	logMsg(fmt.Sprintf("INFO: Device detected: %s (%dx%d)", globalApp.DeviceModel, w, h))
}
