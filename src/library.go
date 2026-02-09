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
		logMsg(fmt.Sprintf("Failed to decode artwork: %v", err))
		return nil
	}

	return img
}

// decodeAlbumArt decodes raw art bytes into images for all albums
// NOW: Only decode on-demand to save memory and startup time
func (app *MiyooPod) decodeAlbumArt() {
	start := time.Now()

	// Pre-cache only the center size (280px) - the only size actually used
	// COVER_SIDE_SIZE and COVER_FAR_SIZE are unused (coverflow feature not implemented)
	sizes := []int{COVER_CENTER_SIZE}

	successCount := 0
	failCount := 0
	noArtCount := 0

	// OPTIMIZATION: Only decode artwork for albums that will be displayed immediately
	// to reduce startup time and memory usage. Other artwork will be decoded on-demand.
	// For now, just decode the first 20 albums to speed up initial menu display.
	maxPreCache := 20
	decodedCount := 0

	for i, album := range app.Library.Albums {
		// Skip pre-caching after first 20 albums to save memory
		if decodedCount >= maxPreCache {
			logMsg(fmt.Sprintf("[ART] Skipping pre-cache for remaining %d albums (will decode on-demand)",
				len(app.Library.Albums)-i))
			break
		}

		if album.ArtData == nil {
			// Try loading from disk if path is set
			if album.ArtPath != "" {
				if err := app.loadAlbumArtwork(album); err != nil {
					noArtCount++
					logMsg(fmt.Sprintf("[ART] Album %d/%d: %s - %s | Failed to load from disk: %v",
						i+1, len(app.Library.Albums), album.Artist, album.Name, err))
					continue
				}
			} else {
				noArtCount++
				logMsg(fmt.Sprintf("[ART] Album %d/%d: %s - %s | No art data available",
					i+1, len(app.Library.Albums), album.Artist, album.Name))
				continue
			}
		}

		logMsg(fmt.Sprintf("[ART] Album %d/%d: %s - %s | Art data: %d bytes, ext: %s",
			i+1, len(app.Library.Albums), album.Artist, album.Name, len(album.ArtData), album.ArtExt))

		decodeStart := time.Now()
		reader := bytes.NewReader(album.ArtData)
		img, format, err := image.Decode(reader)
		if err != nil {
			failCount++
			logMsg(fmt.Sprintf("[ART] ✗ FAILED to decode: %s - %s | Error: %v | Data size: %d bytes, ext: %s",
				album.Artist, album.Name, err, len(album.ArtData), album.ArtExt))
			// Log first few bytes to help diagnose format issues
			if len(album.ArtData) > 0 {
				previewLen := 16
				if len(album.ArtData) < previewLen {
					previewLen = len(album.ArtData)
				}
				logMsg(fmt.Sprintf("[ART]   First %d bytes: %v", previewLen, album.ArtData[:previewLen]))
			}
			continue
		}
		decodeTime := time.Since(decodeStart)

		successCount++
		album.ArtImg = img

		// Pre-cache ALL sizes (200px, 140px, 100px) to avoid resize during playback
		srcBounds := img.Bounds()
		logMsg(fmt.Sprintf("[ART] ✓ Decoded: %s - %s | Format: %s, Dimensions: %dx%d, Decode time: %v",
			album.Artist, album.Name, format, srcBounds.Dx(), srcBounds.Dy(), decodeTime))

		cacheStart := time.Now()
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
		cacheTime := time.Since(cacheStart)
		logMsg(fmt.Sprintf("[ART] ✓ Cached for: %s - %s | Cache time: %v",
			album.Artist, album.Name, cacheTime))

		// Free memory: clear both ArtData and ArtImg after caching
		// We only need the cached 280px version, not the full decoded image
		album.ArtData = nil
		album.ArtImg = nil
		decodedCount++
	}

	logMsg(fmt.Sprintf("[ART] Decode summary: %d succeeded, %d failed, %d no art, %d skipped | Total time: %v",
		successCount, failCount, noArtCount, len(app.Library.Albums)-decodedCount, time.Since(start)))

	// Generate default album art and pre-cache at center size only
	defaultSizes := []int{COVER_CENTER_SIZE}
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
		// First, try to load from saved artwork file
		if album.ArtPath != "" {
			if err := app.loadAlbumArtwork(album); err == nil {
				logMsg(fmt.Sprintf("Loaded saved artwork for: %s - %s (%s)", album.Artist, album.Name, album.ArtPath))
				continue // Successfully loaded from disk, skip extraction
			} else {
				logMsg(fmt.Sprintf("Failed to load saved artwork: %s - %s (%v)", album.Artist, album.Name, err))
				// Fall through to try extracting from MP3
			}
		}

		// Extract from MP3 files
		logMsg(fmt.Sprintf("Extracting art for album: %s - %s (%d tracks)", album.Artist, album.Name, len(album.Tracks)))
		// Try ALL tracks in this album to find art (don't rely on has_art flag)
		for i, track := range album.Tracks {
			if f, err := os.Open(track.Path); err == nil {
				if m, err := tag.ReadFrom(f); err == nil {
					if pic := m.Picture(); pic != nil {
						album.ArtData = pic.Data
						album.ArtExt = pic.Ext
						logMsg(fmt.Sprintf("  ✓ Found art in track %d: %s (size: %d bytes, ext: %s, type: %s)", i+1, filepath.Base(track.Path), len(pic.Data), pic.Ext, pic.MIMEType))

						// Save to disk to avoid re-extraction next time
						if err := app.saveAlbumArtwork(album); err != nil {
							logMsg(fmt.Sprintf("  Warning: Failed to save artwork to disk: %v", err))
						}

						// Update the track's has_art flag if we found art
						if !track.HasArt {
							track.HasArt = true
						}
						f.Close()
						break // Got art for this album, move to next album
					} else {
						logMsg(fmt.Sprintf("  - Track %d: %s - no picture data | Tag format: %T, FileType: %s", i+1, filepath.Base(track.Path), m, m.FileType()))
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

	logMsg(fmt.Sprintf("INFO: Library losaded from JSON: %d tracks, %d albums, %d artists, %d playlists in %v",
		len(lib.Tracks), len(lib.Albums), len(lib.Artists), len(lib.Playlists), time.Since(start)))

	return nil
}
