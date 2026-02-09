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
	for displayIdx := app.QueueScrollOffset; displayIdx < totalTracks && displayIdx < app.QueueScrollOffset+visibleItems; displayIdx++ {
		// Get actual track index (respecting shuffle order)
		trackIdx := displayIdx
		if app.Queue.Shuffle && len(app.Queue.ShuffleOrder) > 0 && displayIdx < len(app.Queue.ShuffleOrder) {
			trackIdx = app.Queue.ShuffleOrder[displayIdx]
		}

		if trackIdx >= len(app.Queue.Tracks) {
			continue
		}

		track := app.Queue.Tracks[trackIdx]
		isSelected := (displayIdx == app.QueueSelectedIndex)
		isPlaying := (displayIdx == app.Queue.CurrentIndex)

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
		label := fmt.Sprintf("%s%d. %s", prefix, displayIdx+1, track.Title)
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
			app.setScreen(ScreenNowPlaying)
			app.drawCurrentScreen()
		}
		return
	}

	totalTracks := len(app.Queue.Tracks)

	switch key {
	case UP:
		if app.QueueSelectedIndex > 0 {
			app.QueueSelectedIndex--
		} else if totalTracks > 0 {
			// Wrap to bottom
			app.QueueSelectedIndex = totalTracks - 1
		}
		app.drawCurrentScreen()
	case DOWN:
		if app.QueueSelectedIndex < totalTracks-1 {
			app.QueueSelectedIndex++
		} else if totalTracks > 0 {
			// Wrap to top
			app.QueueSelectedIndex = 0
		}
		app.drawCurrentScreen()
	case B, LEFT:
		app.setScreen(ScreenNowPlaying)
		app.drawCurrentScreen()
	case A:
		// Jump to selected track and play it
		// QueueSelectedIndex is the display/playback position
		app.Queue.CurrentIndex = app.QueueSelectedIndex
		app.playCurrentQueueTrack()
		app.setScreen(ScreenNowPlaying)
		app.drawCurrentScreen()
	case X:
		// Remove selected track
		// In shuffle mode, we need to remove from the playback position
		app.removeFromQueueAtPlaybackPosition(app.QueueSelectedIndex)
	case SELECT:
		// Clear all except currently playing track
		app.clearQueueExceptCurrent()
	}
}

// clearQueueExceptCurrent removes all tracks except the currently playing one
func (app *MiyooPod) clearQueueExceptCurrent() {
	if app.Queue == nil || len(app.Queue.Tracks) == 0 {
		return
	}

	// If nothing is playing or queue is already empty, just clear everything
	if app.Playing == nil || app.Playing.State == StateStopped || app.Queue.CurrentIndex < 0 {
		app.clearQueue()
		return
	}

	// Get the currently playing track
	var currentTrack *Track
	if app.Queue.Shuffle && len(app.Queue.ShuffleOrder) > 0 {
		if app.Queue.CurrentIndex < len(app.Queue.ShuffleOrder) {
			physicalIdx := app.Queue.ShuffleOrder[app.Queue.CurrentIndex]
			if physicalIdx < len(app.Queue.Tracks) {
				currentTrack = app.Queue.Tracks[physicalIdx]
			}
		}
	} else {
		if app.Queue.CurrentIndex < len(app.Queue.Tracks) {
			currentTrack = app.Queue.Tracks[app.Queue.CurrentIndex]
		}
	}

	if currentTrack == nil {
		app.clearQueue()
		return
	}

	// Keep only the current track
	app.Queue.Tracks = []*Track{currentTrack}
	app.Queue.CurrentIndex = 0
	app.Queue.ShuffleOrder = nil
	app.Queue.Shuffle = false // Disable shuffle since only one track remains
	app.QueueSelectedIndex = 0
	app.QueueScrollOffset = 0
	app.NPCacheDirty = true
	app.drawCurrentScreen()
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

// removeFromQueueAtPlaybackPosition removes a track at the given playback position
// (respecting shuffle order if active)
func (app *MiyooPod) removeFromQueueAtPlaybackPosition(playbackPos int) {
	if app.Queue == nil || playbackPos < 0 {
		return
	}

	// Get the physical index of the track to remove
	physicalIdx := playbackPos
	if app.Queue.Shuffle && len(app.Queue.ShuffleOrder) > 0 {
		if playbackPos >= len(app.Queue.ShuffleOrder) {
			return
		}
		physicalIdx = app.Queue.ShuffleOrder[playbackPos]
	}

	if physicalIdx < 0 || physicalIdx >= len(app.Queue.Tracks) {
		return
	}

	// Don't remove if it's the only track and it's playing
	if len(app.Queue.Tracks) == 1 {
		app.clearQueue()
		return
	}

	// Remove the track from physical array
	app.Queue.Tracks = append(app.Queue.Tracks[:physicalIdx], app.Queue.Tracks[physicalIdx+1:]...)

	// Adjust shuffle order if active
	if app.Queue.Shuffle && len(app.Queue.ShuffleOrder) > 0 {
		newShuffleOrder := make([]int, 0, len(app.Queue.ShuffleOrder)-1)
		for i, shuffleIdx := range app.Queue.ShuffleOrder {
			if i == playbackPos {
				// Skip this playback position (the track we're removing)
				continue
			}
			// Adjust physical indices after the removed track
			if shuffleIdx > physicalIdx {
				newShuffleOrder = append(newShuffleOrder, shuffleIdx-1)
			} else if shuffleIdx < physicalIdx {
				newShuffleOrder = append(newShuffleOrder, shuffleIdx)
			}
			// Note: shuffleIdx == physicalIdx case is handled by i == playbackPos check
		}
		app.Queue.ShuffleOrder = newShuffleOrder
	}

	// Adjust current playback position
	if playbackPos < app.Queue.CurrentIndex {
		app.Queue.CurrentIndex--
	} else if playbackPos == app.Queue.CurrentIndex {
		// Removed the currently playing track
		// Clear the Playing.Track pointer to avoid stale reference
		if app.Playing != nil {
			app.Playing.Track = nil
		}
		maxIdx := len(app.Queue.Tracks) - 1
		if app.Queue.Shuffle && len(app.Queue.ShuffleOrder) > 0 {
			maxIdx = len(app.Queue.ShuffleOrder) - 1
		}
		if app.Queue.CurrentIndex > maxIdx {
			app.Queue.CurrentIndex = maxIdx
		}
		// Play the next track in the queue
		if len(app.Queue.Tracks) > 0 {
			app.playCurrentQueueTrack()
		}
	}

	// Adjust selected index
	maxIdx := len(app.Queue.Tracks) - 1
	if app.Queue.Shuffle && len(app.Queue.ShuffleOrder) > 0 {
		maxIdx = len(app.Queue.ShuffleOrder) - 1
	}
	if app.QueueSelectedIndex > maxIdx {
		app.QueueSelectedIndex = maxIdx
	}
	if app.QueueSelectedIndex < 0 && maxIdx >= 0 {
		app.QueueSelectedIndex = 0
	}

	app.NPCacheDirty = true
	app.drawCurrentScreen()
}

// removeFromQueue removes a track at the specified physical index
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

	// Adjust shuffle order if active (preserve existing shuffle sequence)
	if app.Queue.Shuffle && len(app.Queue.ShuffleOrder) > 0 {
		newShuffleOrder := make([]int, 0, len(app.Queue.ShuffleOrder))
		for _, shuffleIdx := range app.Queue.ShuffleOrder {
			if shuffleIdx == idx {
				// Skip the removed track
				continue
			} else if shuffleIdx > idx {
				// Adjust indices after the removed track
				newShuffleOrder = append(newShuffleOrder, shuffleIdx-1)
			} else {
				// Keep indices before the removed track
				newShuffleOrder = append(newShuffleOrder, shuffleIdx)
			}
		}
		app.Queue.ShuffleOrder = newShuffleOrder
	}

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

	// Check if track is already in queue
	if app.isTrackInQueue(track) {
		logMsg(fmt.Sprintf("Track already in queue: %s", track.Title))
		return
	}

	// Insert after current track (iPod-like behavior)
	insertPos := app.Queue.CurrentIndex + 1
	if insertPos > len(app.Queue.Tracks) {
		insertPos = len(app.Queue.Tracks)
	}

	// Insert the track
	app.Queue.Tracks = append(app.Queue.Tracks[:insertPos],
		append([]*Track{track}, app.Queue.Tracks[insertPos:]...)...)

	// If shuffle is on, extend the shuffle order without regenerating
	if app.Queue.Shuffle && len(app.Queue.ShuffleOrder) > 0 {
		// First, adjust existing shuffle indices that are >= insertPos
		// (all tracks shifted right by the insertion)
		for i := range app.Queue.ShuffleOrder {
			if app.Queue.ShuffleOrder[i] >= insertPos {
				app.Queue.ShuffleOrder[i]++
			}
		}

		// Then append the new track index to the end of shuffle order
		// This preserves the existing shuffle sequence
		app.Queue.ShuffleOrder = append(app.Queue.ShuffleOrder, insertPos)
	}

	logMsg(fmt.Sprintf("Added to queue: %s - %s", track.Artist, track.Title))
}

// addTracksToQueue appends multiple tracks to the queue
func (app *MiyooPod) addTracksToQueue(tracks []*Track) {
	if app.Queue == nil || len(tracks) == 0 {
		return
	}

	// If queue is empty, initialize with these tracks
	if len(app.Queue.Tracks) == 0 {
		app.Queue.Tracks = make([]*Track, len(tracks))
		copy(app.Queue.Tracks, tracks)
		app.Queue.CurrentIndex = 0
		if app.Queue.Shuffle {
			app.buildShuffleOrder(0)
		}
		return
	}

	// Insert after current track
	insertPos := app.Queue.CurrentIndex + 1
	if insertPos > len(app.Queue.Tracks) {
		insertPos = len(app.Queue.Tracks)
	}

	// Insert the tracks
	app.Queue.Tracks = append(app.Queue.Tracks[:insertPos],
		append(tracks, app.Queue.Tracks[insertPos:]...)...)

	// If shuffle is on, extend the shuffle order without regenerating
	if app.Queue.Shuffle && len(app.Queue.ShuffleOrder) > 0 {
		numNewTracks := len(tracks)

		// Adjust existing shuffle indices that are >= insertPos
		for i := range app.Queue.ShuffleOrder {
			if app.Queue.ShuffleOrder[i] >= insertPos {
				app.Queue.ShuffleOrder[i] += numNewTracks
			}
		}

		// Append new track indices to the end of shuffle order
		for i := 0; i < numNewTracks; i++ {
			app.Queue.ShuffleOrder = append(app.Queue.ShuffleOrder, insertPos+i)
		}
	}

	logMsg(fmt.Sprintf("Added %d tracks to queue", len(tracks)))
}
