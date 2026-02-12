package main

import (
	"image"
	"image/color"
	"strconv"

	"github.com/fogleman/gg"
	"golang.org/x/image/font"
)

// Pre-rendered digit sprites for fast time display (bypass gg text rendering)
type DigitSprites struct {
	Glyphs map[byte]*image.RGBA // '0'-'9', ':'
	GlyphW map[byte]int         // width of each glyph
	Height int                  // common height
	Loaded bool
}

// parseHexColor converts "#RRGGBB" to RGBA bytes
func parseHexColor(hex string) (r, g, b, a uint8) {
	if len(hex) == 7 && hex[0] == '#' {
		hex = hex[1:]
	}
	if len(hex) != 6 {
		return 0, 0, 0, 255
	}
	rr, _ := strconv.ParseUint(hex[0:2], 16, 8)
	gg, _ := strconv.ParseUint(hex[2:4], 16, 8)
	bb, _ := strconv.ParseUint(hex[4:6], 16, 8)
	return uint8(rr), uint8(gg), uint8(bb), 255
}

// initDigitSprites pre-renders '0'-'9' and ':' at the given font face
func (app *MiyooPod) initDigitSprites(face font.Face) {
	app.Digits = &DigitSprites{
		Glyphs: make(map[byte]*image.RGBA),
		GlyphW: make(map[byte]int),
	}

	chars := "0123456789:"
	for i := 0; i < len(chars); i++ {
		ch := chars[i]
		text := string(ch)

		// Measure character width
		dc := gg.NewContext(1, 1)
		dc.SetFontFace(face)
		w, _ := dc.MeasureString(text)
		charW := int(w) + 2
		charH := 24 // font size 18 with padding

		// Render character to small RGBA buffer
		dc2 := gg.NewContext(charW, charH)
		dc2.SetFontFace(face)
		dc2.SetRGBA(1, 1, 1, 1) // white - we'll tint when blitting
		dc2.DrawStringAnchored(text, 0, float64(charH)/2, 0, 0.5)

		img, ok := dc2.Image().(*image.RGBA)
		if !ok {
			nrgba := dc2.Image().(*image.NRGBA)
			img = image.NewRGBA(nrgba.Rect)
			for y := 0; y < charH; y++ {
				for x := 0; x < charW; x++ {
					img.Set(x, y, nrgba.At(x, y))
				}
			}
		}

		app.Digits.Glyphs[ch] = img
		app.Digits.GlyphW[ch] = charW
		app.Digits.Height = charH
	}
	app.Digits.Loaded = true
}

// fastFillRect fills a rectangle directly in the framebuffer with the given color
// Much faster than gg.DrawRectangle + Fill on ARM
func (app *MiyooPod) fastFillRect(x, y, w, h int, r, g, b, a uint8) {
	fb := app.FB
	bounds := fb.Rect
	stride := fb.Stride

	// Clip to framebuffer bounds
	x0 := x
	y0 := y
	x1 := x + w
	y1 := y + h
	if x0 < bounds.Min.X {
		x0 = bounds.Min.X
	}
	if y0 < bounds.Min.Y {
		y0 = bounds.Min.Y
	}
	if x1 > bounds.Max.X {
		x1 = bounds.Max.X
	}
	if y1 > bounds.Max.Y {
		y1 = bounds.Max.Y
	}
	if x0 >= x1 || y0 >= y1 {
		return
	}

	pix := fb.Pix
	for row := y0; row < y1; row++ {
		rowOff := row*stride + x0*4
		for col := x0; col < x1; col++ {
			pix[rowOff] = r
			pix[rowOff+1] = g
			pix[rowOff+2] = b
			pix[rowOff+3] = a
			rowOff += 4
		}
	}
}

// fastCopyRegion copies a rectangular region from src to dst framebuffer
// Only copies the specified rows, much faster than full framebuffer copy
func fastCopyRegion(dst, src *image.RGBA, y0, y1 int) {
	if y0 < 0 {
		y0 = 0
	}
	if y1 > dst.Rect.Max.Y {
		y1 = dst.Rect.Max.Y
	}
	if y0 >= y1 {
		return
	}

	stride := dst.Stride
	startOff := y0 * stride
	endOff := y1 * stride
	copy(dst.Pix[startOff:endOff], src.Pix[startOff:endOff])
}

// fastBlitTinted blits a pre-rendered glyph sprite onto the framebuffer
// with color tinting (source alpha * tint color)
func (app *MiyooPod) fastBlitTinted(sprite *image.RGBA, dx, dy int, tr, tg, tb uint8) {
	fb := app.FB
	fbBounds := fb.Rect
	spBounds := sprite.Rect
	sw := spBounds.Dx()
	sh := spBounds.Dy()

	for sy := 0; sy < sh; sy++ {
		fy := dy + sy
		if fy < fbBounds.Min.Y || fy >= fbBounds.Max.Y {
			continue
		}
		for sx := 0; sx < sw; sx++ {
			fx := dx + sx
			if fx < fbBounds.Min.X || fx >= fbBounds.Max.X {
				continue
			}

			srcOff := sy*sprite.Stride + sx*4
			alpha := sprite.Pix[srcOff+3]
			if alpha == 0 {
				continue
			}

			dstOff := fy*fb.Stride + fx*4
			if alpha == 255 {
				fb.Pix[dstOff] = tr
				fb.Pix[dstOff+1] = tg
				fb.Pix[dstOff+2] = tb
				fb.Pix[dstOff+3] = 255
			} else {
				// Alpha blend
				sa := uint16(alpha)
				da := uint16(255 - alpha)
				fb.Pix[dstOff] = uint8((uint16(tr)*sa + uint16(fb.Pix[dstOff])*da) / 255)
				fb.Pix[dstOff+1] = uint8((uint16(tg)*sa + uint16(fb.Pix[dstOff+1])*da) / 255)
				fb.Pix[dstOff+2] = uint8((uint16(tb)*sa + uint16(fb.Pix[dstOff+2])*da) / 255)
				fb.Pix[dstOff+3] = 255
			}
		}
	}
}

// fastDrawTimeString draws a time string like "1:23" using pre-rendered digit sprites
// Returns the total width drawn
func (app *MiyooPod) fastDrawTimeString(text string, x, y int, r, g, b uint8) int {
	if app.Digits == nil || !app.Digits.Loaded {
		return 0
	}

	cx := x
	for i := 0; i < len(text); i++ {
		ch := text[i]
		sprite, ok := app.Digits.Glyphs[ch]
		if !ok {
			continue
		}
		app.fastBlitTinted(sprite, cx, y-app.Digits.Height/2, r, g, b)
		cx += app.Digits.GlyphW[ch]
	}
	return cx - x
}

// fastDrawTimeStringRight draws a time string right-aligned at x
func (app *MiyooPod) fastDrawTimeStringRight(text string, x, y int, r, g, b uint8) {
	if app.Digits == nil || !app.Digits.Loaded {
		return
	}

	// Calculate total width first
	totalW := 0
	for i := 0; i < len(text); i++ {
		ch := text[i]
		if w, ok := app.Digits.GlyphW[ch]; ok {
			totalW += w
		}
	}

	app.fastDrawTimeString(text, x-totalW, y, r, g, b)
}

// fastDrawProgressBar draws the progress bar using direct pixel operations
// This is the hot path called every second - must be as fast as possible
func (app *MiyooPod) fastDrawProgressBar(x, y, width int, position, duration float64) {
	bgR, bgG, bgB, _ := parseHexColor(app.CurrentTheme.ProgBG)
	fgR, fgG, fgB, _ := parseHexColor(app.CurrentTheme.Progress)
	dimR, dimG, dimB, _ := parseHexColor(app.CurrentTheme.Dim)

	// Background track
	app.fastFillRect(x, y, width, PROGRESS_BAR_H, bgR, bgG, bgB, 255)

	// Filled portion
	if duration > 0 {
		progress := position / duration
		if progress > 1 {
			progress = 1
		}
		filledWidth := int(float64(width) * progress)
		if filledWidth > 0 {
			app.fastFillRect(x, y, filledWidth, PROGRESS_BAR_H, fgR, fgG, fgB, 255)
		}
	}

	// Time labels using pre-rendered digit sprites
	posText := formatTime(position)
	durText := formatTime(duration)

	timeY := y - 15
	app.fastDrawTimeString(posText, x, timeY, dimR, dimG, dimB)
	app.fastDrawTimeStringRight(durText, x+width, timeY, dimR, dimG, dimB)
}

// fastBlitImage copies an image.Image directly to the framebuffer at dx,dy
// Much faster than gg.DrawImage for pre-rendered images
func (app *MiyooPod) fastBlitImage(src image.Image, dx, dy int) {
	fb := app.FB
	bounds := src.Bounds()
	sw := bounds.Dx()
	sh := bounds.Dy()

	// Try fast path for RGBA images
	if rgba, ok := src.(*image.RGBA); ok {
		for sy := 0; sy < sh; sy++ {
			fy := dy + sy
			if fy < 0 || fy >= SCREEN_HEIGHT {
				continue
			}
			srcOff := sy * rgba.Stride
			dstOff := fy*fb.Stride + dx*4
			srcEnd := srcOff + sw*4
			dstEnd := dstOff + sw*4
			if dx >= 0 && dx+sw <= SCREEN_WIDTH {
				copy(fb.Pix[dstOff:dstEnd], rgba.Pix[srcOff:srcEnd])
			}
		}
		return
	}

	// Fallback for other image types
	for sy := 0; sy < sh; sy++ {
		fy := dy + sy
		if fy < 0 || fy >= SCREEN_HEIGHT {
			continue
		}
		for sx := 0; sx < sw; sx++ {
			fx := dx + sx
			if fx < 0 || fx >= SCREEN_WIDTH {
				continue
			}
			r, g, b, a := src.At(bounds.Min.X+sx, bounds.Min.Y+sy).RGBA()
			dstOff := fy*fb.Stride + fx*4
			fb.Pix[dstOff] = uint8(r >> 8)
			fb.Pix[dstOff+1] = uint8(g >> 8)
			fb.Pix[dstOff+2] = uint8(b >> 8)
			fb.Pix[dstOff+3] = uint8(a >> 8)
		}
	}
}

// Region constants for partial framebuffer updates
const (
	// Progress bar dirty region: covers time labels + bar + padding
	PROGRESS_REGION_Y0 = PROGRESS_BAR_Y - 22                 // above time text
	PROGRESS_REGION_Y1 = PROGRESS_BAR_Y + PROGRESS_BAR_H + 4 // below bar
)

// blitMarqueeWindow blits a horizontal sliding window from a pre-rendered strip
// onto the framebuffer, wrapping around seamlessly. Only non-transparent pixels
// are drawn (tinted with tr,tg,tb), so the header background shows through.
func (app *MiyooPod) blitMarqueeWindow(dstX, dstY, windowW int, strip *image.RGBA, offset int, tr, tg, tb uint8) {
	fb := app.FB
	fbBounds := fb.Rect
	stripW := strip.Rect.Dx()
	stripH := strip.Rect.Dy()

	if stripW <= 0 || windowW <= 0 {
		return
	}

	for sy := 0; sy < stripH; sy++ {
		fy := dstY + sy
		if fy < fbBounds.Min.Y || fy >= fbBounds.Max.Y {
			continue
		}
		for wx := 0; wx < windowW; wx++ {
			fx := dstX + wx
			if fx < fbBounds.Min.X || fx >= fbBounds.Max.X {
				continue
			}

			// Source x wraps around the strip
			sx := (offset + wx) % stripW

			srcOff := sy*strip.Stride + sx*4
			alpha := strip.Pix[srcOff+3]
			if alpha == 0 {
				continue
			}

			dstOff := fy*fb.Stride + fx*4
			if alpha == 255 {
				fb.Pix[dstOff] = tr
				fb.Pix[dstOff+1] = tg
				fb.Pix[dstOff+2] = tb
				fb.Pix[dstOff+3] = 255
			} else {
				sa := uint16(alpha)
				da := uint16(255 - alpha)
				fb.Pix[dstOff] = uint8((uint16(tr)*sa + uint16(fb.Pix[dstOff])*da) / 255)
				fb.Pix[dstOff+1] = uint8((uint16(tg)*sa + uint16(fb.Pix[dstOff+1])*da) / 255)
				fb.Pix[dstOff+2] = uint8((uint16(tb)*sa + uint16(fb.Pix[dstOff+2])*da) / 255)
				fb.Pix[dstOff+3] = 255
			}
		}
	}
}

// parseColor is a helper that returns color.RGBA from hex
func parseColor(hex string) color.RGBA {
	r, g, b, a := parseHexColor(hex)
	return color.RGBA{r, g, b, a}
}
