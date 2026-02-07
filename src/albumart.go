package main

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// scanAlbumArt fetches missing album artwork from MusicBrainz with progress UI
func (app *MiyooPod) scanAlbumArt() {
	dc := app.DC

	// Count albums without art
	missingCount := 0
	for _, album := range app.Library.Albums {
		if album.ArtData == nil && album.ArtPath == "" {
			missingCount++
		}
	}

	if missingCount == 0 {
		app.showAlbumArtResult("All albums have artwork!!", 0, 0, 0)
		return
	}

	start := time.Now()
	fetchedCount := 0
	failedCount := 0
	currentAlbum := 0

	for _, album := range app.Library.Albums {
		if album.ArtData == nil && album.ArtPath == "" && album.Name != "" && album.Artist != "" {
			currentAlbum++

			// Fetch artwork with UI updates
			if app.fetchAlbumArtFromMusicBrainzWithUI(album, currentAlbum, missingCount) {
				fetchedCount++
			} else {
				failedCount++
			}
		}
	}

	// Decode newly fetched artwork
	if fetchedCount > 0 {
		app.showAlbumArtProgress(nil, missingCount, missingCount, "Decoding artwork...") // Show "Decoding..."
		dc.SetHexColor(app.CurrentTheme.BG)
		dc.Clear()
		dc.SetFontFace(app.FontMenu)
		dc.SetHexColor(app.CurrentTheme.ItemTxt)
		dc.DrawStringAnchored("Decoding album artwork...", SCREEN_WIDTH/2, SCREEN_HEIGHT/2, 0.5, 0.5)
		app.triggerRefresh()

		app.decodeAlbumArt()

		// Save library with new artwork
		if err := app.saveLibraryJSON(); err != nil {
			logMsg(fmt.Sprintf("WARNING: Failed to save library: %v", err))
		}
	}

	// Show results
	elapsed := time.Since(start)
	app.showAlbumArtResult(fmt.Sprintf("Scan complete in %v", elapsed), fetchedCount, failedCount, missingCount)
}

// showAlbumArtProgress displays progress during album art scanning
func (app *MiyooPod) showAlbumArtProgress(album *Album, current, total int, status string) {
	dc := app.DC

	dc.SetHexColor(app.CurrentTheme.BG)
	dc.Clear()

	// Header
	app.drawHeader("Scan Album Art")

	// Progress text
	dc.SetFontFace(app.FontMenu)
	dc.SetHexColor(app.CurrentTheme.ItemTxt)
	progressText := fmt.Sprintf("Scanning %d of %d albums", current, total)
	dc.DrawStringAnchored(progressText, SCREEN_WIDTH/2, SCREEN_HEIGHT/2-60, 0.5, 0.5)

	// Progress bar
	barWidth := 400
	barHeight := 30
	barX := (SCREEN_WIDTH - barWidth) / 2
	barY := SCREEN_HEIGHT/2 - 20

	// Background
	dc.SetHexColor(app.CurrentTheme.Dim)
	dc.DrawRectangle(float64(barX), float64(barY), float64(barWidth), float64(barHeight))
	dc.Fill()

	// Progress fill
	if total > 0 {
		fillWidth := int(float64(barWidth) * float64(current) / float64(total))
		dc.SetHexColor(app.CurrentTheme.SelBG)
		dc.DrawRectangle(float64(barX), float64(barY), float64(fillWidth), float64(barHeight))
		dc.Fill()
	}

	// Border
	dc.SetHexColor(app.CurrentTheme.ItemTxt)
	dc.SetLineWidth(2)
	dc.DrawRectangle(float64(barX), float64(barY), float64(barWidth), float64(barHeight))
	dc.Stroke()

	// Current album info
	if album != nil {
		dc.SetFontFace(app.FontSmall)
		dc.SetHexColor(app.CurrentTheme.ItemTxt)
		albumInfo := app.truncateText(fmt.Sprintf("%s - %s", album.Artist, album.Name),
			float64(SCREEN_WIDTH-40), app.FontSmall)
		dc.DrawStringAnchored(albumInfo, SCREEN_WIDTH/2, SCREEN_HEIGHT/2+40, 0.5, 0.5)
	}

	// Status message
	if status != "" {
		dc.SetFontFace(app.FontSmall)
		dc.SetHexColor(app.CurrentTheme.Dim)
		// Wrap long status messages
		lines := wrapText(status, float64(SCREEN_WIDTH-40), app.FontSmall)
		yPos := SCREEN_HEIGHT/2 + 65
		for i, line := range lines {
			if i >= 3 { // Max 3 lines
				break
			}
			dc.DrawStringAnchored(line, SCREEN_WIDTH/2, float64(yPos+(i*18)), 0.5, 0.5)
		}
	}

	app.triggerRefresh()
}

// wrapText breaks text into lines that fit within maxWidth
func wrapText(text string, maxWidth float64, face font.Face) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{}
	}

	var lines []string
	currentLine := ""

	for _, word := range words {
		test := currentLine
		if test != "" {
			test += " "
		}
		test += word

		width := measureString(test, face)
		if width <= maxWidth {
			currentLine = test
		} else {
			if currentLine != "" {
				lines = append(lines, currentLine)
			}
			currentLine = word
		}
	}

	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	return lines
}

// measureString measures the width of a string with the given font
func measureString(s string, face font.Face) float64 {
	width := fixed.Int26_6(0)
	prevRune := rune(-1)
	for _, r := range s {
		if prevRune >= 0 {
			width += face.Kern(prevRune, r)
		}
		adv, ok := face.GlyphAdvance(r)
		if !ok {
			// Fallback for missing glyphs
			continue
		}
		width += adv
		prevRune = r
	}
	return float64(width) / 64.0 // Convert from fixed.Int26_6 to float64
}

// fetchAlbumArtFromMusicBrainzWithUI wraps the MusicBrainz fetch with UI status updates
func (app *MiyooPod) fetchAlbumArtFromMusicBrainzWithUI(album *Album, current, total int) bool {
	// Set up status callback
	app.albumArtStatusFunc = func(status string) {
		app.showAlbumArtProgress(album, current, total, status)
	}
	defer func() {
		app.albumArtStatusFunc = nil
	}()

	// Call the actual MusicBrainz fetch
	return app.fetchAlbumArtFromMusicBrainz(album)
}

// showAlbumArtResult displays the final results of album art scanning
func (app *MiyooPod) showAlbumArtResult(title string, fetched, failed, total int) {
	dc := app.DC

	dc.SetHexColor(app.CurrentTheme.BG)
	dc.Clear()

	// Header
	app.drawHeader("Scan Album Art")

	yPos := SCREEN_HEIGHT/2 - 80

	// Title
	dc.SetFontFace(app.FontMenu)
	dc.SetHexColor(app.CurrentTheme.ItemTxt)
	dc.DrawStringAnchored(title, SCREEN_WIDTH/2, float64(yPos), 0.5, 0.5)
	yPos += 60

	// Results
	dc.SetFontFace(app.FontArtist)
	dc.SetHexColor(app.CurrentTheme.ItemTxt)

	if total > 0 {
		dc.DrawStringAnchored(fmt.Sprintf("✓ Fetched: %d", fetched), SCREEN_WIDTH/2, float64(yPos), 0.5, 0.5)
		yPos += 30

		if failed > 0 {
			dc.SetHexColor(app.CurrentTheme.Dim)
			dc.DrawStringAnchored(fmt.Sprintf("✗ Failed: %d", failed), SCREEN_WIDTH/2, float64(yPos), 0.5, 0.5)
			yPos += 30
		}

		stillMissing := total - fetched
		if stillMissing > 0 {
			dc.SetHexColor(app.CurrentTheme.Dim)
			dc.DrawStringAnchored(fmt.Sprintf("Still missing: %d", stillMissing), SCREEN_WIDTH/2, float64(yPos), 0.5, 0.5)
		}
	}

	// Instructions
	dc.SetFontFace(app.FontSmall)
	dc.SetHexColor(app.CurrentTheme.Dim)

	stillMissing := total - fetched
	if stillMissing > 0 {
		dc.DrawStringAnchored("Press A to retry • Press B to return", SCREEN_WIDTH/2, SCREEN_HEIGHT-40, 0.5, 0.5)
	} else {
		dc.DrawStringAnchored("Press B to return", SCREEN_WIDTH/2, SCREEN_HEIGHT-40, 0.5, 0.5)
	}

	app.triggerRefresh()

	// Wait for button press
	app.waitForAlbumArtExit(stillMissing)
}

// waitForAlbumArtExit waits for user to press B to exit album art results
func (app *MiyooPod) waitForAlbumArtExit(stillMissing int) {
	for app.Running {
		key := Key(C_GetKeyPress())
		if key == NONE {
			time.Sleep(33 * time.Millisecond)
			continue
		}

		if key == B || key == MENU {
			// Return to menu
			app.CurrentScreen = ScreenMenu
			app.drawMenuScreen()
			return
		}

		if key == A && stillMissing > 0 {
			// Retry scanning
			app.scanAlbumArt()
			return
		}
	}
}
