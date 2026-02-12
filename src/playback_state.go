package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

const PLAYBACK_STATE_PATH = "/mnt/SDCARD/Media/Music/.miyoopod_playback.json"

// PlaybackState stores queue and playback position for session persistence
type PlaybackState struct {
	// Queue tracks stored as paths (resolved against library on load)
	TrackPaths   []string   `json:"track_paths"`
	CurrentIndex int        `json:"current_index"`
	Position     float64    `json:"position"`
	Shuffle      bool       `json:"shuffle"`
	ShuffleOrder []int      `json:"shuffle_order,omitempty"`
	Repeat       RepeatMode `json:"repeat"`
	Volume       float64    `json:"volume"`
}

// savePlaybackState persists the current queue and playback position to disk
func (app *MiyooPod) savePlaybackState() {
	if app.Queue == nil || len(app.Queue.Tracks) == 0 {
		// Remove stale state file if queue is empty
		os.Remove(PLAYBACK_STATE_PATH)
		return
	}

	// Build path list from queue tracks
	paths := make([]string, len(app.Queue.Tracks))
	for i, t := range app.Queue.Tracks {
		paths[i] = t.Path
	}

	// Get current playback position — try audio backend first, fall back to poller value
	position := 0.0
	if app.Playing != nil {
		if app.Playing.State != StateStopped {
			state := audioGetState()
			if state.Position > 0 {
				position = state.Position
			} else if app.Playing.Position > 0 {
				// Audio backend may be unavailable (shutting down), use last known position
				position = app.Playing.Position
			}
		} else if app.Playing.Position > 0 {
			// Stopped but have last known position (e.g., force quit)
			position = app.Playing.Position
		}
	}

	ps := PlaybackState{
		TrackPaths:   paths,
		CurrentIndex: app.Queue.CurrentIndex,
		Position:     position,
		Shuffle:      app.Queue.Shuffle,
		ShuffleOrder: app.Queue.ShuffleOrder,
		Repeat:       app.Queue.Repeat,
	}

	data, err := json.MarshalIndent(ps, "", "  ")
	if err != nil {
		logMsg(fmt.Sprintf("ERROR: Failed to marshal playback state: %v", err))
		return
	}

	if err := os.WriteFile(PLAYBACK_STATE_PATH, data, 0644); err != nil {
		logMsg(fmt.Sprintf("ERROR: Failed to save playback state: %v", err))
	}
}

// restorePlaybackState loads a previously saved queue and resumes paused at the saved position
func (app *MiyooPod) restorePlaybackState() {
	data, err := os.ReadFile(PLAYBACK_STATE_PATH)
	if err != nil {
		// No saved state, nothing to restore
		return
	}

	var ps PlaybackState
	if err := json.Unmarshal(data, &ps); err != nil {
		logMsg(fmt.Sprintf("WARNING: Could not parse playback state: %v", err))
		return
	}

	if len(ps.TrackPaths) == 0 {
		return
	}

	// Resolve track paths against the loaded library
	if app.Library == nil || app.Library.TracksByPath == nil {
		logMsg("WARNING: Cannot restore playback state - library not loaded")
		return
	}

	tracks := make([]*Track, 0, len(ps.TrackPaths))
	pathToNewIdx := make(map[int]int) // old index -> new index mapping
	for oldIdx, path := range ps.TrackPaths {
		if t, ok := app.Library.TracksByPath[path]; ok {
			pathToNewIdx[oldIdx] = len(tracks)
			tracks = append(tracks, t)
		}
	}

	if len(tracks) == 0 {
		logMsg("WARNING: No saved queue tracks found in library")
		return
	}

	// Rebuild queue
	app.Queue.Tracks = tracks

	// Remap current index
	if newIdx, ok := pathToNewIdx[ps.CurrentIndex]; ok {
		app.Queue.CurrentIndex = newIdx
	} else {
		app.Queue.CurrentIndex = 0
	}

	// Restore shuffle state
	app.Queue.Shuffle = ps.Shuffle
	if ps.Shuffle && len(ps.ShuffleOrder) > 0 {
		// Remap shuffle order indices
		newOrder := make([]int, 0, len(ps.ShuffleOrder))
		for _, oldIdx := range ps.ShuffleOrder {
			if newIdx, ok := pathToNewIdx[oldIdx]; ok {
				newOrder = append(newOrder, newIdx)
			}
		}
		if len(newOrder) > 0 {
			app.Queue.ShuffleOrder = newOrder
		} else {
			// Shuffle order couldn't be remapped, rebuild it
			app.buildShuffleOrder(app.Queue.CurrentIndex)
		}
	}

	app.Queue.Repeat = ps.Repeat

	// Volume is now restored from settings, not playback state

	// Load the current track and pause at the saved position
	track := app.getCurrentTrack()
	if track == nil {
		return
	}

	app.Playing.Track = track
	if track.Duration > 0 {
		app.Playing.Duration = track.Duration
	}

	err = app.mpvLoadFile(track.Path)
	if err != nil {
		logMsg(fmt.Sprintf("WARNING: Failed to load saved track: %v", err))
		app.Playing.State = StateStopped
		app.Playing.Track = nil
		return
	}

	// Pause immediately - user expects to resume manually
	audioPause()
	app.Playing.State = StatePaused

	// Set position immediately so UI shows saved position
	if ps.Position > 0 {
		app.Playing.Position = ps.Position
	}

	app.updateCoverflowForCurrentTrack()
	app.NPCacheDirty = true

	logMsg(fmt.Sprintf("INFO: Restored playback state - %s at %.1fs (paused)", track.Title, ps.Position))

	// Seek to saved position in background — audio backend needs time to be ready
	if ps.Position > 0 {
		seekTarget := ps.Position
		go func() {
			for attempt := 0; attempt < 10; attempt++ {
				time.Sleep(100 * time.Millisecond)
				audioSeek(seekTarget)

				time.Sleep(50 * time.Millisecond)
				state := audioGetState()
				if state.Position > 0 && state.Duration > 0 {
					app.Playing.Position = state.Position
					app.Playing.Duration = state.Duration
					app.NPCacheDirty = true
					app.requestRedraw()
					return
				}
			}
			logMsg(fmt.Sprintf("WARNING: Could not verify seek to %.1fs for %s", seekTarget, track.Title))
		}()
	}
}
