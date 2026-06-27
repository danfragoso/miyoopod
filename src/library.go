package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dhowden/tag"
	"github.com/fogleman/gg"
)

// startLibraryScan launches a background library scan, switching to the scan screen.
// The onComplete callback (if non-nil) is called when the scan finishes.
func (app *MiyooPod) startLibraryScan(onComplete func()) {
	if app.LibScanRunning {
		return
	}

	app.LibScanRunning = true
	app.LibScanDone = false
	app.LibScanCount = 0
	app.LibScanFolder = ""
	app.LibScanStatus = "Starting scan..."
	app.LibScanElapsed = ""
	app.LibScanPhase = "scanning"

	app.setScreen(ScreenLibraryScan)
	app.drawCurrentScreen()

	go app.runLibraryScan(onComplete)
}

// runLibraryScan is the background goroutine that performs the actual library scan.
// IMPORTANT: Never call draw functions from here — only update state and requestRedraw.
func (app *MiyooPod) runLibraryScan(onComplete func()) {
	start := time.Now()
	logMsg("INFO: Scanning music library...")

	app.Library = &Library{
		TracksByPath:  make(map[string]*Track),
		AlbumsByKey:   make(map[string]*Album),
		ArtistsByName: make(map[string]*Artist),
	}

	fileCount := 0

	filepath.Walk(MUSIC_ROOT, func(path string, info os.FileInfo, err error) error {
		if err != nil || !app.Running {
			return nil
		}
		if info.IsDir() {
			if isSystemPath(info.Name()) {
				return filepath.SkipDir
			}
			app.LibScanFolder = path
			return nil
		}

		if isSystemPath(filepath.Base(path)) {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".mp3", ".flac", ".ogg":
			app.scanTrack(path)
			fileCount++
			app.LibScanCount = fileCount

			if fileCount%5 == 0 {
				app.LibScanStatus = fmt.Sprintf("Found %d songs...", fileCount)
				app.requestRedraw()
			}

		case ".m3u", ".m3u8":
			app.Library.Playlists = append(app.Library.Playlists, &Playlist{
				Name: strings.TrimSuffix(filepath.Base(path), ext),
				Path: path,
			})
		}

		return nil
	})

	// Sort phase
	app.LibScanPhase = "sorting"
	app.LibScanStatus = "Sorting library..."
	app.requestRedraw()

	sort.Slice(app.Library.Tracks, func(i, j int) bool {
		return strings.ToLower(app.Library.Tracks[i].Title) < strings.ToLower(app.Library.Tracks[j].Title)
	})
	sort.Slice(app.Library.Albums, func(i, j int) bool {
		return strings.ToLower(app.Library.Albums[i].Name) < strings.ToLower(app.Library.Albums[j].Name)
	})
	sort.Slice(app.Library.Artists, func(i, j int) bool {
		return strings.ToLower(app.Library.Artists[i].Name) < strings.ToLower(app.Library.Artists[j].Name)
	})
	for _, album := range app.Library.Albums {
		sort.Slice(album.Tracks, func(i, j int) bool {
			if album.Tracks[i].DiscNum != album.Tracks[j].DiscNum {
				return album.Tracks[i].DiscNum < album.Tracks[j].DiscNum
			}
			return album.Tracks[i].TrackNum < album.Tracks[j].TrackNum
		})
	}
	for _, artist := range app.Library.Artists {
		sort.Slice(artist.Albums, func(i, j int) bool {
			return strings.ToLower(artist.Albums[i].Name) < strings.ToLower(artist.Albums[j].Name)
		})
	}

	// Parse playlists
	for _, pl := range app.Library.Playlists {
		app.parsePlaylist(pl)
	}

	// Decode album art
	app.LibScanPhase = "decoding"
	app.LibScanStatus = "Decoding album art..."
	app.requestRedraw()

	app.decodeAlbumArt()

	// Save to JSON
	app.LibScanPhase = "saving"
	app.LibScanStatus = "Saving library..."
	app.requestRedraw()

	if err := app.saveLibraryJSON(); err != nil {
		logMsg(fmt.Sprintf("WARNING: Failed to save library: %v", err))
	}

	elapsed := time.Since(start)
	app.LibScanElapsed = elapsed.Truncate(time.Second).String()
	app.LibScanStatus = fmt.Sprintf("%d tracks, %d albums, %d artists",
		len(app.Library.Tracks), len(app.Library.Albums), len(app.Library.Artists))

	logMsg(fmt.Sprintf("INFO: Library scan complete: %d tracks, %d albums, %d artists, %d playlists",
		len(app.Library.Tracks), len(app.Library.Albums), len(app.Library.Artists), len(app.Library.Playlists)))

	app.LibScanRunning = false

	// Call onComplete before marking done — ensures menu is rebuilt before user can navigate away
	if onComplete != nil {
		onComplete()
	}

	app.LibScanDone = true
	app.requestRedraw()
}

// scanTrack reads metadata from a single audio file
func (app *MiyooPod) scanTrack(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	track := &Track{Path: path}

	m, err := tag.ReadFrom(f)
	if err == nil {
		track.Title = m.Title()
		track.Artist = m.Artist()
		track.Album = m.Album()
		track.AlbumArtist = m.AlbumArtist()
		track.TrackNum, track.TrackTotal = m.Track()
		track.DiscNum, _ = m.Disc()
		track.Year = m.Year()
		track.Genre = m.Genre()
		track.Lyrics = m.Lyrics()

		if pic := m.Picture(); pic != nil {
			track.HasArt = true
			logMsg(fmt.Sprintf("[SCAN] Track has art: %s | Size: %d bytes, Type: %s, Ext: %s",
				filepath.Base(path), len(pic.Data), pic.MIMEType, pic.Ext))
		} else {
			logMsg(fmt.Sprintf("[SCAN] Track has NO art: %s | Format: %T",
				filepath.Base(path), m))
		}
	} else {
		logMsg(fmt.Sprintf("[SCAN] Tag read error: %s | Error: %v", filepath.Base(path), err))
	}

	// Extract duration using SDL_mixer
	track.Duration = audioGetDurationForFile(path)
	if track.Duration == 0 {
		logMsg(fmt.Sprintf("[SCAN] Warning: Could not extract duration for: %s", filepath.Base(path)))
	}

	// Derive average bitrate from file size and duration (kbps)
	if track.Duration > 0 {
		if info, err := os.Stat(path); err == nil {
			track.Bitrate = int(float64(info.Size()) * 8 / track.Duration / 1000)
		}
	}

	// Read sample rate from file header
	track.SampleRate = readSampleRate(path)

	// Fallback: use filename as title
	if track.Title == "" {
		track.Title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if track.Artist == "" {
		track.Artist = "Unknown Artist"
	}
	if track.Album == "" {
		track.Album = "Unknown Album"
	}

	// Register track
	app.Library.Tracks = append(app.Library.Tracks, track)
	app.Library.TracksByPath[path] = track

	// Build album key
	albumArtist := track.AlbumArtist
	if albumArtist == "" {
		albumArtist = track.Artist
	}
	albumKey := albumArtist + "|" + track.Album

	// Register or get album
	album, exists := app.Library.AlbumsByKey[albumKey]
	if !exists {
		album = &Album{
			Name:   track.Album,
			Artist: albumArtist,
		}
		app.Library.AlbumsByKey[albumKey] = album
		app.Library.Albums = append(app.Library.Albums, album)
	}
	album.Tracks = append(album.Tracks, track)

	// Extract art for album (first track with art wins)
	if track.HasArt && album.ArtData == nil && album.ArtPath == "" {
		logMsg(fmt.Sprintf("[EXTRACT] Attempting to extract art for album: %s - %s from %s",
			album.Artist, album.Name, filepath.Base(track.Path)))
		f.Seek(0, 0)
		if m2, err2 := tag.ReadFrom(f); err2 == nil {
			if pic := m2.Picture(); pic != nil {
				album.ArtData = pic.Data
				album.ArtExt = pic.Ext
				logMsg(fmt.Sprintf("[EXTRACT] ✓ SUCCESS: %s - %s | Source: %s | Size: %d bytes, Type: %s, Ext: %s",
					album.Artist, album.Name, filepath.Base(track.Path), len(pic.Data), pic.MIMEType, pic.Ext))

				// Save to disk to avoid re-extraction on next startup
				if err := app.saveAlbumArtwork(album); err != nil {
					logMsg(fmt.Sprintf("[EXTRACT] Warning: Failed to save artwork to disk: %v", err))
				}
			} else {
				logMsg(fmt.Sprintf("[EXTRACT] ✗ FAILED: Track has art flag but Picture() returned nil: %s | Format: %T",
					track.Path, m2))
			}
		} else {
			logMsg(fmt.Sprintf("[EXTRACT] ✗ FAILED: Re-read tag error: %s | Error: %v", track.Path, err2))
		}
	} else if !track.HasArt && album.ArtData == nil && album.ArtPath == "" {
		logMsg(fmt.Sprintf("[EXTRACT] Skipping %s - track.HasArt=false, album %s - %s has no art yet",
			filepath.Base(track.Path), album.Artist, album.Name))
	}

	// Register artist
	artist, exists := app.Library.ArtistsByName[albumArtist]
	if !exists {
		artist = &Artist{Name: albumArtist}
		app.Library.ArtistsByName[albumArtist] = artist
		app.Library.Artists = append(app.Library.Artists, artist)
	}

	// Avoid duplicate album refs on same artist
	found := false
	for _, a := range artist.Albums {
		if a == album {
			found = true
			break
		}
	}
	if !found {
		artist.Albums = append(artist.Albums, album)
	}
}

// fetchMissingAlbumArt fetches album artwork from MusicBrainz for albums without embedded art
func (app *MiyooPod) fetchMissingAlbumArt() {
	missingCount := 0
	fetchedCount := 0

	// Count albums without art
	for _, album := range app.Library.Albums {
		if album.ArtData == nil {
			missingCount++
		}
	}

	if missingCount == 0 {
		logMsg("[MUSICBRAINZ] All albums have artwork, skipping MusicBrainz fetch")
		return
	}

	logMsg(fmt.Sprintf("[MUSICBRAINZ] Fetching artwork for %d albums without embedded art...", missingCount))

	for _, album := range app.Library.Albums {
		if album.ArtData == nil && album.Name != "" && album.Artist != "" {
			if app.fetchAlbumArtFromMusicBrainz(album) {
				fetchedCount++
			}
		}
	}

	logMsg(fmt.Sprintf("[MUSICBRAINZ] Fetched artwork for %d/%d albums", fetchedCount, missingCount))
}

// decodeArtwork decodes raw artwork bytes into an image
func (app *MiyooPod) decodeArtwork(artData []byte, artExt string) image.Image {
	if len(artData) == 0 {
		return nil
	}

	reader := bytes.NewReader(artData)
	img, _, err := image.Decode(reader)
	if err != nil {
		logMsg(fmt.Sprintf("WARNING: Failed to decode artwork: %v", err))
		return nil
	}

	return img
}

// decodeAlbumArt decodes raw art bytes into images for all albums
// NOW: Only decode on-demand to save memory and startup time
func (app *MiyooPod) decodeAlbumArt() {
	start := time.Now()

	size := COVER_CENTER_SIZE
	successCount := 0
	failCount := 0
	noArtCount := 0
	rgbaCacheHits := 0

	for i, album := range app.Library.Albums {
		key := fmt.Sprintf("%s|%s_%d", album.Artist, album.Name, size)

		// Fast path: check for pre-resized RGBA pixel cache
		rgbaPath := app.rgbaCachePath(album)
		if rgbaPath != "" {
			if img := app.loadRGBACache(rgbaPath, size); img != nil {
				app.Coverflow.CoverCache[key] = img
				album.ArtData = nil
				album.ArtImg = nil
				rgbaCacheHits++
				successCount++
				continue
			}
		}

		// No RGBA cache — need to decode from source image
		if album.ArtData == nil {
			if album.ArtPath != "" {
				if err := app.loadAlbumArtwork(album); err != nil {
					noArtCount++
					continue
				}
			} else {
				noArtCount++
				continue
			}
		}

		logMsg(fmt.Sprintf("[ART] Decoding %d/%d: %s - %s (%d bytes)",
			i+1, len(app.Library.Albums), album.Artist, album.Name, len(album.ArtData)))

		reader := bytes.NewReader(album.ArtData)
		img, _, err := image.Decode(reader)
		if err != nil {
			failCount++
			logMsg(fmt.Sprintf("WARNING: Failed to decode art for %s - %s: %v", album.Artist, album.Name, err))
			continue
		}

		successCount++

		// Resize to target size
		srcBounds := img.Bounds()
		dc := gg.NewContext(size, size)
		sx := float64(size) / float64(srcBounds.Dx())
		sy := float64(size) / float64(srcBounds.Dy())
		dc.Scale(sx, sy)
		dc.DrawImage(img, 0, 0)
		resized := dc.Image()
		app.Coverflow.CoverCache[key] = resized

		// Save RGBA cache for next startup
		if rgbaPath != "" {
			if rgba, ok := resized.(*image.RGBA); ok {
				app.saveRGBACache(rgbaPath, rgba)
			}
		}

		album.ArtData = nil
		album.ArtImg = nil
	}

	logMsg(fmt.Sprintf("INFO: Album art: %d cached (%d RGBA fast), %d decoded, %d failed, %d no art | %v",
		successCount, rgbaCacheHits, successCount-rgbaCacheHits, failCount, noArtCount, time.Since(start)))

	// Generate default album art
	dc := gg.NewContext(size, size)
	dc.SetHexColor("#333333")
	dc.Clear()
	dc.SetHexColor("#666666")
	if app.FontSmall != nil {
		dc.SetFontFace(app.FontSmall)
		dc.DrawStringAnchored("No Art", float64(size)/2, float64(size)/2, 0.5, 0.5)
	}
	app.DefaultArt = dc.Image()
	app.Coverflow.CoverCache[fmt.Sprintf("__default__%d", size)] = dc.Image()
}

// rgbaCachePath returns the path for an album's pre-resized RGBA cache file, or "" if no art
func (app *MiyooPod) rgbaCachePath(album *Album) string {
	if album.ArtPath == "" && album.ArtData == nil {
		return ""
	}
	hash := generateAlbumCacheKey(album.Artist, album.Name)
	return fmt.Sprintf("%s%s_%d.rgba", ARTWORK_DIR, hash, COVER_CENTER_SIZE)
}

// loadRGBACache loads a pre-resized RGBA pixel buffer from disk
func (app *MiyooPod) loadRGBACache(path string, size int) image.Image {
	expectedSize := size * size * 4 // RGBA = 4 bytes per pixel
	data, err := os.ReadFile(path)
	if err != nil || len(data) != expectedSize {
		return nil
	}
	img := &image.RGBA{
		Pix:    data,
		Stride: size * 4,
		Rect:   image.Rect(0, 0, size, size),
	}
	return img
}

// saveRGBACache writes a pre-resized RGBA pixel buffer to disk
func (app *MiyooPod) saveRGBACache(path string, img *image.RGBA) {
	os.MkdirAll(ARTWORK_DIR, 0755)
	if err := os.WriteFile(path, img.Pix, 0644); err != nil {
		logMsg(fmt.Sprintf("WARNING: Failed to save RGBA cache: %v", err))
	}
}

// deferredArtExtraction runs in background after startup to extract album art
// from MP3 tags for albums that don't have cached artwork yet.
func (app *MiyooPod) deferredArtExtraction() {
	extracted := 0
	for _, album := range app.Library.Albums {
		// Skip albums that already have art (loaded from cache or RGBA)
		if album.ArtData != nil || album.ArtPath != "" {
			continue
		}

		// Try extracting from MP3 files
		for _, track := range album.Tracks {
			f, err := os.Open(track.Path)
			if err != nil {
				continue
			}
			m, err := tag.ReadFrom(f)
			if err != nil {
				f.Close()
				continue
			}
			pic := m.Picture()
			f.Close()
			if pic == nil {
				continue
			}

			album.ArtData = pic.Data
			album.ArtExt = pic.Ext

			// Save to disk for next time
			if err := app.saveAlbumArtwork(album); err == nil {
				// Also generate RGBA cache
				rgbaPath := app.rgbaCachePath(album)
				if rgbaPath != "" {
					reader := bytes.NewReader(pic.Data)
					if img, _, err := image.Decode(reader); err == nil {
						size := COVER_CENTER_SIZE
						dc := gg.NewContext(size, size)
						srcBounds := img.Bounds()
						dc.Scale(float64(size)/float64(srcBounds.Dx()), float64(size)/float64(srcBounds.Dy()))
						dc.DrawImage(img, 0, 0)
						resized := dc.Image()

						key := fmt.Sprintf("%s|%s_%d", album.Artist, album.Name, size)
						app.Coverflow.CoverCache[key] = resized

						if rgba, ok := resized.(*image.RGBA); ok {
							app.saveRGBACache(rgbaPath, rgba)
						}
					}
				}
			}

			album.ArtData = nil
			extracted++
			app.requestRedraw()
			break // Got art for this album
		}
	}

	if extracted > 0 {
		logMsg(fmt.Sprintf("INFO: Background art extraction: found art for %d albums", extracted))
		app.NPCacheDirty = true
		app.requestRedraw()
	}
}

// drawLibraryScanScreen renders the library scan progress screen.
// Called from drawCurrentScreen on the main thread only.
func (app *MiyooPod) drawLibraryScanScreen() {
	dc := app.DC

	dc.SetHexColor(app.CurrentTheme.BG)
	dc.Clear()

	app.drawHeader("Scanning Library")

	if app.LibScanDone {
		app.drawLibraryScanResults()
		return
	}

	y := MENU_TOP_Y

	// Row 1: Track count
	dc.SetFontFace(app.FontMenu)
	dc.SetHexColor(app.CurrentTheme.ItemTxt)
	textY := float64(y) + float64(MENU_ITEM_HEIGHT)/2
	dc.DrawStringAnchored(
		fmt.Sprintf("%d songs found", app.LibScanCount),
		float64(MENU_LEFT_PAD), textY, 0, 0.5,
	)
	y += MENU_ITEM_HEIGHT

	// Row 2: Current folder (highlighted like selected menu item)
	if app.LibScanFolder != "" {
		dc.SetHexColor(app.CurrentTheme.SelBG)
		dc.DrawRectangle(0, float64(y), SCREEN_WIDTH, MENU_ITEM_HEIGHT)
		dc.Fill()

		dc.SetFontFace(app.FontSmall)
		dc.SetHexColor(app.CurrentTheme.SelTxt)
		displayPath := app.LibScanFolder
		if len(displayPath) > 60 {
			displayPath = "..." + displayPath[len(displayPath)-57:]
		}
		textY = float64(y) + float64(MENU_ITEM_HEIGHT)/2
		dc.DrawStringAnchored(displayPath, float64(MENU_LEFT_PAD), textY, 0, 0.5)
		y += MENU_ITEM_HEIGHT
	}

	// Row 3: Status/phase
	if app.LibScanStatus != "" {
		dc.SetFontFace(app.FontSmall)
		dc.SetHexColor(app.CurrentTheme.Dim)
		textY = float64(y) + float64(MENU_ITEM_HEIGHT)/2
		dc.DrawStringAnchored(app.LibScanStatus, float64(MENU_LEFT_PAD), textY, 0, 0.5)
	}
}

// drawLibraryScanResults renders the results after library scan is complete.
func (app *MiyooPod) drawLibraryScanResults() {
	dc := app.DC
	y := MENU_TOP_Y

	// Row 1: Completion message
	dc.SetFontFace(app.FontMenu)
	dc.SetHexColor(app.CurrentTheme.ItemTxt)
	textY := float64(y) + float64(MENU_ITEM_HEIGHT)/2
	title := "Scan complete"
	if app.LibScanElapsed != "" {
		title = fmt.Sprintf("Scan complete in %s", app.LibScanElapsed)
	}
	dc.DrawStringAnchored(title, float64(MENU_LEFT_PAD), textY, 0, 0.5)
	y += MENU_ITEM_HEIGHT

	// Separator
	dc.SetHexColor(app.CurrentTheme.ProgBG)
	dc.DrawRectangle(float64(MENU_LEFT_PAD), float64(y), float64(SCREEN_WIDTH-MENU_LEFT_PAD-MENU_RIGHT_PAD), 1)
	dc.Fill()
	y += 8

	if app.Library != nil {
		// Tracks
		dc.SetFontFace(app.FontMenu)
		dc.SetHexColor(app.CurrentTheme.ItemTxt)
		textY = float64(y) + float64(MENU_ITEM_HEIGHT)/2
		dc.DrawStringAnchored("Tracks", float64(MENU_LEFT_PAD), textY, 0, 0.5)
		dc.SetHexColor(app.CurrentTheme.Accent)
		dc.DrawStringAnchored(fmt.Sprintf("%d", len(app.Library.Tracks)), float64(SCREEN_WIDTH-MENU_RIGHT_PAD), textY, 1, 0.5)
		y += MENU_ITEM_HEIGHT

		// Albums
		dc.SetFontFace(app.FontMenu)
		dc.SetHexColor(app.CurrentTheme.ItemTxt)
		textY = float64(y) + float64(MENU_ITEM_HEIGHT)/2
		dc.DrawStringAnchored("Albums", float64(MENU_LEFT_PAD), textY, 0, 0.5)
		dc.SetHexColor(app.CurrentTheme.Accent)
		dc.DrawStringAnchored(fmt.Sprintf("%d", len(app.Library.Albums)), float64(SCREEN_WIDTH-MENU_RIGHT_PAD), textY, 1, 0.5)
		y += MENU_ITEM_HEIGHT

		// Artists
		dc.SetFontFace(app.FontMenu)
		dc.SetHexColor(app.CurrentTheme.ItemTxt)
		textY = float64(y) + float64(MENU_ITEM_HEIGHT)/2
		dc.DrawStringAnchored("Artists", float64(MENU_LEFT_PAD), textY, 0, 0.5)
		dc.SetHexColor(app.CurrentTheme.Accent)
		dc.DrawStringAnchored(fmt.Sprintf("%d", len(app.Library.Artists)), float64(SCREEN_WIDTH-MENU_RIGHT_PAD), textY, 1, 0.5)
		y += MENU_ITEM_HEIGHT

		// Playlists
		if len(app.Library.Playlists) > 0 {
			dc.SetFontFace(app.FontMenu)
			dc.SetHexColor(app.CurrentTheme.ItemTxt)
			textY = float64(y) + float64(MENU_ITEM_HEIGHT)/2
			dc.DrawStringAnchored("Playlists", float64(MENU_LEFT_PAD), textY, 0, 0.5)
			dc.SetHexColor(app.CurrentTheme.Accent)
			dc.DrawStringAnchored(fmt.Sprintf("%d", len(app.Library.Playlists)), float64(SCREEN_WIDTH-MENU_RIGHT_PAD), textY, 1, 0.5)
		}
	}
}

// drawLibraryScanStatusBar renders the status bar for the library scan screen.
func (app *MiyooPod) drawLibraryScanStatusBar() {
	dc := app.DC

	barY := float64(SCREEN_HEIGHT - STATUS_BAR_HEIGHT)

	dc.SetHexColor(app.CurrentTheme.HeaderBG)
	dc.DrawRectangle(0, barY, SCREEN_WIDTH, STATUS_BAR_HEIGHT)
	dc.Fill()

	centerY := barY + float64(STATUS_BAR_HEIGHT)/2

	if app.LibScanDone {
		app.drawButtonLegend(12, centerY, "B", "Back")
	} else {
		app.drawButtonLegend(12, centerY, "", "Scanning...")
	}
}

// handleLibraryScanKey handles key input on the library scan screen.
func (app *MiyooPod) handleLibraryScanKey(key Key) {
	switch key {
	case B, MENU:
		if app.LibScanDone {
			app.LibScanDone = false
			app.setScreen(ScreenMenu)
			app.drawCurrentScreen()
		}
		// Don't allow cancelling during scan — library state would be inconsistent
	}
}

// saveLibraryJSON writes the library to a JSON file
func (app *MiyooPod) saveLibraryJSON() error {
	if app.Library == nil {
		return fmt.Errorf("library is nil")
	}

	logMsg("Saving library to JSON...")
	start := time.Now()

	data, err := json.MarshalIndent(app.Library, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal library: %v", err)
	}

	err = os.WriteFile(LIBRARY_JSON_PATH, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write library file: %v", err)
	}

	logMsg(fmt.Sprintf("INFO: Library saved to JSON in %v", time.Since(start)))
	return nil
}

// loadLibraryJSON loads the library from a JSON file
func (app *MiyooPod) loadLibraryJSON() error {
	logMsg("Loading library from JSON...")
	start := time.Now()

	data, err := os.ReadFile(LIBRARY_JSON_PATH)
	if err != nil {
		return fmt.Errorf("failed to read library file: %v", err)
	}

	lib := &Library{
		TracksByPath:  make(map[string]*Track),
		AlbumsByKey:   make(map[string]*Album),
		ArtistsByName: make(map[string]*Artist),
	}

	err = json.Unmarshal(data, lib)
	if err != nil {
		return fmt.Errorf("failed to unmarshal library: %v", err)
	}

	// Filter out system metadata files that may be in cached library from a previous scan
	filtered := lib.Tracks[:0]
	for _, track := range lib.Tracks {
		if !isSystemPath(filepath.Base(track.Path)) {
			filtered = append(filtered, track)
		}
	}
	lib.Tracks = filtered

	app.Library = lib

	// Rebuild lookup maps
	for _, track := range lib.Tracks {
		lib.TracksByPath[track.Path] = track
	}

	for _, album := range lib.Albums {
		albumKey := album.Artist + "|" + album.Name
		lib.AlbumsByKey[albumKey] = album
	}

	for _, artist := range lib.Artists {
		lib.ArtistsByName[artist.Name] = artist
	}

	// Rebuild relationships between tracks, albums, and artists
	for _, album := range lib.Albums {
		album.Tracks = nil // Clear tracks
	}

	for _, artist := range lib.Artists {
		artist.Albums = nil // Clear albums
	}

	// Rebuild track-album relationships and extract album art
	for _, track := range lib.Tracks {
		albumArtist := track.AlbumArtist
		if albumArtist == "" {
			albumArtist = track.Artist
		}
		albumKey := albumArtist + "|" + track.Album

		if album, exists := lib.AlbumsByKey[albumKey]; exists {
			album.Tracks = append(album.Tracks, track)
		}
	}

	// Artwork loading is handled by decodeAlbumArt() which checks RGBA pixel cache first
	// (fast file read), falling back to raw image decode only when needed.
	// MP3 art extraction for albums without any cached art is deferred to background.

	// Rebuild artist-album relationships
	for _, album := range lib.Albums {
		if artist, exists := lib.ArtistsByName[album.Artist]; exists {
			// Check if album is already in artist's list
			found := false
			for _, a := range artist.Albums {
				if a == album {
					found = true
					break
				}
			}
			if !found {
				artist.Albums = append(artist.Albums, album)
			}
		}
	}

	// Parse playlists (they're just references, need to be re-read)
	for _, pl := range lib.Playlists {
		app.parsePlaylist(pl)
	}

	// Decode album art
	app.decodeAlbumArt()

	logMsg(fmt.Sprintf("INFO: Library loaded from JSON: %d tracks, %d albums, %d artists, %d playlists in %v",
		len(lib.Tracks), len(lib.Albums), len(lib.Artists), len(lib.Playlists), time.Since(start)))

	return nil
}

// readSampleRate parses the sample rate directly from the audio file header.
// Supports FLAC (STREAMINFO block), OGG Vorbis (identification header), and MP3 (sync frame).
func readSampleRate(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".flac":
		// fLaC marker (4) + block header (4) + 2 bytes min block size + 2 bytes max block size
		// + 3 bytes min frame size + 3 bytes max frame size = 14 bytes before the sample rate field.
		// Sample rate is the top 20 bits of the next 3 bytes.
		var magic [4]byte
		if err := binary.Read(f, binary.BigEndian, &magic); err != nil || string(magic[:]) != "fLaC" {
			return 0
		}
		// Skip 4-byte block header
		var hdr [4]byte
		if _, err := f.Read(hdr[:]); err != nil {
			return 0
		}
		// Skip 10 bytes (min/max block sizes + min/max frame sizes)
		if _, err := f.Seek(10, 1); err != nil {
			return 0
		}
		// Read 3 bytes; sample rate is the upper 20 bits
		var b [3]byte
		if _, err := f.Read(b[:]); err != nil {
			return 0
		}
		return int(b[0])<<12 | int(b[1])<<4 | int(b[2])>>4

	case ".ogg":
		// Scan for the vorbis identification packet "\x01vorbis",
		// then 4-byte version + 1-byte channels + 4-byte LE sample rate.
		buf := make([]byte, 256)
		n, _ := f.Read(buf)
		idx := bytes.Index(buf[:n], []byte("\x01vorbis"))
		if idx < 0 {
			return 0
		}
		off := idx + 11 // 7 header bytes + 4 version bytes
		if off+4 > n {
			return 0
		}
		return int(binary.LittleEndian.Uint32(buf[off : off+4]))

	case ".mp3":
		// Scan for the first valid MP3 sync frame (0xFF 0xE0 mask).
		buf := make([]byte, 4096)
		n, _ := f.Read(buf)
		mp3SampleRates := [4][4]int{
			{11025, 12000, 8000, 0},  // MPEG 2.5
			{0, 0, 0, 0},             // reserved
			{22050, 24000, 16000, 0}, // MPEG 2
			{44100, 48000, 32000, 0}, // MPEG 1
		}
		for i := 0; i < n-3; i++ {
			if buf[i] == 0xFF && (buf[i+1]&0xE0) == 0xE0 {
				mpegVer := (buf[i+1] >> 3) & 0x3
				srIdx := (buf[i+2] >> 2) & 0x3
				if srIdx < 3 {
					return mp3SampleRates[mpegVer][srIdx]
				}
			}
		}
	}
	return 0
}

// isSystemPath returns true for filesystem metadata paths that should be skipped
// during library scanning. Covers macOS (._ AppleDouble, .DS_Store, __MACOSX,
// .fseventsd, .Spotlight-V100, .Trashes) and Windows (Thumbs.db, desktop.ini).
func isSystemPath(name string) bool {
	switch name {
	case ".DS_Store", "Thumbs.db", "desktop.ini",
		".fseventsd", ".Spotlight-V100", ".Trashes",
		"lost+found", "__MACOSX":
		return true
	}
	return strings.HasPrefix(name, "._") || strings.HasPrefix(name, "__MACOSX")
}
