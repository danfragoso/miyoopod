package main

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// scanAlbumArt starts fetching missing album artwork in a background goroutine.
// Switches to ScreenAlbumArt so the main loop handles keys normally.
func (app *MiyooPod) scanAlbumArt() {
	if app.AlbumArtFetching {
		return
	}

	// Count albums without art
	missingCount := 0
	for _, album := range app.Library.Albums {
		if album.ArtData == nil && album.ArtPath == "" && album.Name != "" && album.Artist != "" {
			missingCount++
		}
	}

	if missingCount == 0 {
		app.AlbumArtFetching = false
		app.AlbumArtDone = true
		app.AlbumArtElapsed = ""
		app.AlbumArtFetched = 0
		app.AlbumArtFailed = 0
		app.AlbumArtTotal = 0
		app.AlbumArtStatus = "All albums have artwork!"
		app.AlbumArtAlbumName = ""
		app.AlbumArtArtist = ""
		app.setScreen(ScreenAlbumArt)
		app.drawCurrentScreen()
		return
	}

	// Initialize state
	app.AlbumArtFetching = true
	app.AlbumArtDone = false
	app.AlbumArtCurrent = 0
	app.AlbumArtTotal = missingCount
	app.AlbumArtFetched = 0
	app.AlbumArtFailed = 0
	app.AlbumArtAlbumName = ""
	app.AlbumArtArtist = ""
	app.AlbumArtStatus = "Starting..."
	app.AlbumArtElapsed = ""

	app.setScreen(ScreenAlbumArt)
	app.drawCurrentScreen()

	go app.runAlbumArtFetch()
}

// runAlbumArtFetch is the background goroutine that fetches album art.
// IMPORTANT: Never call draw functions from here — only update state and requestRedraw.
func (app *MiyooPod) runAlbumArtFetch() {
	start := time.Now()

	for _, album := range app.Library.Albums {
		if !app.Running || !app.AlbumArtFetching {
			break
		}

		if album.ArtData == nil && album.ArtPath == "" && album.Name != "" && album.Artist != "" {
			app.AlbumArtCurrent++
			app.AlbumArtAlbumName = album.Name
			app.AlbumArtArtist = album.Artist
			app.AlbumArtStatus = "Searching MusicBrainz..."
			app.requestRedraw()

			// Status callback updates state and signals redraw (no draw calls)
			app.albumArtStatusFunc = func(status string) {
				app.AlbumArtStatus = status
				app.requestRedraw()
			}

			if app.fetchAlbumArtFromMusicBrainz(album) {
				app.AlbumArtFetched++
			} else {
				app.AlbumArtFailed++
			}

			app.albumArtStatusFunc = nil
		}
	}

	// Decode newly fetched artwork
	if app.AlbumArtFetched > 0 {
		app.AlbumArtStatus = "Decoding artwork..."
		app.AlbumArtAlbumName = ""
		app.AlbumArtArtist = ""
		app.requestRedraw()

		app.decodeAlbumArt()

		if err := app.saveLibraryJSON(); err != nil {
			logMsg(fmt.Sprintf("WARNING: Failed to save library: %v", err))
		}
	}

	elapsed := time.Since(start)
	app.AlbumArtElapsed = elapsed.Truncate(time.Second).String()
	app.AlbumArtFetching = false
	app.AlbumArtDone = true
	app.requestRedraw()
}

// drawAlbumArtScreen renders the album art fetch screen using list-style layout.
// Called from drawCurrentScreen on the main thread only.
func (app *MiyooPod) drawAlbumArtScreen() {
	dc := app.DC

	dc.SetHexColor(app.CurrentTheme.BG)
	dc.Clear()

	app.drawHeader("Fetch Album Art")

	if app.AlbumArtDone {
		app.drawAlbumArtResults()
		return
	}

	// List-style progress display (like queue/menu screens)
	y := MENU_TOP_Y

	// Row 1: Progress count
	dc.SetFontFace(app.FontMenu)
	dc.SetHexColor(app.CurrentTheme.ItemTxt)
	textY := float64(y) + float64(MENU_ITEM_HEIGHT)/2
	dc.DrawStringAnchored(
		fmt.Sprintf("Scanning %d of %d albums", app.AlbumArtCurrent, app.AlbumArtTotal),
		float64(MENU_LEFT_PAD), textY, 0, 0.5,
	)

	// Progress fraction on right
	if app.AlbumArtTotal > 0 {
		dc.SetFontFace(app.FontSmall)
		dc.SetHexColor(app.CurrentTheme.Dim)
		pct := int(100 * float64(app.AlbumArtCurrent) / float64(app.AlbumArtTotal))
		dc.DrawStringAnchored(fmt.Sprintf("%d%%", pct), float64(SCREEN_WIDTH-MENU_RIGHT_PAD), textY, 1, 0.5)
	}
	y += MENU_ITEM_HEIGHT

	// Progress bar (full width, thin)
	barX := MENU_LEFT_PAD
	barW := SCREEN_WIDTH - MENU_LEFT_PAD - MENU_RIGHT_PAD
	barH := 6

	dc.SetHexColor(app.CurrentTheme.ProgBG)
	dc.DrawRoundedRectangle(float64(barX), float64(y), float64(barW), float64(barH), 3)
	dc.Fill()

	if app.AlbumArtTotal > 0 {
		fillW := int(float64(barW) * float64(app.AlbumArtCurrent) / float64(app.AlbumArtTotal))
		if fillW > 0 {
			dc.SetHexColor(app.CurrentTheme.Accent)
			dc.DrawRoundedRectangle(float64(barX), float64(y), float64(fillW), float64(barH), 3)
			dc.Fill()
		}
	}
	y += barH + 12

	// Row 2: Current album (highlighted like a selected menu item)
	if app.AlbumArtAlbumName != "" {
		dc.SetHexColor(app.CurrentTheme.SelBG)
		dc.DrawRectangle(0, float64(y), SCREEN_WIDTH, MENU_ITEM_HEIGHT)
		dc.Fill()

		dc.SetFontFace(app.FontMenu)
		dc.SetHexColor(app.CurrentTheme.SelTxt)
		albumLabel := app.truncateText(app.AlbumArtAlbumName, float64(SCREEN_WIDTH-MENU_LEFT_PAD-MENU_RIGHT_PAD), app.FontMenu)
		textY = float64(y) + float64(MENU_ITEM_HEIGHT)/2
		dc.DrawStringAnchored(albumLabel, float64(MENU_LEFT_PAD), textY, 0, 0.5)

		// Artist on right
		dc.SetFontFace(app.FontSmall)
		dc.SetHexColor(app.CurrentTheme.SelTxt)
		artistLabel := app.truncateText(app.AlbumArtArtist, 200, app.FontSmall)
		dc.DrawStringAnchored(artistLabel, float64(SCREEN_WIDTH-MENU_RIGHT_PAD), textY, 1, 0.5)
		y += MENU_ITEM_HEIGHT
	}

	// Row 3: Status message
	if app.AlbumArtStatus != "" {
		dc.SetFontFace(app.FontSmall)
		dc.SetHexColor(app.CurrentTheme.Dim)
		textY = float64(y) + float64(MENU_ITEM_HEIGHT)/2
		statusText := app.truncateText(app.AlbumArtStatus, float64(SCREEN_WIDTH-MENU_LEFT_PAD-MENU_RIGHT_PAD), app.FontSmall)
		dc.DrawStringAnchored(statusText, float64(MENU_LEFT_PAD), textY, 0, 0.5)
		y += MENU_ITEM_HEIGHT
	}

	// Row 4+: Running totals
	if app.AlbumArtFetched > 0 || app.AlbumArtFailed > 0 {
		dc.SetFontFace(app.FontSmall)
		textY = float64(y) + float64(MENU_ITEM_HEIGHT)/2

		dc.SetHexColor(app.CurrentTheme.Accent)
		dc.DrawStringAnchored(fmt.Sprintf("Fetched: %d", app.AlbumArtFetched), float64(MENU_LEFT_PAD), textY, 0, 0.5)

		if app.AlbumArtFailed > 0 {
			dc.SetHexColor(app.CurrentTheme.Dim)
			dc.DrawStringAnchored(fmt.Sprintf("Failed: %d", app.AlbumArtFailed), float64(MENU_LEFT_PAD+150), textY, 0, 0.5)
		}
	}
}

// drawAlbumArtResults renders the results after album art scanning is done.
func (app *MiyooPod) drawAlbumArtResults() {
	dc := app.DC
	y := MENU_TOP_Y

	// Row 1: Elapsed time
	dc.SetFontFace(app.FontMenu)
	dc.SetHexColor(app.CurrentTheme.ItemTxt)
	textY := float64(y) + float64(MENU_ITEM_HEIGHT)/2
	title := "Scan complete"
	if app.AlbumArtElapsed != "" {
		title = fmt.Sprintf("Scan complete in %s", app.AlbumArtElapsed)
	}
	if app.AlbumArtTotal == 0 {
		title = app.AlbumArtStatus // "All albums have artwork!"
	}
	dc.DrawStringAnchored(title, float64(MENU_LEFT_PAD), textY, 0, 0.5)
	y += MENU_ITEM_HEIGHT

	// Separator line
	dc.SetHexColor(app.CurrentTheme.ProgBG)
	dc.DrawRectangle(float64(MENU_LEFT_PAD), float64(y), float64(SCREEN_WIDTH-MENU_LEFT_PAD-MENU_RIGHT_PAD), 1)
	dc.Fill()
	y += 8

	// Results rows
	if app.AlbumArtTotal > 0 {
		// Fetched count
		dc.SetFontFace(app.FontMenu)
		dc.SetHexColor(app.CurrentTheme.ItemTxt)
		textY = float64(y) + float64(MENU_ITEM_HEIGHT)/2
		dc.DrawStringAnchored("Fetched", float64(MENU_LEFT_PAD), textY, 0, 0.5)
		dc.SetHexColor(app.CurrentTheme.Accent)
		dc.DrawStringAnchored(fmt.Sprintf("%d", app.AlbumArtFetched), float64(SCREEN_WIDTH-MENU_RIGHT_PAD), textY, 1, 0.5)
		y += MENU_ITEM_HEIGHT

		// Failed count
		if app.AlbumArtFailed > 0 {
			dc.SetFontFace(app.FontMenu)
			dc.SetHexColor(app.CurrentTheme.ItemTxt)
			textY = float64(y) + float64(MENU_ITEM_HEIGHT)/2
			dc.DrawStringAnchored("Failed", float64(MENU_LEFT_PAD), textY, 0, 0.5)
			dc.SetHexColor(app.CurrentTheme.Dim)
			dc.DrawStringAnchored(fmt.Sprintf("%d", app.AlbumArtFailed), float64(SCREEN_WIDTH-MENU_RIGHT_PAD), textY, 1, 0.5)
			y += MENU_ITEM_HEIGHT
		}

		// Still missing
		stillMissing := app.AlbumArtTotal - app.AlbumArtFetched
		if stillMissing > 0 {
			dc.SetFontFace(app.FontMenu)
			dc.SetHexColor(app.CurrentTheme.ItemTxt)
			textY = float64(y) + float64(MENU_ITEM_HEIGHT)/2
			dc.DrawStringAnchored("Still missing", float64(MENU_LEFT_PAD), textY, 0, 0.5)
			dc.SetHexColor(app.CurrentTheme.Dim)
			dc.DrawStringAnchored(fmt.Sprintf("%d", stillMissing), float64(SCREEN_WIDTH-MENU_RIGHT_PAD), textY, 1, 0.5)
		}
	}
}

// drawAlbumArtStatusBar renders the status bar for the album art screen.
func (app *MiyooPod) drawAlbumArtStatusBar() {
	dc := app.DC

	barY := float64(SCREEN_HEIGHT - STATUS_BAR_HEIGHT)

	// Background
	dc.SetHexColor(app.CurrentTheme.HeaderBG)
	dc.DrawRectangle(0, barY, SCREEN_WIDTH, STATUS_BAR_HEIGHT)
	dc.Fill()

	centerY := barY + float64(STATUS_BAR_HEIGHT)/2

	if app.AlbumArtDone {
		app.drawButtonLegend(12, centerY, "B", "Back")
		stillMissing := app.AlbumArtTotal - app.AlbumArtFetched
		if stillMissing > 0 {
			app.drawButtonLegend(100, centerY, "A", "Retry")
		}
	} else {
		app.drawButtonLegend(12, centerY, "B", "Cancel")
	}
}

// handleAlbumArtKey handles key input on the album art screen.
func (app *MiyooPod) handleAlbumArtKey(key Key) {
	switch key {
	case B, MENU:
		app.AlbumArtFetching = false // Signal goroutine to stop
		app.AlbumArtDone = false
		app.setScreen(ScreenMenu)
		app.drawCurrentScreen()
	case A:
		if app.AlbumArtDone {
			stillMissing := app.AlbumArtTotal - app.AlbumArtFetched
			if stillMissing > 0 {
				app.AlbumArtDone = false
				app.scanAlbumArt()
			}
		}
	}
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
			continue
		}
		width += adv
		prevRune = r
	}
	return float64(width) / 64.0
}
