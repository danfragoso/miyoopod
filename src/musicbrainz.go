package main

import (
	"bytes"
	"crypto/md5"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/fogleman/gg"
)

// MusicBrainz API constants
const (
	MUSICBRAINZ_API_URL       = "https://musicbrainz.org/ws/2/"
	COVERART_ARCHIVE_URL      = "https://coverartarchive.org/release/%s/front"
	MUSICBRAINZ_USER_AGENT    = "MiyooPod/1.0.0 (https://github.com/danfragoso/miyoopod)"
	MUSICBRAINZ_RATE_LIMIT_MS = 1000 // 1 request per second as per MusicBrainz guidelines
)

var lastMusicBrainzRequest time.Time

// getInsecureHTTPClient returns an HTTP client that skips TLS verification
// This is necessary for embedded devices with incorrect system time (stuck at Unix epoch)
func getInsecureHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // Required due to system clock being set to 1970
			},
		},
	}
}

// MusicBrainzRelease represents a simplified MusicBrainz release response
type MusicBrainzRelease struct {
	ID string `json:"id"`
}

type MusicBrainzReleaseSearch struct {
	Releases []MusicBrainzRelease `json:"releases"`
}

// fetchAlbumArtFromMusicBrainz attempts to fetch album artwork from MusicBrainz/Cover Art Archive
func (app *MiyooPod) fetchAlbumArtFromMusicBrainz(album *Album) bool {
	if album == nil || album.Name == "" {
		return false
	}

	logMsg(fmt.Sprintf("[MUSICBRAINZ] Fetching art for: %s - %s", album.Artist, album.Name))

	// Step 1: Search for releases
	releaseIDs, err := searchMusicBrainzRelease(album.Artist, album.Name)
	if err != nil {
		logMsg(fmt.Sprintf("WARNING: [MUSICBRAINZ] Search failed: %v", err))
		return false
	}

	if len(releaseIDs) == 0 {
		logMsg(fmt.Sprintf("[MUSICBRAINZ] No release found for: %s - %s", album.Artist, album.Name))
		return false
	}

	logMsg(fmt.Sprintf("[MUSICBRAINZ] Found %d release(s), trying each for cover art...", len(releaseIDs)))

	// Update UI status
	if app.albumArtStatusFunc != nil {
		app.albumArtStatusFunc(fmt.Sprintf("Searching MusicBrainz...\nFound %d release(s)", len(releaseIDs)))
	}

	// Step 2: Try each release until we find cover art
	for i, releaseID := range releaseIDs {
		logMsg(fmt.Sprintf("[MUSICBRAINZ] Trying release %d/%d: %s", i+1, len(releaseIDs), releaseID))

		// Update UI status
		if app.albumArtStatusFunc != nil {
			app.albumArtStatusFunc(fmt.Sprintf("Trying release %d/%d...", i+1, len(releaseIDs)))
		}

		artData, mimeType, err := fetchCoverArt(releaseID)
		if err != nil {
			logMsg(fmt.Sprintf("[MUSICBRAINZ] Release %d failed: %v", i+1, err))

			// Update UI status
			if app.albumArtStatusFunc != nil {
				app.albumArtStatusFunc(fmt.Sprintf("Release %d/%d failed\nTrying next...", i+1, len(releaseIDs)))
			}

			continue // Try next release
		}

		if len(artData) == 0 {
			logMsg(fmt.Sprintf("[MUSICBRAINZ] Release %d has no cover art", i+1))

			// Update UI status
			if app.albumArtStatusFunc != nil {
				app.albumArtStatusFunc(fmt.Sprintf("Release %d/%d no art\nTrying next...", i+1, len(releaseIDs)))
			}

			continue // Try next release
		}

		// Found art! Downscale and save
		// Update UI status
		if app.albumArtStatusFunc != nil {
			app.albumArtStatusFunc(fmt.Sprintf("Downloading from release %d/%d...", i+1, len(releaseIDs)))
		}

		artData, mimeType, err = downscaleImage(artData, mimeType, 200)
		if err != nil {
			logMsg(fmt.Sprintf("WARNING: [MUSICBRAINZ] Failed to process image: %v", err))
			continue // Try next release
		}

		// Determine file extension from MIME type
		ext := ""
		switch mimeType {
		case "image/jpeg", "image/jpg":
			ext = "jpg"
		case "image/png":
			ext = "png"
		}

		album.ArtData = artData
		album.ArtExt = ext

		// Save artwork to disk to persist across restarts and save RAM
		if err := app.saveAlbumArtwork(album); err != nil {
			logMsg(fmt.Sprintf("WARNING: [MUSICBRAINZ] Failed to save artwork to disk: %v", err))
			// Continue anyway, art is still in memory
		}

		logMsg(fmt.Sprintf("[MUSICBRAINZ] ✓ Successfully fetched art from release %d: %d bytes, type: %s", i+1, len(artData), mimeType))

		// Update UI status
		if app.albumArtStatusFunc != nil {
			app.albumArtStatusFunc(fmt.Sprintf("✓ Success!\nFetched from release %d/%d", i+1, len(releaseIDs)))
		}

		return true
	}

	// No releases had cover art
	logMsg(fmt.Sprintf("[MUSICBRAINZ] No cover art found in any of the %d releases", len(releaseIDs)))

	// Update UI status
	if app.albumArtStatusFunc != nil {
		app.albumArtStatusFunc(fmt.Sprintf("✗ Failed\nNo art found in %d releases", len(releaseIDs)))
	}

	return false
}

// downscaleImage downscales an image if it's larger than maxSize
func downscaleImage(artData []byte, mimeType string, maxSize int) ([]byte, string, error) {
	// Decode the image
	reader := bytes.NewReader(artData)
	img, format, err := image.Decode(reader)
	if err != nil {
		return artData, mimeType, err // Return original if decode fails
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// If image is already small enough, return original
	if width <= maxSize && height <= maxSize {
		logMsg(fmt.Sprintf("[MUSICBRAINZ] Image size %dx%d is OK, no downscaling needed", width, height))
		return artData, mimeType, nil
	}

	logMsg(fmt.Sprintf("[MUSICBRAINZ] Downscaling image from %dx%d to fit %dx%d", width, height, maxSize, maxSize))

	// Calculate new dimensions maintaining aspect ratio
	newWidth, newHeight := width, height
	if width > height {
		newWidth = maxSize
		newHeight = (height * maxSize) / width
	} else {
		newHeight = maxSize
		newWidth = (width * maxSize) / height
	}

	// Resize using gg
	dc := gg.NewContext(newWidth, newHeight)
	sx := float64(newWidth) / float64(width)
	sy := float64(newHeight) / float64(height)
	dc.Scale(sx, sy)
	dc.DrawImage(img, 0, 0)

	// Encode to bytes
	var buf bytes.Buffer
	if format == "png" || mimeType == "image/png" {
		err = png.Encode(&buf, dc.Image())
		mimeType = "image/png"
	} else {
		err = jpeg.Encode(&buf, dc.Image(), &jpeg.Options{Quality: 90})
		mimeType = "image/jpeg"
	}

	if err != nil {
		return artData, mimeType, err
	}

	logMsg(fmt.Sprintf("[MUSICBRAINZ] Downscaled to %dx%d, size: %d bytes (was %d bytes)",
		newWidth, newHeight, buf.Len(), len(artData)))

	return buf.Bytes(), mimeType, nil
}

// searchMusicBrainzRelease searches for a release and returns up to 5 release IDs
func searchMusicBrainzRelease(artist, album string) ([]string, error) {
	// Rate limiting
	rateLimitMusicBrainz()

	// Build search query
	query := fmt.Sprintf("artist:%s AND release:%s",
		sanitizeSearchTerm(artist),
		sanitizeSearchTerm(album))

	searchURL := fmt.Sprintf("%srelease/?query=%s&fmt=json&limit=5",
		MUSICBRAINZ_API_URL,
		url.QueryEscape(query))

	// Create request with user agent (required by MusicBrainz)
	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", MUSICBRAINZ_USER_AGENT)

	client := getInsecureHTTPClient(10 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MusicBrainz API returned status: %d", resp.StatusCode)
	}

	var result MusicBrainzReleaseSearch
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Releases) == 0 {
		return []string{}, nil
	}

	// Extract all release IDs
	releaseIDs := make([]string, len(result.Releases))
	for i, release := range result.Releases {
		releaseIDs[i] = release.ID
	}

	return releaseIDs, nil
}

// fetchCoverArt fetches cover art from Cover Art Archive
func fetchCoverArt(releaseID string) ([]byte, string, error) {
	// Rate limiting
	rateLimitMusicBrainz()

	coverURL := fmt.Sprintf(COVERART_ARCHIVE_URL, releaseID)

	req, err := http.NewRequest("GET", coverURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", MUSICBRAINZ_USER_AGENT)

	client := getInsecureHTTPClient(10 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, "", nil // No cover art available
	}

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("Cover Art Archive returned status: %d", resp.StatusCode)
	}

	mimeType := resp.Header.Get("Content-Type")
	artData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	return artData, mimeType, nil
}

// rateLimitMusicBrainz ensures we don't exceed MusicBrainz rate limits
func rateLimitMusicBrainz() {
	now := time.Now()
	elapsed := now.Sub(lastMusicBrainzRequest)
	if elapsed < time.Duration(MUSICBRAINZ_RATE_LIMIT_MS)*time.Millisecond {
		time.Sleep(time.Duration(MUSICBRAINZ_RATE_LIMIT_MS)*time.Millisecond - elapsed)
	}
	lastMusicBrainzRequest = time.Now()
}

// sanitizeSearchTerm cleans up search terms for MusicBrainz queries
func sanitizeSearchTerm(term string) string {
	// Remove special characters that might confuse the search
	term = strings.TrimSpace(term)
	// Escape quotes
	term = strings.ReplaceAll(term, `"`, `\"`)
	// Quote the term if it contains spaces
	if strings.Contains(term, " ") {
		term = `"` + term + `"`
	}
	return term
}

// generateAlbumCacheKey generates a cache key for an album
func generateAlbumCacheKey(artist, album string) string {
	key := fmt.Sprintf("%s|%s", strings.ToLower(artist), strings.ToLower(album))
	hash := md5.Sum([]byte(key))
	return fmt.Sprintf("%x", hash)
}

// saveAlbumArtwork saves album artwork to disk and clears ArtData from memory
func (app *MiyooPod) saveAlbumArtwork(album *Album) error {
	if album == nil || album.ArtData == nil || len(album.ArtData) == 0 {
		return fmt.Errorf("no artwork data to save")
	}

	// Create artwork directory if it doesn't exist
	if err := os.MkdirAll(ARTWORK_DIR, 0755); err != nil {
		return fmt.Errorf("failed to create artwork directory: %v", err)
	}

	// Generate filename using hash of artist+album
	hash := generateAlbumCacheKey(album.Artist, album.Name)
	filename := fmt.Sprintf("%s.%s", hash, album.ArtExt)
	filepath := ARTWORK_DIR + filename

	// Write artwork to disk
	if err := os.WriteFile(filepath, album.ArtData, 0644); err != nil {
		return fmt.Errorf("failed to write artwork file: %v", err)
	}

	// Store path and clear memory
	album.ArtPath = filepath
	album.ArtData = nil // Free memory!

	logMsg(fmt.Sprintf("[ARTWORK] Saved to disk: %s", filepath))
	return nil
}

// loadAlbumArtwork loads album artwork from disk if available
func (app *MiyooPod) loadAlbumArtwork(album *Album) error {
	if album == nil || album.ArtPath == "" {
		return fmt.Errorf("no artwork path set")
	}

	// Check if file exists
	if _, err := os.Stat(album.ArtPath); os.IsNotExist(err) {
		return fmt.Errorf("artwork file not found: %s", album.ArtPath)
	}

	// Read artwork from disk
	artData, err := os.ReadFile(album.ArtPath)
	if err != nil {
		return fmt.Errorf("failed to read artwork file: %v", err)
	}

	// Determine extension from path
	if strings.HasSuffix(album.ArtPath, ".jpg") || strings.HasSuffix(album.ArtPath, ".jpeg") {
		album.ArtExt = "jpg"
	} else if strings.HasSuffix(album.ArtPath, ".png") {
		album.ArtExt = "png"
	}

	album.ArtData = artData
	return nil
}
