package main

import (
	"bytes"
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

// ScanLibrary walks the music directory and builds the library index
func (app *MiyooPod) ScanLibrary() {
	start := time.Now()
	logMsg("Scanning music library...")

	app.Library = &Library{
		TracksByPath:  make(map[string]*Track),
		AlbumsByKey:   make(map[string]*Album),
		ArtistsByName: make(map[string]*Artist),
	}

	fileCount := 0
	currentFolder := ""

	filepath.Walk(MUSIC_ROOT, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			currentFolder = path
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".mp3":
			app.scanTrack(path)
			fileCount++

			// Update scanning progress display every 5 files
			if fileCount%5 == 0 {
				app.showScanningProgress(fileCount, currentFolder)
			}

		case ".m3u", ".m3u8":
			app.Library.Playlists = append(app.Library.Playlists, &Playlist{
				Name: strings.TrimSuffix(filepath.Base(path), ext),
				Path: path,
			})
		}

		return nil
	})

	// Sort tracks by title
	sort.Slice(app.Library.Tracks, func(i, j int) bool {
		return strings.ToLower(app.Library.Tracks[i].Title) < strings.ToLower(app.Library.Tracks[j].Title)
	})

	// Sort albums by name
	sort.Slice(app.Library.Albums, func(i, j int) bool {
		return strings.ToLower(app.Library.Albums[i].Name) < strings.ToLower(app.Library.Albums[j].Name)
	})

	// Sort artists by name
	sort.Slice(app.Library.Artists, func(i, j int) bool {
		return strings.ToLower(app.Library.Artists[i].Name) < strings.ToLower(app.Library.Artists[j].Name)
	})

	// Sort tracks within each album by disc/track number
	for _, album := range app.Library.Albums {
		sort.Slice(album.Tracks, func(i, j int) bool {
			if album.Tracks[i].DiscNum != album.Tracks[j].DiscNum {
				return album.Tracks[i].DiscNum < album.Tracks[j].DiscNum
			}
			return album.Tracks[i].TrackNum < album.Tracks[j].TrackNum
		})
	}

	// Sort albums within each artist
	for _, artist := range app.Library.Artists {
		sort.Slice(artist.Albums, func(i, j int) bool {
			return strings.ToLower(artist.Albums[i].Name) < strings.ToLower(artist.Albums[j].Name)
		})
	}

	// Parse playlists
	for _, pl := range app.Library.Playlists {
		app.parsePlaylist(pl)
	}

	// Decode album art images
	decodeStart := time.Now()
	app.decodeAlbumArt()
	logMsg(fmt.Sprintf("decodeAlbumArt took: %v", time.Since(decodeStart)))

	logMsg(fmt.Sprintf("Library scan complete: %d tracks, %d albums, %d artists, %d playlists",
		len(app.Library.Tracks), len(app.Library.Albums), len(app.Library.Artists), len(app.Library.Playlists)))
	logMsg(fmt.Sprintf("ScanLibrary total took: %v", time.Since(start)))

	// Save library to JSON
	if err := app.saveLibraryJSON(); err != nil {
		logMsg(fmt.Sprintf("WARNING: Failed to save library: %v", err))
	}
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

		if pic := m.Picture(); pic != nil {
			track.HasArt = true
		}
	}

	// Extract duration using SDL_mixer
	track.Duration = audioGetDurationForFile(path)

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
	if track.HasArt && album.ArtData == nil {
		f.Seek(0, 0)
		if m2, err2 := tag.ReadFrom(f); err2 == nil {
			if pic := m2.Picture(); pic != nil {
				album.ArtData = pic.Data
				album.ArtExt = pic.Ext
				logMsg(fmt.Sprintf("Found album art: %s - %s (size: %d bytes, ext: %s)", album.Artist, album.Name, len(pic.Data), pic.Ext))
			} else {
				logMsg(fmt.Sprintf("Track has art flag but no picture data: %s", track.Path))
			}
		} else {
			logMsg(fmt.Sprintf("Failed to re-read tag for art extraction: %s - %v", track.Path, err2))
		}
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

// decodeAlbumArt decodes raw art bytes into images for all albums
func (app *MiyooPod) decodeAlbumArt() {
	start := time.Now()

	// Pre-cache all 3 sizes to eliminate runtime resize overhead
	sizes := []int{COVER_CENTER_SIZE, COVER_SIDE_SIZE, COVER_FAR_SIZE}

	for i, album := range app.Library.Albums {
		if album.ArtData == nil {
			continue
		}

		reader := bytes.NewReader(album.ArtData)
		img, _, err := image.Decode(reader)
		if err != nil {
			logMsg(fmt.Sprintf("Failed to decode art for %s: %v", album.Name, err))
			continue
		}

		album.ArtImg = img

		// Pre-cache ALL sizes (200px, 140px, 100px) to avoid resize during playback
		srcBounds := img.Bounds()
		for _, size := range sizes {
			key := fmt.Sprintf("%s|%s_%d", album.Artist, album.Name, size)
			if _, exists := app.Coverflow.CoverCache[key]; !exists {
				dc := gg.NewContext(size, size)
				sx := float64(size) / float64(srcBounds.Dx())
				sy := float64(size) / float64(srcBounds.Dy())
				dc.Scale(sx, sy)
				dc.DrawImage(img, 0, 0)
				app.Coverflow.CoverCache[key] = dc.Image()
			}
		}

		if i < 5 {
			logMsg(fmt.Sprintf("Decoded & cached album art %d/%d (all sizes)", i+1, len(app.Library.Albums)))
		}
	}
	logMsg(fmt.Sprintf("decodeAlbumArt loop took: %v", time.Since(start)))

	// Generate default album art and pre-cache at all sizes
	defaultSizes := []int{COVER_CENTER_SIZE, COVER_SIDE_SIZE, COVER_FAR_SIZE}
	for _, size := range defaultSizes {
		dc := gg.NewContext(size, size)
		dc.SetHexColor("#333333")
		dc.Clear()
		dc.SetHexColor("#666666")
		if app.FontSmall != nil {
			dc.SetFontFace(app.FontSmall)
			dc.DrawStringAnchored("No Art", float64(size)/2, float64(size)/2, 0.5, 0.5)
		}
		if size == COVER_CENTER_SIZE {
			app.DefaultArt = dc.Image()
		}
		// Pre-cache the default art at this size so getCachedCover never resizes it
		key := fmt.Sprintf("__default__%d", size)
		app.Coverflow.CoverCache[key] = dc.Image()
	}
}

// showScanningProgress displays scanning status on screen
func (app *MiyooPod) showScanningProgress(count int, currentPath string) {
	dc := app.DC

	dc.SetHexColor(app.CurrentTheme.BG)
	dc.Clear()

	dc.SetFontFace(app.FontTitle)
	dc.SetHexColor(app.CurrentTheme.HeaderTxt)
	dc.DrawStringAnchored("Scanning Library", SCREEN_WIDTH/2, SCREEN_HEIGHT/2-60, 0.5, 0.5)

	dc.SetFontFace(app.FontMenu)
	dc.SetHexColor(app.CurrentTheme.ItemTxt)
	dc.DrawStringAnchored(fmt.Sprintf("%d songs found", count), SCREEN_WIDTH/2, SCREEN_HEIGHT/2, 0.5, 0.5)

	dc.SetFontFace(app.FontSmall)
	dc.SetHexColor(app.CurrentTheme.Dim)

	// Truncate long paths to fit on screen
	displayPath := currentPath
	if len(displayPath) > 60 {
		displayPath = "..." + displayPath[len(displayPath)-57:]
	}
	dc.DrawStringAnchored(displayPath, SCREEN_WIDTH/2, SCREEN_HEIGHT/2+40, 0.5, 0.5)

	app.triggerRefresh()
}

// saveLibraryJSON writes the library to a JSON file
func (app *MiyooPod) saveLibraryJSON() error {
	if app.Library == nil {
		return fmt.Errorf("library is nil")
	}

	logMsg("Saving library to JSON...")
	start := time.Now()

	// Save current theme and lock key settings
	app.Library.SavedTheme = app.CurrentTheme.Name
	app.Library.SavedLockKey = app.getLockKeyName()

	data, err := json.MarshalIndent(app.Library, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal library: %v", err)
	}

	err = os.WriteFile(LIBRARY_JSON_PATH, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write library file: %v", err)
	}

	logMsg(fmt.Sprintf("Library saved to JSON in %v", time.Since(start)))
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

	// Re-extract album art for ALL albums (ArtData is not saved in JSON)
	for _, album := range lib.Albums {
		logMsg(fmt.Sprintf("Extracting art for album: %s - %s (%d tracks)", album.Artist, album.Name, len(album.Tracks)))
		// Try ALL tracks in this album to find art (don't rely on has_art flag)
		for i, track := range album.Tracks {
			if f, err := os.Open(track.Path); err == nil {
				if m, err := tag.ReadFrom(f); err == nil {
					if pic := m.Picture(); pic != nil {
						album.ArtData = pic.Data
						album.ArtExt = pic.Ext
						logMsg(fmt.Sprintf("  ✓ Found art in track %d: %s (size: %d bytes, ext: %s)", i+1, filepath.Base(track.Path), len(pic.Data), pic.Ext))
						// Update the track's has_art flag if we found art
						if !track.HasArt {
							track.HasArt = true
						}
						f.Close()
						break // Got art for this album, move to next album
					} else {
						logMsg(fmt.Sprintf("  - Track %d: %s - no picture data", i+1, filepath.Base(track.Path)))
					}
				} else {
					logMsg(fmt.Sprintf("  - Track %d: %s - tag read error: %v", i+1, filepath.Base(track.Path), err))
				}
				f.Close()
			} else {
				logMsg(fmt.Sprintf("  - Track %d: %s - file open error: %v", i+1, track.Path, err))
			}
		}
		if album.ArtData == nil {
			logMsg(fmt.Sprintf("  ✗ No art found for album: %s - %s", album.Artist, album.Name))
		}
	}

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

	// Restore saved theme if available
	if lib.SavedTheme != "" {
		for _, theme := range AllThemes() {
			if theme.Name == lib.SavedTheme {
				app.setTheme(theme)
				logMsg(fmt.Sprintf("Restored theme: %s", lib.SavedTheme))
				break
			}
		}
	}

	// Restore saved lock key if available
	if lib.SavedLockKey != "" {
		switch lib.SavedLockKey {
		case "Y":
			app.LockKey = Y
		case "X":
			app.LockKey = X
		case "SELECT":
			app.LockKey = SELECT
		case "MENU":
			app.LockKey = MENU
		case "L2":
			app.LockKey = L2
		case "R2":
			app.LockKey = R2
		}
		logMsg(fmt.Sprintf("Restored lock key: %s", lib.SavedLockKey))
	}

	logMsg(fmt.Sprintf("Library loaded from JSON: %d tracks, %d albums, %d artists, %d playlists in %v",
		len(lib.Tracks), len(lib.Albums), len(lib.Artists), len(lib.Playlists), time.Since(start)))

	return nil
}
