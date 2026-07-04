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
			if state.Duration > 0 {
				if app.Playing.Track != nil && app.Playing.Track.Duration == 0 {
					app.Playing.Track.Duration = state.Duration
				}
				// Also update the live playing duration used by the progress bar.
				// On first play track.Duration is unknown (0) until the backend
				// reports it, so without this the bar shows a 0 duration.
				if app.Playing.Duration == 0 {
					app.Playing.Duration = state.Duration
					app.NPCacheDirty = true
					app.requestRedraw()
				}
				// Lazy-fill sample rate and (average) bitrate for the format badge
				// now that the duration is known.
				if app.Playing.Track != nil {
					app.fillTrackTechInfo(app.Playing.Track, state.Duration)
				}
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
				// Defer to main loop — avoids racing on Queue/Playing state
				app.TrackEndPending = true
				app.requestRedraw()
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

	// Ensure hardware volume is set before playback starts.
	// loadSettings may have set it too early (during Init) when MI_AO isn't ready yet.
	setMiAOVolume(app.SystemVolume)

	if info, err := os.Stat(path); err == nil && info.Size() > 0 && info.Size() < maxRAMSize {
		loadErr := audioLoadFileToMemory(path)
		if loadErr == nil {
			playErr := audioPlay()
			if playErr == nil {
				// Re-apply volume now that the audio hardware is active. On a
				// restored session the track is paused immediately after load,
				// so the pre-play set above doesn't stick until this point.
				setMiAOVolume(app.SystemVolume)
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
	if err := audioPlay(); err != nil {
		return err
	}
	setMiAOVolume(app.SystemVolume)
	return nil
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
