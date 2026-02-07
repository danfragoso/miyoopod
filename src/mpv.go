package main

import (
	"time"
)

// startPlaybackPoller checks audio state and updates progress display.
// Runs in its own goroutine. Minimal work per tick to avoid starving audio.
func (app *MiyooPod) startPlaybackPoller() {
	lastDrawnSecond := -1
	tickCount := 0

	for app.Running {
		if app.Playing != nil && app.Playing.State != StateStopped {
			state := audioGetState()

			if state.Position >= 0 {
				app.Playing.Position = state.Position
			}
			if state.Duration > 0 && app.Playing.Track != nil && app.Playing.Track.Duration == 0 {
				app.Playing.Track.Duration = state.Duration
			}

			if state.IsPaused && app.Playing.State != StatePaused {
				app.Playing.State = StatePaused
				app.NPCacheDirty = true
			} else if state.IsPlaying && app.Playing.State != StatePlaying {
				app.Playing.State = StatePlaying
				app.NPCacheDirty = true
			}

			if state.Finished {
				app.handleTrackEnd()
			}

			// Update progress bar when on Now Playing screen and second changes
			if app.CurrentScreen == ScreenNowPlaying {
				currentSecond := int(app.Playing.Position)
				if currentSecond != lastDrawnSecond {
					lastDrawnSecond = currentSecond
					app.updateProgressBarOnly()
				}
			}

			// Flush audio buffers every 5 seconds to prevent choppy playback
			// Mimics the fix that happens when user manually pauses/resumes
			tickCount++
			if tickCount >= 5 {
				audioFlushBuffers()
				tickCount = 0
			}
		}
		// Increased sleep to reduce CPU usage and SD card contention
		time.Sleep(1000 * time.Millisecond)
	}
}

func (app *MiyooPod) mpvLoadFile(path string) error {
	// Stream from SD card with larger buffer (128KB) to reduce underruns
	err := audioLoadFile(path)
	if err != nil {
		return err
	}
	return audioPlay()
}

func (app *MiyooPod) mpvTogglePause() {
	audioTogglePause()
}

func (app *MiyooPod) mpvStop() {
	audioStop()
}

func (app *MiyooPod) mpvSeek(seconds float64) {
	newPos := app.Playing.Position + seconds
	if newPos < 0 {
		newPos = 0
	}
	audioSeek(newPos)
}
