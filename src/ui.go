package main

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/image/font"
)

// drawHeader draws the top header bar with a title and optional play state indicator
func (app *MiyooPod) drawHeader(title string) {
	dc := app.DC

	// Header background
	dc.SetHexColor(app.CurrentTheme.HeaderBG)
	dc.DrawRectangle(0, 0, SCREEN_WIDTH, HEADER_HEIGHT)
	dc.Fill()

	// Battery status in top-left
	batteryPercent := getBatteryLevel()
	if batteryPercent >= 0 {
		// Draw battery icon
		app.drawBatteryIcon(12, 14, batteryPercent)
		// Draw percentage right after icon
		dc.SetFontFace(app.FontSmall)
		dc.SetHexColor(app.CurrentTheme.HeaderTxt)
		dc.DrawStringAnchored(fmt.Sprintf("%d%%", batteryPercent), 40, HEADER_HEIGHT/2, 0, 0.5)
	}

	// Header title
	dc.SetFontFace(app.FontHeader)
	dc.SetHexColor(app.CurrentTheme.HeaderTxt)
	dc.DrawStringAnchored(title, SCREEN_WIDTH/2, HEADER_HEIGHT/2, 0.5, 0.5)

	// Play state indicator in top-right
	if app.Playing != nil && app.Playing.State == StatePlaying {
		dc.SetHexColor(app.CurrentTheme.Accent)
		app.drawPlayIcon(SCREEN_WIDTH-30, 10, 20)
	} else if app.Playing != nil && app.Playing.State == StatePaused {
		dc.SetHexColor(app.CurrentTheme.Dim)
		app.drawPauseIcon(SCREEN_WIDTH-30, 10, 20)
	}
}

// drawMenuItem draws a single menu row
func (app *MiyooPod) drawMenuItem(y int, label string, selected bool, hasSubmenu bool, isPlaying bool, isInQueue bool) {
	dc := app.DC

	// Selection highlight
	if selected {
		dc.SetHexColor(app.CurrentTheme.SelBG)
		dc.DrawRectangle(0, float64(y), SCREEN_WIDTH, MENU_ITEM_HEIGHT)
		dc.Fill()
	}

	// Item text
	dc.SetFontFace(app.FontMenu)
	if selected {
		dc.SetHexColor(app.CurrentTheme.SelTxt)
	} else {
		dc.SetHexColor(app.CurrentTheme.ItemTxt)
	}

	// Truncate text if needed
	maxWidth := float64(SCREEN_WIDTH - MENU_LEFT_PAD - MENU_RIGHT_PAD - 15)
	displayLabel := app.truncateText(label, maxWidth, app.FontMenu)

	textY := float64(y) + float64(MENU_ITEM_HEIGHT)/2
	dc.DrawStringAnchored(displayLabel, float64(MENU_LEFT_PAD), textY, 0, 0.5)

	// Queued indicator (checkmark)
	if isInQueue {
		dc.SetFontFace(app.FontMenu)
		if selected {
			dc.SetHexColor(app.CurrentTheme.SelTxt)
		} else {
			dc.SetHexColor(app.CurrentTheme.Accent)
		}
		dc.DrawStringAnchored("✓", float64(SCREEN_WIDTH-85), textY, 0, 0.5)
	}

	// Playing indicator
	if isPlaying {
		dc.SetHexColor(app.CurrentTheme.Accent)
		app.drawPlayIcon(SCREEN_WIDTH-50, y+12, 16)
	}

	// Submenu arrow ">"
	if hasSubmenu {
		dc.SetFontFace(app.FontMenu)
		dc.SetHexColor(app.CurrentTheme.Arrow)
		dc.DrawStringAnchored(">", float64(ARROW_RIGHT_X), textY, 0, 0.5)
	}
}

// drawScrollBar draws a vertical scroll indicator on the right edge
func (app *MiyooPod) drawScrollBar(totalItems, scrollOff, visibleItems int) {
	if totalItems <= visibleItems {
		return
	}

	dc := app.DC

	barX := float64(SCREEN_WIDTH - SCROLL_BAR_WIDTH - 1)
	barTop := float64(MENU_TOP_Y)
	barHeight := float64(SCREEN_HEIGHT - MENU_TOP_Y - STATUS_BAR_HEIGHT)

	// Track
	dc.SetHexColor(app.CurrentTheme.ProgBG)
	dc.DrawRectangle(barX, barTop, SCROLL_BAR_WIDTH, barHeight)
	dc.Fill()

	// Thumb
	thumbHeight := math.Max(barHeight*float64(visibleItems)/float64(totalItems), 10)
	thumbY := barTop + barHeight*float64(scrollOff)/float64(totalItems)

	dc.SetHexColor(app.CurrentTheme.Accent)
	dc.DrawRectangle(barX, thumbY, SCROLL_BAR_WIDTH, thumbHeight)
	dc.Fill()
}

// drawProgressBar draws the playback progress bar
func (app *MiyooPod) drawProgressBar(x, y, width int, position, duration float64) {
	dc := app.DC

	// Background track
	dc.SetHexColor(app.CurrentTheme.ProgBG)
	dc.DrawRectangle(float64(x), float64(y), float64(width), PROGRESS_BAR_H)
	dc.Fill()

	// Filled portion
	if duration > 0 {
		progress := position / duration
		if progress > 1 {
			progress = 1
		}
		filledWidth := float64(width) * progress

		dc.SetHexColor(app.CurrentTheme.Progress)
		dc.DrawRectangle(float64(x), float64(y), filledWidth, PROGRESS_BAR_H)
		dc.Fill()
	}

	// Time labels
	dc.SetFontFace(app.FontTime)
	dc.SetHexColor(app.CurrentTheme.Dim)
	dc.DrawStringAnchored(formatTime(position), float64(x), float64(y)-3, 0, 1)
	dc.DrawStringAnchored(formatTime(duration), float64(x+width), float64(y)-3, 1, 1)
}

// drawPlayIcon draws a small play triangle
func (app *MiyooPod) drawPlayIcon(x, y, size int) {
	dc := app.DC
	fx := float64(x)
	fy := float64(y)
	fs := float64(size)

	dc.MoveTo(fx, fy)
	dc.LineTo(fx+fs, fy+fs/2)
	dc.LineTo(fx, fy+fs)
	dc.ClosePath()
	dc.Fill()
}

// drawPauseIcon draws two small pause bars
func (app *MiyooPod) drawPauseIcon(x, y, size int) {
	dc := app.DC
	fx := float64(x)
	fy := float64(y)
	fs := float64(size)
	barW := fs * 0.3

	dc.DrawRectangle(fx, fy, barW, fs)
	dc.Fill()
	dc.DrawRectangle(fx+fs*0.5, fy, barW, fs)
	dc.Fill()
}

// drawNextIcon draws a next track icon (>>)
func (app *MiyooPod) drawNextIcon(x, y, size int) {
	dc := app.DC
	fx := float64(x)
	fy := float64(y)
	fs := float64(size)

	// First triangle
	dc.MoveTo(fx, fy)
	dc.LineTo(fx+fs*0.4, fy+fs/2)
	dc.LineTo(fx, fy+fs)
	dc.ClosePath()
	dc.Fill()

	// Second triangle
	dc.MoveTo(fx+fs*0.5, fy)
	dc.LineTo(fx+fs*0.9, fy+fs/2)
	dc.LineTo(fx+fs*0.5, fy+fs)
	dc.ClosePath()
	dc.Fill()
}

// drawPrevIcon draws a previous track icon (<<)
func (app *MiyooPod) drawPrevIcon(x, y, size int) {
	dc := app.DC
	fx := float64(x)
	fy := float64(y)
	fs := float64(size)

	// First triangle (pointing left)
	dc.MoveTo(fx+fs*0.4, fy)
	dc.LineTo(fx, fy+fs/2)
	dc.LineTo(fx+fs*0.4, fy+fs)
	dc.ClosePath()
	dc.Fill()

	// Second triangle (pointing left)
	dc.MoveTo(fx+fs*0.9, fy)
	dc.LineTo(fx+fs*0.5, fy+fs/2)
	dc.LineTo(fx+fs*0.9, fy+fs)
	dc.ClosePath()
	dc.Fill()
}

// drawShuffleIcon draws a shuffle icon (crossed curved arrows)
func (app *MiyooPod) drawShuffleIcon(x, y, size int) {
	dc := app.DC
	fx := float64(x)
	fy := float64(y)
	fs := float64(size)

	// Scale factor from 24x24 SVG viewBox
	scale := fs / 24.0

	dc.SetLineWidth(2)

	// Top curved path (left to right upper)
	dc.MoveTo(fx+3*scale, fy+6*scale)
	dc.CubicTo(
		fx+6*scale, fy+6*scale,
		fx+8*scale, fy+3*scale,
		fx+12*scale, fy+6*scale,
	)
	dc.CubicTo(
		fx+15*scale, fy+8.5*scale,
		fx+16*scale, fy+6*scale,
		fx+18*scale, fy+6*scale,
	)
	dc.Stroke()

	// Top arrow head (pointing right and down)
	dc.MoveTo(fx+18*scale, fy+3*scale)
	dc.LineTo(fx+21*scale, fy+6*scale)
	dc.LineTo(fx+18*scale, fy+9*scale)
	dc.Stroke()

	// Bottom curved path (left to right lower, crossing)
	dc.MoveTo(fx+3*scale, fy+18*scale)
	dc.CubicTo(
		fx+6*scale, fy+18*scale,
		fx+8*scale, fy+21*scale,
		fx+12*scale, fy+18*scale,
	)
	dc.CubicTo(
		fx+15*scale, fy+15.5*scale,
		fx+16*scale, fy+18*scale,
		fx+18*scale, fy+18*scale,
	)
	dc.Stroke()

	// Bottom arrow head (pointing right and up)
	dc.MoveTo(fx+18*scale, fy+15*scale)
	dc.LineTo(fx+21*scale, fy+18*scale)
	dc.LineTo(fx+18*scale, fy+21*scale)
	dc.Stroke()
}

// drawRepeatIcon draws a repeat icon (rounded rectangle loop)
func (app *MiyooPod) drawRepeatIcon(x, y, size int) {
	dc := app.DC
	fx := float64(x)
	fy := float64(y)
	fs := float64(size)

	// Scale factor from 24x24 SVG viewBox
	scale := fs / 24.0

	dc.SetLineWidth(2)

	// Top path (right to left with rounded corners)
	dc.MoveTo(fx+14*scale, fy+5*scale)
	dc.LineTo(fx+6.8*scale, fy+5*scale)
	dc.CubicTo(
		fx+5*scale, fy+5*scale,
		fx+2*scale, fy+7*scale,
		fx+2*scale, fy+9.8*scale,
	)
	dc.LineTo(fx+2*scale, fy+15.5*scale)
	dc.Stroke()

	// Top arrow head (pointing up and left)
	dc.MoveTo(fx+11*scale, fy+2*scale)
	dc.LineTo(fx+14*scale, fy+5*scale)
	dc.LineTo(fx+11*scale, fy+8*scale)
	dc.Stroke()

	// Bottom path (left to right with rounded corners)
	dc.MoveTo(fx+10*scale, fy+19*scale)
	dc.LineTo(fx+17.2*scale, fy+19*scale)
	dc.CubicTo(
		fx+19*scale, fy+19*scale,
		fx+22*scale, fy+17*scale,
		fx+22*scale, fy+14.2*scale,
	)
	dc.LineTo(fx+22*scale, fy+8.5*scale)
	dc.Stroke()

	// Bottom arrow head (pointing down and right)
	dc.MoveTo(fx+10*scale, fy+16*scale)
	dc.LineTo(fx+13*scale, fy+19*scale)
	dc.LineTo(fx+10*scale, fy+22*scale)
	dc.Stroke()
}

// drawRepeatOneIcon draws a repeat one icon (rounded rectangle loop with vertical bar)
func (app *MiyooPod) drawRepeatOneIcon(x, y, size int) {
	dc := app.DC
	fx := float64(x)
	fy := float64(y)
	fs := float64(size)

	// Scale factor from 24x24 SVG viewBox
	scale := fs / 24.0

	dc.SetLineWidth(2)

	// Top path (right to left with rounded corners)
	dc.MoveTo(fx+14*scale, fy+5*scale)
	dc.LineTo(fx+6.8*scale, fy+5*scale)
	dc.CubicTo(
		fx+5*scale, fy+5*scale,
		fx+2*scale, fy+7*scale,
		fx+2*scale, fy+9.8*scale,
	)
	dc.LineTo(fx+2*scale, fy+15.5*scale)
	dc.Stroke()

	// Top arrow head (pointing up and left)
	dc.MoveTo(fx+11*scale, fy+2*scale)
	dc.LineTo(fx+14*scale, fy+5*scale)
	dc.LineTo(fx+11*scale, fy+8*scale)
	dc.Stroke()

	// Bottom path (left to right with rounded corners)
	dc.MoveTo(fx+10*scale, fy+19*scale)
	dc.LineTo(fx+17.2*scale, fy+19*scale)
	dc.CubicTo(
		fx+19*scale, fy+19*scale,
		fx+22*scale, fy+17*scale,
		fx+22*scale, fy+14.2*scale,
	)
	dc.LineTo(fx+22*scale, fy+8.5*scale)
	dc.Stroke()

	// Bottom arrow head (pointing down and right)
	dc.MoveTo(fx+10*scale, fy+16*scale)
	dc.LineTo(fx+13*scale, fy+19*scale)
	dc.LineTo(fx+10*scale, fy+22*scale)
	dc.Stroke()

	// Draw vertical bar in center
	dc.SetLineWidth(2.5)
	dc.DrawLine(fx+12*scale, fy+9*scale, fx+12*scale, fy+15*scale)
	dc.Stroke()
}

// measureTextCached measures text width with caching to avoid expensive font operations
func (app *MiyooPod) measureTextCached(text string, face font.Face) float64 {
	// Cache key uses text + font ID (zero-allocation for cache hits)
	cacheKey := text + "|" + app.fontID(face)

	if width, ok := app.TextMeasureCache[cacheKey]; ok {
		return width
	}

	// Must set font face before measuring
	app.DC.SetFontFace(face)
	width, _ := app.DC.MeasureString(text)
	app.TextMeasureCache[cacheKey] = width
	return width
}

// fontID returns a short stable key for a font face (zero-allocation)
func (app *MiyooPod) fontID(face font.Face) string {
	switch face {
	case app.FontHeader:
		return "H"
	case app.FontMenu:
		return "M"
	case app.FontTitle:
		return "T"
	case app.FontArtist:
		return "A"
	case app.FontAlbum:
		return "B"
	case app.FontTime:
		return "I"
	case app.FontSmall:
		return "S"
	default:
		return "X"
	}
}

// truncateText truncates text with "..." if it exceeds maxWidth
// Optimized with caching and binary search instead of O(n²) linear scan
// Requires font face to be passed for caching
func (app *MiyooPod) truncateText(text string, maxWidth float64, face font.Face) string {
	w := app.measureTextCached(text, face)
	if w <= maxWidth {
		return text
	}

	// Binary search for optimal truncation point
	left, right := 0, len(text)
	bestFit := 0

	for left <= right {
		mid := (left + right) / 2
		if mid == 0 {
			return "..."
		}

		truncated := text[:mid] + "..."
		w := app.measureTextCached(truncated, face)

		if w <= maxWidth {
			bestFit = mid
			left = mid + 1
		} else {
			right = mid - 1
		}
	}

	if bestFit == 0 {
		return "..."
	}

	return text[:bestFit] + "..."
}

// formatTime converts seconds to "M:SS" format (zero-allocation for common values)
func formatTime(seconds float64) string {
	if seconds < 0 || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return "0:00"
	}

	totalSec := int(seconds)
	min := totalSec / 60
	sec := totalSec % 60

	// Avoid fmt.Sprintf allocation - build string directly
	result := make([]byte, 0, 5)
	if min >= 10 {
		result = append(result, byte('0'+min/10))
	}
	result = append(result, byte('0'+min%10))
	result = append(result, ':')
	result = append(result, byte('0'+sec/10))
	result = append(result, byte('0'+sec%10))
	return string(result)
}

// drawCenteredText draws text centered horizontally at the given Y position
func (app *MiyooPod) drawCenteredText(text string, y float64, face font.Face, color string) {
	dc := app.DC
	dc.SetFontFace(face)
	dc.SetHexColor(color)

	maxWidth := float64(SCREEN_WIDTH - 40)
	displayText := text
	w := app.measureTextCached(text, face)
	if w > maxWidth {
		displayText = app.truncateText(text, maxWidth, face)
	}

	dc.DrawStringAnchored(displayText, SCREEN_WIDTH/2, y, 0.5, 0.5)
}

func (app *MiyooPod) drawStatusIndicators(y int) {
	dc := app.DC

	// All controls aligned to the right side (starting at x: 330 to match track info)
	rightX := 330

	// Shuffle icon
	if app.Queue != nil && app.Queue.Shuffle {
		dc.SetHexColor(app.CurrentTheme.Accent)
	} else {
		dc.SetHexColor(app.CurrentTheme.Dim)
	}
	app.drawShuffleIcon(rightX, y-10, 20)

	// Previous button
	dc.SetHexColor(app.CurrentTheme.ItemTxt)
	app.drawPrevIcon(rightX+50, y-10, 20)

	// Play/Pause in center
	if app.Playing != nil && app.Playing.State == StatePlaying {
		dc.SetHexColor(app.CurrentTheme.Accent)
		app.drawPauseIcon(rightX+100, y-10, 20)
	} else {
		dc.SetHexColor(app.CurrentTheme.Accent)
		app.drawPlayIcon(rightX+100, y-10, 20)
	}

	// Next button
	dc.SetHexColor(app.CurrentTheme.ItemTxt)
	app.drawNextIcon(rightX+140, y-10, 20)

	// Repeat icon
	if app.Queue != nil && app.Queue.Repeat != RepeatOff {
		dc.SetHexColor(app.CurrentTheme.Accent)
	} else {
		dc.SetHexColor(app.CurrentTheme.Dim)
	}

	if app.Queue != nil && app.Queue.Repeat == RepeatOne {
		app.drawRepeatOneIcon(rightX+190, y-10, 20)
	} else {
		app.drawRepeatIcon(rightX+190, y-10, 20)
	}
}

// drawStatusBar draws a permanent status bar at the bottom of the screen
func (app *MiyooPod) drawStatusBar() {
	dc := app.DC
	barY := float64(SCREEN_HEIGHT - STATUS_BAR_HEIGHT)

	// Background
	dc.SetHexColor(app.CurrentTheme.HeaderBG)
	dc.DrawRectangle(0, barY, SCREEN_WIDTH, STATUS_BAR_HEIGHT)
	dc.Fill()

	// Separator line at top of bar
	dc.SetHexColor(app.CurrentTheme.ProgBG)
	dc.SetLineWidth(1)
	dc.DrawLine(0, barY, SCREEN_WIDTH, barY)
	dc.Stroke()

	centerY := barY + float64(STATUS_BAR_HEIGHT)/2
	dc.SetFontFace(app.FontSmall)

	// Left side: context-sensitive button legend
	switch app.CurrentScreen {
	case ScreenMenu:
		app.drawButtonLegend(12, centerY, "A", "Select")
		app.drawButtonLegend(130, centerY, "B", "Back")
		if app.Playing != nil && app.Playing.State != StateStopped {
			app.drawButtonLegend(235, centerY, "START", "Now Playing")
		}
		// Show "Add to Queue" if viewing tracks, albums, or artists
		if len(app.MenuStack) > 0 {
			current := app.MenuStack[len(app.MenuStack)-1]
			if len(current.Items) > 0 {
				firstItem := current.Items[0]
				// Show Y hint for tracks, albums, or artists
				if firstItem.Track != nil || firstItem.Album != nil || firstItem.Artist != nil {
					// Only show if nothing is playing to avoid overlap
					if app.Playing == nil || app.Playing.State == StateStopped {
						app.drawButtonLegend(360, centerY, "Y", "Add to Q")
					}
				}
			}
		}
	case ScreenNowPlaying:
		app.drawButtonLegend(12, centerY, "B", "Back")
		app.drawButtonLegend(110, centerY, "A", "Play/Pause")
		app.drawButtonLegend(270, centerY, "L/R", "Prev/Next")
		app.drawButtonLegend(400, centerY, "→", "Queue")
	case ScreenQueue:
		app.drawButtonLegend(12, centerY, "B", "Back")
		app.drawButtonLegend(100, centerY, "A", "Play")
		app.drawButtonLegend(200, centerY, "X", "Remove")
		app.drawButtonLegend(320, centerY, "MENU", "Clear All")
	}

	// Right side: now playing mini status (only on menu screen)
	if app.CurrentScreen == ScreenMenu && app.Playing != nil && app.Playing.Track != nil && app.Playing.State != StateStopped {
		track := app.Playing.Track

		// Play/pause icon
		iconX := 430
		iconY := int(centerY) - 6
		if app.Playing.State == StatePlaying {
			dc.SetHexColor(app.CurrentTheme.Accent)
			app.drawPlayIcon(iconX, iconY, 12)
		} else {
			dc.SetHexColor(app.CurrentTheme.Dim)
			app.drawPauseIcon(iconX, iconY, 12)
		}

		// Track info (truncated)
		dc.SetFontFace(app.FontSmall)
		dc.SetHexColor(app.CurrentTheme.HeaderTxt)
		info := track.Artist + " - " + track.Title
		maxW := float64(SCREEN_WIDTH - iconX - 30)
		info = app.truncateText(info, maxW, app.FontSmall)
		dc.DrawStringAnchored(info, float64(iconX+18), centerY, 0, 0.5)
	}
}

// drawButtonLegend draws a single button label like "[A] Select"
func (app *MiyooPod) drawButtonLegend(x int, centerY float64, button string, label string) {
	dc := app.DC

	// Button badge
	dc.SetFontFace(app.FontSmall)
	btnW, _ := dc.MeasureString(button)
	badgePad := 6.0
	badgeW := btnW + badgePad*2
	badgeH := 20.0
	badgeX := float64(x)
	badgeY := centerY - badgeH/2

	// Badge background
	dc.SetHexColor(app.CurrentTheme.ProgBG)
	dc.DrawRoundedRectangle(badgeX, badgeY, badgeW, badgeH, 4)
	dc.Fill()

	// Badge text
	dc.SetHexColor(app.CurrentTheme.HeaderTxt)
	dc.DrawStringAnchored(button, badgeX+badgeW/2, centerY, 0.5, 0.5)

	// Label text
	dc.SetHexColor(app.CurrentTheme.Dim)
	dc.DrawStringAnchored(label, badgeX+badgeW+6, centerY, 0, 0.5)
}

// getBatteryLevel reads the battery percentage from the system
func getBatteryLevel() int {
	// Try reading battery capacity from various paths
	// Miyoo Mini Plus stores battery percentage in /tmp/percBat
	paths := []string{
		"/tmp/percBat", // Miyoo Mini Plus
		"/sys/class/power_supply/battery/capacity",
		"/sys/class/power_supply/BAT0/capacity",
		"/sys/class/power_supply/axp22-battery/capacity",
		"/proc/battery",
		"/proc/miyoo/battery",
		"/sys/class/mstar/battery",
		"/mnt/SDCARD/.tmp_update/battery",
		"/tmp/battery_capacity",
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err == nil {
			capacityStr := strings.TrimSpace(string(data))
			capacity, err := strconv.Atoi(capacityStr)
			if err == nil && capacity >= 0 && capacity <= 100 {
				return capacity
			}
		}
	}

	return -1 // Battery level unavailable
}

// drawBatteryIcon draws a battery icon with percentage-based fill
func (app *MiyooPod) drawBatteryIcon(x, y, percent int) {
	dc := app.DC

	// Battery dimensions
	bodyW := 20.0
	bodyH := 12.0
	tipW := 2.0
	tipH := 6.0

	fx := float64(x)
	fy := float64(y)

	// Battery body outline
	dc.SetLineWidth(1.5)
	if percent <= 20 {
		dc.SetHexColor("#FF5555") // Red for low battery
	} else if percent <= 40 {
		dc.SetHexColor("#FFAA00") // Orange for medium-low
	} else {
		dc.SetHexColor(app.CurrentTheme.HeaderTxt)
	}
	dc.DrawRectangle(fx, fy, bodyW, bodyH)
	dc.Stroke()

	// Battery tip (right side)
	dc.DrawRectangle(fx+bodyW, fy+(bodyH-tipH)/2, tipW, tipH)
	dc.Fill()

	// Battery fill level
	if percent > 0 {
		fillW := (bodyW - 3) * float64(percent) / 100.0
		if percent <= 20 {
			dc.SetHexColor("#FF5555")
		} else if percent <= 40 {
			dc.SetHexColor("#FFAA00")
		} else {
			dc.SetHexColor(app.CurrentTheme.Accent)
		}
		dc.DrawRectangle(fx+1.5, fy+1.5, fillW, bodyH-3)
		dc.Fill()
	}
}

// showError displays an error message popup for 1 second
func (app *MiyooPod) showError(message string) {
	app.ErrorMessage = message
	app.ErrorTime = time.Now()
	app.drawCurrentScreen() // Redraw immediately to show the error
}

// drawErrorPopup draws an error popup overlay if an error is active
func (app *MiyooPod) drawErrorPopup() {
	if app.ErrorMessage == "" {
		return
	}

	// Check if 1 second has elapsed
	elapsed := time.Since(app.ErrorTime)
	if elapsed > time.Second {
		app.ErrorMessage = "" // Clear the error
		return
	}

	dc := app.DC

	// Semi-transparent overlay
	dc.SetRGBA(0, 0, 0, 0.7)
	dc.DrawRectangle(0, 0, SCREEN_WIDTH, SCREEN_HEIGHT)
	dc.Fill()

	// Error popup dimensions
	popupWidth := 500.0
	popupHeight := 160.0
	popupX := (SCREEN_WIDTH - popupWidth) / 2
	popupY := (SCREEN_HEIGHT - popupHeight) / 2

	// Popup background
	dc.SetHexColor("#2C2C2C")
	dc.DrawRoundedRectangle(popupX, popupY, popupWidth, popupHeight, 12)
	dc.Fill()

	// Popup border
	dc.SetHexColor("#FF5555")
	dc.SetLineWidth(3)
	dc.DrawRoundedRectangle(popupX, popupY, popupWidth, popupHeight, 12)
	dc.Stroke()

	// Warning icon (triangle with exclamation mark)
	iconSize := 40.0
	iconX := popupX + 30
	iconY := popupY + popupHeight/2 - iconSize/2

	// Warning triangle
	dc.SetHexColor("#FF5555")
	dc.MoveTo(iconX+iconSize/2, iconY)
	dc.LineTo(iconX+iconSize, iconY+iconSize)
	dc.LineTo(iconX, iconY+iconSize)
	dc.ClosePath()
	dc.Fill()

	// Exclamation mark
	dc.SetHexColor("#FFFFFF")
	dc.DrawRectangle(iconX+iconSize/2-3, iconY+10, 6, 18)
	dc.Fill()
	dc.DrawCircle(iconX+iconSize/2, iconY+34, 3)
	dc.Fill()

	// Error text
	dc.SetFontFace(app.FontMenu)
	dc.SetHexColor("#FFFFFF")
	textX := popupX + 90
	textY := popupY + popupHeight/2
	dc.DrawStringWrapped(app.ErrorMessage, textX, textY, 0, 0.5, popupWidth-110, 1.5, 0)
}

// toggleLock toggles the screen lock state
func (app *MiyooPod) toggleLock() {
	app.Locked = !app.Locked

	if app.Locked {
		// Save current brightness and fully dim the screen
		app.BrightnessBeforeLock = getBrightness()
		setBrightness(0) // Fully dim (screen off)
	} else {
		// Restore previous brightness
		if app.BrightnessBeforeLock > 0 {
			setBrightness(app.BrightnessBeforeLock)
		} else {
			setBrightness(100) // Default to 100 if no saved value
		}
	}

	app.drawCurrentScreen()
}

// drawLockOverlay draws a dimmed overlay when screen is locked
func (app *MiyooPod) drawLockOverlay() {
	dc := app.DC

	// Semi-transparent black overlay to dim the screen
	dc.SetRGBA(0, 0, 0, 0.6)
	dc.DrawRectangle(0, 0, SCREEN_WIDTH, SCREEN_HEIGHT)
	dc.Fill()

	// "LOCKED" text
	centerX := SCREEN_WIDTH / 2.0
	centerY := SCREEN_HEIGHT / 2.0

	dc.SetFontFace(app.FontTitle)
	dc.SetHexColor("#FFFFFF")
	dc.DrawStringAnchored("LOCKED", centerX, centerY-15, 0.5, 0.5)

	// Hint text
	dc.SetFontFace(app.FontSmall)
	dc.SetHexColor("#CCCCCC")
	lockKeyName := app.getLockKeyName()
	dc.DrawStringAnchored(fmt.Sprintf("Double-press %s to unlock", lockKeyName), centerX, centerY+15, 0.5, 0.5)
}

// getBrightness reads the current PWM duty_cycle (brightness level)
func getBrightness() int {
	data, err := os.ReadFile("/sys/class/pwm/pwmchip0/pwm0/duty_cycle")
	if err != nil {
		return -1
	}
	brightness, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return -1
	}
	return brightness
}

// setBrightness sets the PWM duty_cycle (brightness level)
// Valid range is typically 0-100 for Miyoo Mini Plus
func setBrightness(level int) {
	if level < 0 {
		level = 0
	}
	if level > 100 {
		level = 100
	}
	os.WriteFile("/sys/class/pwm/pwmchip0/pwm0/duty_cycle", []byte(fmt.Sprintf("%d", level)), 0644)
}
