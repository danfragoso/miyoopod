package main

import (
	"fmt"
	"image"
	"math/rand"
)

// playTrackFromList starts playing a track from a list, building a queue from the context
func (app *MiyooPod) playTrackFromList(tracks []*Track, startIdx int) {
	if len(tracks) == 0 || startIdx < 0 || startIdx >= len(tracks) {
		return
	}

	app.Queue.Tracks = make([]*Track, len(tracks))
	copy(app.Queue.Tracks, tracks)
	app.Queue.CurrentIndex = startIdx

	if app.Queue.Shuffle {
		app.buildShuffleOrder(startIdx)
	}

	app.playCurrentQueueTrack()

	app.CurrentScreen = ScreenNowPlaying
	app.refreshRootMenu()
	app.drawCurrentScreen()
}

// shuffleAllAndPlay queues all library tracks, shuffles, and starts playing
func (app *MiyooPod) shuffleAllAndPlay() {
	if len(app.Library.Tracks) == 0 {
		return
	}

	app.Queue.Tracks = make([]*Track, len(app.Library.Tracks))
	copy(app.Queue.Tracks, app.Library.Tracks)
	app.Queue.Shuffle = true
	app.Queue.CurrentIndex = 0
	app.buildShuffleOrder(0)

	app.playCurrentQueueTrack()

	app.CurrentScreen = ScreenNowPlaying
	app.refreshRootMenu()
	app.drawCurrentScreen()
}

// playCurrentQueueTrack loads and plays the current track in the queue
func (app *MiyooPod) playCurrentQueueTrack() {
	track := app.getCurrentTrack()
	if track == nil {
		return
	}

	logMsg(fmt.Sprintf("Playing: %s - %s", track.Artist, track.Title))

	app.Playing.Track = track
	app.Playing.State = StatePlaying
	app.Playing.Position = 0
	app.Playing.Duration = track.Duration

	err := app.mpvLoadFile(track.Path)
	if err != nil {
		logMsg(fmt.Sprintf("Failed to load: %v", err))
		app.Playing.State = StateStopped
		app.showError(fmt.Sprintf("Failed to load audio\n%s", err.Error()))
	}

	app.updateCoverflowForCurrentTrack()
	app.NPCacheDirty = true
}

// getCurrentTrack returns the current track based on queue and shuffle state
func (app *MiyooPod) getCurrentTrack() *Track {
	if app.Queue == nil || len(app.Queue.Tracks) == 0 {
		return nil
	}

	idx := app.Queue.CurrentIndex
	if app.Queue.Shuffle && len(app.Queue.ShuffleOrder) > 0 {
		if idx >= 0 && idx < len(app.Queue.ShuffleOrder) {
			idx = app.Queue.ShuffleOrder[idx]
		}
	}

	if idx >= 0 && idx < len(app.Queue.Tracks) {
		return app.Queue.Tracks[idx]
	}
	return nil
}

func (app *MiyooPod) togglePlayPause() {
	if app.Playing == nil || app.Playing.State == StateStopped {
		return
	}
	app.mpvTogglePause()
	app.NPCacheDirty = true
}

func (app *MiyooPod) nextTrack() {
	if app.Queue == nil || len(app.Queue.Tracks) == 0 {
		return
	}

	app.Queue.CurrentIndex++
	if app.Queue.CurrentIndex >= len(app.Queue.Tracks) {
		if app.Queue.Repeat == RepeatAll {
			app.Queue.CurrentIndex = 0
		} else {
			app.Queue.CurrentIndex = len(app.Queue.Tracks) - 1
			app.mpvStop()
			app.Playing.State = StateStopped
			return
		}
	}

	app.playCurrentQueueTrack()
}

func (app *MiyooPod) prevTrack() {
	if app.Queue == nil || len(app.Queue.Tracks) == 0 {
		return
	}

	if app.Playing != nil && app.Playing.Position > 3.0 {
		app.mpvSeek(-app.Playing.Position)
		return
	}

	app.Queue.CurrentIndex--
	if app.Queue.CurrentIndex < 0 {
		if app.Queue.Repeat == RepeatAll {
			app.Queue.CurrentIndex = len(app.Queue.Tracks) - 1
		} else {
			app.Queue.CurrentIndex = 0
		}
	}

	app.playCurrentQueueTrack()
}

func (app *MiyooPod) handleTrackEnd() {
	if app.Queue == nil || len(app.Queue.Tracks) == 0 {
		app.Playing.State = StateStopped
		return
	}

	if app.Queue.Repeat == RepeatOne {
		app.playCurrentQueueTrack()
		return
	}

	app.Queue.CurrentIndex++
	if app.Queue.CurrentIndex >= len(app.Queue.Tracks) {
		if app.Queue.Repeat == RepeatAll {
			app.Queue.CurrentIndex = 0
			app.playCurrentQueueTrack()
		} else {
			app.Queue.CurrentIndex = len(app.Queue.Tracks) - 1
			app.Playing.State = StateStopped
			app.NPCacheDirty = true
			app.refreshRootMenu()
			app.drawCurrentScreen()
		}
		return
	}

	app.playCurrentQueueTrack()
	app.drawCurrentScreen()
}

func (app *MiyooPod) toggleShuffle() {
	if app.Queue == nil {
		return
	}
	app.Queue.Shuffle = !app.Queue.Shuffle
	if app.Queue.Shuffle && len(app.Queue.Tracks) > 0 {
		app.buildShuffleOrder(app.Queue.CurrentIndex)
	}
	app.NPCacheDirty = true
}

func (app *MiyooPod) cycleRepeat() {
	if app.Queue == nil {
		return
	}
	switch app.Queue.Repeat {
	case RepeatOff:
		app.Queue.Repeat = RepeatAll
	case RepeatAll:
		app.Queue.Repeat = RepeatOne
	case RepeatOne:
		app.Queue.Repeat = RepeatOff
	}
	app.NPCacheDirty = true
}

func (app *MiyooPod) buildShuffleOrder(startIdx int) {
	n := len(app.Queue.Tracks)
	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	for i := n - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		order[i], order[j] = order[j], order[i]
	}
	for i, v := range order {
		if v == startIdx {
			order[0], order[i] = order[i], order[0]
			break
		}
	}
	app.Queue.ShuffleOrder = order
}

func (app *MiyooPod) updateCoverflowForCurrentTrack() {
	if app.Playing == nil || app.Playing.Track == nil {
		return
	}

	track := app.Playing.Track

	if len(app.Coverflow.Albums) == 0 {
		app.Coverflow.Albums = app.Library.Albums
	}

	for i, album := range app.Coverflow.Albums {
		if album.Name == track.Album && album.Artist == track.Artist {
			app.Coverflow.CenterIndex = i
			return
		}
		albumArtist := track.AlbumArtist
		if albumArtist == "" {
			albumArtist = track.Artist
		}
		if album.Name == track.Album && album.Artist == albumArtist {
			app.Coverflow.CenterIndex = i
			return
		}
	}
}

// drawNowPlayingScreen renders the Now Playing screen with background caching.
// Static elements are cached as a pixel buffer. Only progress bar redraws each second.
func (app *MiyooPod) drawNowPlayingScreen() {
	dc := app.DC

	if app.Playing == nil || app.Playing.Track == nil {
		dc.SetHexColor(app.CurrentTheme.BG)
		dc.Clear()
		app.drawHeader("Now Playing")
		dc.SetFontFace(app.FontMenu)
		dc.SetHexColor(app.CurrentTheme.Dim)
		dc.DrawStringAnchored("No track playing", SCREEN_WIDTH/2, SCREEN_HEIGHT/2, 0.5, 0.5)
		return
	}

	if app.NowPlayingBG == nil || app.NPCacheDirty {
		app.renderNowPlayingFull()
		app.drawStatusBar()

		if app.NowPlayingBG == nil {
			app.NowPlayingBG = image.NewRGBA(image.Rect(0, 0, SCREEN_WIDTH, SCREEN_HEIGHT))
		}
		copy(app.NowPlayingBG.Pix, app.FB.Pix)
		app.NPCacheDirty = false
	} else {
		copy(app.FB.Pix, app.NowPlayingBG.Pix)
	}

	// Draw progress bar using direct pixel operations (bypass gg)
	app.fastDrawProgressBar(40, PROGRESS_BAR_Y, SCREEN_WIDTH-80, app.Playing.Position, app.Playing.Duration)
}

// updateProgressBarOnly is the fast path called by the playback poller.
// Only updates progress bar region, bypasses gg entirely.
func (app *MiyooPod) updateProgressBarOnly() {
	if app.Playing == nil || app.NowPlayingBG == nil {
		return
	}

	// Restore progress bar region from cache
	fastCopyRegion(app.FB, app.NowPlayingBG, PROGRESS_REGION_Y0, PROGRESS_REGION_Y1)

	// Draw progress bar with direct pixel ops
	app.fastDrawProgressBar(40, PROGRESS_BAR_Y, SCREEN_WIDTH-80, app.Playing.Position, app.Playing.Duration)

	// Full refresh (partial was buggy on Miyoo Mini SDL2)
	app.triggerRefresh()
}

// renderNowPlayingFull draws all static now-playing elements via gg
func (app *MiyooPod) renderNowPlayingFull() {
	dc := app.DC
	track := app.Playing.Track

	dc.SetHexColor(app.CurrentTheme.BG)
	dc.Clear()

	app.drawHeader("Now Playing")
	app.DrawCoverflow()

	// Track info on the right side of the album art
	infoX := 330
	infoStartY := 80

	// Title
	dc.SetFontFace(app.FontTitle)
	dc.SetHexColor(app.CurrentTheme.ItemTxt)
	maxWidth := float64(SCREEN_WIDTH - infoX - 20)
	titleText := app.truncateText(track.Title, maxWidth, app.FontTitle)
	dc.DrawString(titleText, float64(infoX), float64(infoStartY))

	// Artist
	dc.SetFontFace(app.FontArtist)
	dc.SetHexColor(app.CurrentTheme.Dim)
	artistText := app.truncateText(track.Artist, maxWidth, app.FontArtist)
	dc.DrawString(artistText, float64(infoX), float64(infoStartY+35))

	// Album
	if track.Album != "" && track.Album != "Unknown Album" {
		albumText := app.truncateText(track.Album, maxWidth, app.FontArtist)
		dc.DrawString(albumText, float64(infoX), float64(infoStartY+65))
	}

	// Track/Disc numbers if available
	if track.TrackNum > 0 {
		dc.SetFontFace(app.FontSmall)
		trackInfo := fmt.Sprintf("Track %d", track.TrackNum)
		if track.TrackTotal > 0 {
			trackInfo = fmt.Sprintf("Track %d/%d", track.TrackNum, track.TrackTotal)
		}
		dc.DrawString(trackInfo, float64(infoX), float64(infoStartY+95))
	}

	app.drawStatusIndicators(infoStartY + 250)
}
