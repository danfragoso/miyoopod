package main

import (
	"image"
	"image/color"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// Glyph atlas: a lazy per-glyph cache plus a composed label sprite cache.
//
// Text in the hot UI paths (menu lists especially) is otherwise re-rasterized
// through freetype every frame. Instead we render each glyph once (white on
// transparent), cache it, and compose whole labels from cached glyphs into a
// label sprite that is blitted with tinting. After warmup, drawing a label is
// a memcpy-style blit and touches freetype zero times.

// GlyphKey identifies a rasterized glyph for a given font face.
type GlyphKey struct {
	Face uint8
	R    rune
}

// Glyph is a tight white-on-transparent sprite plus placement metrics.
type Glyph struct {
	Img      *image.RGBA // nil for whitespace / missing glyphs
	Advance  int         // pen advance in pixels
	BearingX int         // left side bearing (dr.Min.X)
	Top      int         // top relative to baseline (dr.Min.Y, negative = above)
}

// LabelKey identifies a composed label sprite (color-independent: tinted at blit).
type LabelKey struct {
	Text string
	Face uint8
}

// LabelSprite is a fully composed line of text, white on transparent.
type LabelSprite struct {
	Img  *image.RGBA
	W, H int // sprite dimensions including padding
	PadX int // horizontal padding (left origin offset)
	PadY int // vertical padding (top offset)
	BoxH int // font metric box height (ascent+descent), used for centering
}

const (
	labelCacheLimit = 768 // bounded; cleared wholesale when exceeded
	labelPadX       = 2
	labelPadY       = 2
)

// faceID maps a font face to a small stable id for cache keys.
func (app *MiyooPod) faceID(face font.Face) uint8 {
	switch face {
	case app.FontHeader:
		return 1
	case app.FontMenu:
		return 2
	case app.FontTitle:
		return 3
	case app.FontArtist:
		return 4
	case app.FontAlbum:
		return 5
	case app.FontTime:
		return 6
	case app.FontSmall:
		return 7
	default:
		return 0
	}
}

// getGlyph returns the cached white sprite + metrics for a rune, rendering it
// on first sight. Never returns nil (whitespace yields a sprite-less advance).
func (app *MiyooPod) getGlyph(face font.Face, fid uint8, r rune) *Glyph {
	key := GlyphKey{Face: fid, R: r}
	if g, ok := app.GlyphCache[key]; ok {
		return g
	}

	dr, mask, maskp, adv, ok := face.Glyph(fixed.Point26_6{}, r)
	if !ok || dr.Dx() <= 0 || dr.Dy() <= 0 {
		a, _ := face.GlyphAdvance(r)
		g := &Glyph{Img: nil, Advance: a.Round()}
		app.GlyphCache[key] = g
		return g
	}

	w := dr.Dx()
	h := dr.Dy()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			_, _, _, a := mask.At(maskp.X+x, maskp.Y+y).RGBA()
			alpha := uint8(a >> 8)
			if alpha == 0 {
				continue
			}
			o := y*img.Stride + x*4
			img.Pix[o] = 255
			img.Pix[o+1] = 255
			img.Pix[o+2] = 255
			img.Pix[o+3] = alpha
		}
	}

	g := &Glyph{
		Img:      img,
		Advance:  adv.Round(),
		BearingX: dr.Min.X,
		Top:      dr.Min.Y,
	}
	app.GlyphCache[key] = g
	return g
}

// measureTextFast returns the pixel width of s using cached glyph advances and
// kerning. Self-consistent with getLabelSprite's composition.
func (app *MiyooPod) measureTextFast(s string, fid uint8, face font.Face) int {
	var width fixed.Int26_6
	prev := rune(-1)
	for _, r := range s {
		if prev >= 0 {
			width += face.Kern(prev, r)
		}
		a, ok := face.GlyphAdvance(r)
		if !ok {
			a, _ = face.GlyphAdvance(' ')
		}
		width += a
		prev = r
	}
	return width.Ceil()
}

// getLabelSprite composes (and caches) a white-on-transparent sprite for text.
func (app *MiyooPod) getLabelSprite(text string, fid uint8, face font.Face) *LabelSprite {
	key := LabelKey{Text: text, Face: fid}
	if ls, ok := app.LabelCache[key]; ok {
		return ls
	}
	if len(app.LabelCache) >= labelCacheLimit {
		app.LabelCache = make(map[LabelKey]*LabelSprite)
	}

	m := face.Metrics()
	ascent := m.Ascent.Ceil()
	descent := m.Descent.Ceil()
	boxH := ascent + descent
	if boxH < 1 {
		boxH = 1
	}

	textW := app.measureTextFast(text, fid, face)
	imgW := textW + 2*labelPadX
	imgH := boxH + 2*labelPadY
	if imgW < 1 {
		imgW = 1
	}

	img := image.NewRGBA(image.Rect(0, 0, imgW, imgH))
	baseline := labelPadY + ascent

	penX := fixed.Int26_6(0)
	prev := rune(-1)
	for _, r := range text {
		if prev >= 0 {
			penX += face.Kern(prev, r)
		}
		g := app.getGlyph(face, fid, r)
		if g.Img != nil {
			dx := labelPadX + penX.Round() + g.BearingX
			dy := baseline + g.Top
			blitGlyphInto(img, g.Img, dx, dy)
		}
		a, ok := face.GlyphAdvance(r)
		if !ok {
			a, _ = face.GlyphAdvance(' ')
		}
		penX += a
		prev = r
	}

	ls := &LabelSprite{Img: img, W: imgW, H: imgH, PadX: labelPadX, PadY: labelPadY, BoxH: boxH}
	app.LabelCache[key] = ls
	return ls
}

// drawLabelCached blits a tinted, vertically-centered label at the given left
// edge x. centerY is the vertical center of the font metric box, matching
// gg.DrawStringAnchored(text, x, centerY, 0, 0.5).
func (app *MiyooPod) drawLabelCached(text string, x, centerY int, fid uint8, face font.Face, col color.RGBA) {
	if text == "" {
		return
	}
	ls := app.getLabelSprite(text, fid, face)
	left := x - ls.PadX
	top := centerY - ls.BoxH/2 - ls.PadY
	app.fastBlitTinted(ls.Img, left, top, col.R, col.G, col.B)
}

// blitGlyphInto composites a white glyph sprite into a label sprite, keeping the
// stronger alpha where glyphs overlap.
func blitGlyphInto(dst, src *image.RGBA, dx, dy int) {
	dw := dst.Rect.Dx()
	dh := dst.Rect.Dy()
	sw := src.Rect.Dx()
	sh := src.Rect.Dy()
	for sy := 0; sy < sh; sy++ {
		ty := dy + sy
		if ty < 0 || ty >= dh {
			continue
		}
		for sx := 0; sx < sw; sx++ {
			tx := dx + sx
			if tx < 0 || tx >= dw {
				continue
			}
			so := sy*src.Stride + sx*4
			sa := src.Pix[so+3]
			if sa == 0 {
				continue
			}
			to := ty*dst.Stride + tx*4
			if sa >= dst.Pix[to+3] {
				dst.Pix[to] = 255
				dst.Pix[to+1] = 255
				dst.Pix[to+2] = 255
				dst.Pix[to+3] = sa
			}
		}
	}
}
