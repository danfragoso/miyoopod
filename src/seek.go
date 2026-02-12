package main

import (
	"time"
)

const (
	seekHoldThreshold = 400 * time.Millisecond // Hold this long before seeking starts
	seekTickInterval  = 200 * time.Millisecond  // How often seek ticks fire
)

// seekAmount returns how many seconds to seek per tick, accelerating over time.
// Mimics iOS: starts at 2s/tick, ramps to 10s/tick over ~5 seconds of holding.
func (app *MiyooPod) seekAmount() float64 {
	held := time.Since(app.SeekStartTime).Seconds()
	switch {
	case held < 2:
		return 2.0
	case held < 4:
		return 5.0
	default:
		return 10.0
	}
}

// seekKeyPressed is called when L or R is pressed on the Now Playing screen.
// It records the press time and direction. The main loop calls pollSeek() to
// detect holds and perform seeking.
func (app *MiyooPod) seekKeyPressed(direction int) {
	// Ignore if already tracking a press
	if app.SeekHeld {
		return
	}
	app.SeekHeld = true
	app.SeekActive = false
	app.SeekDirection = direction
	app.SeekStartTime = time.Now()
	app.LastSeekTick = time.Time{}
}

// seekKeyReleased is called when L or R is released.
// If seek was never activated (short tap), it returns the direction for prev/next.
// Returns 0 if no action needed (seek was active or no press was tracked).
func (app *MiyooPod) seekKeyReleased() int {
	if !app.SeekHeld {
		return 0
	}

	direction := app.SeekDirection
	wasActive := app.SeekActive

	// Reset all seek state
	app.SeekHeld = false
	app.SeekActive = false
	app.SeekDirection = 0
	app.SeekStartTime = time.Time{}
	app.LastSeekTick = time.Time{}

	if wasActive {
		// Was seeking — just stop, no track skip
		return 0
	}
	// Was a short tap — return direction for prev/next
	return direction
}

// pollSeek is called from the main loop (~30Hz). It checks if a held L/R
// has passed the threshold and performs seek ticks at regular intervals.
func (app *MiyooPod) pollSeek() {
	if !app.SeekHeld || app.SeekStartTime.IsZero() {
		return
	}

	// Don't seek if not on Now Playing or nothing is playing
	if app.CurrentScreen != ScreenNowPlaying || app.Playing == nil ||
		(app.Playing.State != StatePlaying && app.Playing.State != StatePaused) {
		app.SeekHeld = false
		app.SeekActive = false
		return
	}

	elapsed := time.Since(app.SeekStartTime)

	// Haven't held long enough yet
	if elapsed < seekHoldThreshold {
		return
	}

	// Activate seeking if not already
	if !app.SeekActive {
		app.SeekActive = true
		app.LastSeekTick = time.Now()
		app.performSeekTick()
		return
	}

	// Perform seek ticks at interval
	if time.Since(app.LastSeekTick) >= seekTickInterval {
		app.LastSeekTick = time.Now()
		app.performSeekTick()
	}
}

// performSeekTick executes a single seek step in the current direction
func (app *MiyooPod) performSeekTick() {
	if app.Playing == nil || app.Playing.State == StateStopped {
		return
	}

	amount := app.seekAmount() * float64(app.SeekDirection)
	app.mpvSeek(amount)

	// Update position immediately for responsive UI
	state := audioGetState()
	if state.Position >= 0 {
		app.Playing.Position = state.Position
	}

	if app.CurrentScreen == ScreenNowPlaying {
		app.updateProgressBarOnly()
	}
}
