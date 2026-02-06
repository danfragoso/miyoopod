package main

import "fmt"

// isTrackInQueue checks if a track is in the queue
func (app *MiyooPod) isTrackInQueue(track *Track) bool {
	if app.Queue == nil || track == nil {
		return false
	}
	for _, t := range app.Queue.Tracks {
		if t.Path == track.Path {
			return true
		}
	}
	return false
}

// drawQueueScreen renders the queue view
func (app *MiyooPod) drawQueueScreen() {
	dc := app.DC
	dc.SetHexColor(app.CurrentTheme.BG)
	dc.Clear()

	app.drawHeader("Queue")

	if app.Queue == nil || len(app.Queue.Tracks) == 0 {
		dc.SetFontFace(app.FontMenu)
		dc.SetHexColor(app.CurrentTheme.Dim)
		dc.DrawStringAnchored("Queue is empty", SCREEN_WIDTH/2, SCREEN_HEIGHT/2, 0.5, 0.5)
		return
	}

	totalTracks := len(app.Queue.Tracks)
	visibleItems := (SCREEN_HEIGHT - MENU_TOP_Y - STATUS_BAR_HEIGHT) / MENU_ITEM_HEIGHT

	// Ensure selected index is valid
	if app.QueueSelectedIndex < 0 {
		app.QueueSelectedIndex = 0
	}
	if app.QueueSelectedIndex >= totalTracks {
		app.QueueSelectedIndex = totalTracks - 1
	}

	// Adjust scroll offset to keep selected item visible
	if app.QueueSelectedIndex < app.QueueScrollOffset {
		app.QueueScrollOffset = app.QueueSelectedIndex
	}
	if app.QueueSelectedIndex >= app.QueueScrollOffset+visibleItems {
		app.QueueScrollOffset = app.QueueSelectedIndex - visibleItems + 1
	}

	// Ensure scroll offset is valid
	if app.QueueScrollOffset < 0 {
		app.QueueScrollOffset = 0
	}
	maxScroll := totalTracks - visibleItems
	if maxScroll < 0 {
		maxScroll = 0
	}
	if app.QueueScrollOffset > maxScroll {
		app.QueueScrollOffset = maxScroll
	}

	// Draw tracks
	y := MENU_TOP_Y
	for i := app.QueueScrollOffset; i < totalTracks && i < app.QueueScrollOffset+visibleItems; i++ {
		track := app.Queue.Tracks[i]
		isSelected := (i == app.QueueSelectedIndex)
		isPlaying := (i == app.Queue.CurrentIndex)

		// Highlight selected track
		if isSelected {
			dc.SetHexColor(app.CurrentTheme.SelBG)
			dc.DrawRectangle(0, float64(y), SCREEN_WIDTH, MENU_ITEM_HEIGHT)
			dc.Fill()
		}

		// Track number and title
		dc.SetFontFace(app.FontMenu)
		if isSelected {
			dc.SetHexColor(app.CurrentTheme.SelTxt)
		} else {
			dc.SetHexColor(app.CurrentTheme.ItemTxt)
		}

		// Add ▶ indicator for currently playing track
		prefix := ""
		if isPlaying {
			prefix = "▶ "
		}
		label := fmt.Sprintf("%s%d. %s", prefix, i+1, track.Title)
		maxWidth := float64(SCREEN_WIDTH - MENU_LEFT_PAD - MENU_RIGHT_PAD - 40)
		displayLabel := app.truncateText(label, maxWidth, app.FontMenu)
		textY := float64(y) + float64(MENU_ITEM_HEIGHT)/2
		dc.DrawStringAnchored(displayLabel, float64(MENU_LEFT_PAD), textY, 0, 0.5)

		// Artist name (right side)
		dc.SetFontFace(app.FontSmall)
		if isSelected {
			dc.SetHexColor(app.CurrentTheme.SelTxt)
		} else {
			dc.SetHexColor(app.CurrentTheme.Dim)
		}
		artistText := app.truncateText(track.Artist, 150, app.FontSmall)
		dc.DrawStringAnchored(artistText, float64(SCREEN_WIDTH-MENU_RIGHT_PAD), textY, 1, 0.5)

		y += MENU_ITEM_HEIGHT
	}

	// Draw scroll bar
	app.drawScrollBar(totalTracks, app.QueueScrollOffset, visibleItems)
}

// handleQueueKey processes key input when viewing the queue
func (app *MiyooPod) handleQueueKey(key Key) {
	if app.Queue == nil || len(app.Queue.Tracks) == 0 {
		if key == B || key == LEFT {
			app.CurrentScreen = ScreenNowPlaying
			app.drawCurrentScreen()
		}
		return
	}

	totalTracks := len(app.Queue.Tracks)

	switch key {
	case UP:
		if app.QueueSelectedIndex > 0 {
			app.QueueSelectedIndex--
			app.drawCurrentScreen()
		}
	case DOWN:
		if app.QueueSelectedIndex < totalTracks-1 {
			app.QueueSelectedIndex++
			app.drawCurrentScreen()
		}
	case B, LEFT:
		app.CurrentScreen = ScreenNowPlaying
		app.drawCurrentScreen()
	case A:
		// Jump to selected track and play it
		app.Queue.CurrentIndex = app.QueueSelectedIndex
		app.playCurrentQueueTrack()
		app.CurrentScreen = ScreenNowPlaying
		app.drawCurrentScreen()
	case X:
		// Remove selected track
		app.removeFromQueue(app.QueueSelectedIndex)
	case MENU:
		// Clear queue
		app.clearQueue()
	}
}

// clearQueue removes all tracks from the queue and stops playback
func (app *MiyooPod) clearQueue() {
	if app.Queue == nil {
		return
	}

	audioStop()
	app.Queue.Tracks = nil
	app.Queue.CurrentIndex = 0
	app.Queue.ShuffleOrder = nil
	if app.Playing != nil {
		app.Playing.State = StateStopped
	}
	app.QueueScrollOffset = 0
	app.NPCacheDirty = true
	app.drawCurrentScreen()
}

// removeFromQueue removes a track at the specified index
func (app *MiyooPod) removeFromQueue(idx int) {
	if app.Queue == nil || idx < 0 || idx >= len(app.Queue.Tracks) {
		return
	}

	// Don't remove if it's the only track and it's playing
	if len(app.Queue.Tracks) == 1 {
		app.clearQueue()
		return
	}

	// Remove the track
	app.Queue.Tracks = append(app.Queue.Tracks[:idx], app.Queue.Tracks[idx+1:]...)

	// Adjust current index if needed
	if idx < app.Queue.CurrentIndex {
		app.Queue.CurrentIndex--
	} else if idx == app.Queue.CurrentIndex {
		// Removed the currently playing track
		if app.Queue.CurrentIndex >= len(app.Queue.Tracks) {
			app.Queue.CurrentIndex = len(app.Queue.Tracks) - 1
		}
		// Play the next track in the queue
		if len(app.Queue.Tracks) > 0 {
			app.playCurrentQueueTrack()
		}
	}

	// Rebuild shuffle order if needed
	if app.Queue.Shuffle {
		app.buildShuffleOrder(app.Queue.CurrentIndex)
	}

	// Adjust selected index
	if app.QueueSelectedIndex >= len(app.Queue.Tracks) {
		app.QueueSelectedIndex = len(app.Queue.Tracks) - 1
	}
	if app.QueueSelectedIndex < 0 {
		app.QueueSelectedIndex = 0
	}

	app.NPCacheDirty = true
	app.drawCurrentScreen()
}

// addToQueue appends a track to the queue
func (app *MiyooPod) addToQueue(track *Track) {
	if app.Queue == nil {
		return
	}

	// If queue is empty, initialize it with this track and start playing
	if len(app.Queue.Tracks) == 0 {
		app.Queue.Tracks = []*Track{track}
		app.Queue.CurrentIndex = 0
		app.playCurrentQueueTrack()
		app.NPCacheDirty = true
		return
	}

	app.Queue.Tracks = append(app.Queue.Tracks, track)

	// Rebuild shuffle order if needed
	if app.Queue.Shuffle {
		app.buildShuffleOrder(app.Queue.CurrentIndex)
	}
}

// addTracksToQueue appends multiple tracks to the queue
func (app *MiyooPod) addTracksToQueue(tracks []*Track) {
	if app.Queue == nil {
		return
	}

	app.Queue.Tracks = append(app.Queue.Tracks, tracks...)

	// Rebuild shuffle order if needed
	if app.Queue.Shuffle {
		app.buildShuffleOrder(app.Queue.CurrentIndex)
	}
}
