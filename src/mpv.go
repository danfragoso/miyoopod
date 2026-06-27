package main

import (
	"fmt"
	"os"
	"time"
)

// startPlaybackPoller checks audio state and updates progress display.
// Runs in its own goroutine. Minimal work per tick to avoid starving audio.
func (app *MiyooPod) startPlaybackPoller() {
	lastDrawnSecond := -1
	saveTickCount := 0

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
				app.requestRedraw()
			} else if state.IsPlaying && app.Playing.State != StatePlaying {
				app.Playing.State = StatePlaying
				app.NPCacheDirty = true
				app.requestRedraw()
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

			// Redraw lyrics screen only when the highlighted LRC line changes,
			// and only when the user is not holding a scroll key (avoids flash during scroll).
			if app.CurrentScreen == ScreenLyrics && app.LyricsCachedLRC != nil && app.LastKey == NONE {
				activeLRC := activeLRCIndex(app.LyricsCachedLRC, app.Playing.Position)
				if activeLRC != app.LyricsLastActiveLRC {
					app.LyricsLastActiveLRC = activeLRC
					app.requestRedraw()
				}
			}

			// Save playback state every 10 seconds during active playback
			saveTickCount++
			if saveTickCount >= 10 {
				app.savePlaybackState()
				saveTickCount = 0
			}
		}
		// Increased sleep to reduce CPU usage and SD card contention
		time.Sleep(1000 * time.Millisecond)
	}
}

func (app *MiyooPod) mpvLoadFile(path string) error {
	const maxRAMSize = 20 * 1024 * 1024 // 20MB

	if info, err := os.Stat(path); err == nil && info.Size() > 0 && info.Size() < maxRAMSize {
		loadErr := audioLoadFileToMemory(path)
		if loadErr == nil {
			playErr := audioPlay()
			if playErr == nil {
				return nil
			}
			logMsg(fmt.Sprintf("WARNING: RAM play failed (%v), falling back to streaming: %s", playErr, path))
		} else {
			logMsg(fmt.Sprintf("WARNING: RAM load failed (%v), falling back to streaming: %s", loadErr, path))
		}
		audioStop()
	}

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
	if app.Playing == nil {
		return
	}
	newPos := app.Playing.Position + seconds
	if newPos < 0 {
		newPos = 0
	}
	if newPos > app.Playing.Duration && app.Playing.Duration > 0 {
		newPos = app.Playing.Duration
	}
	audioSeek(newPos)
}
