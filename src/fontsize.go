package main

import "fmt"

// applyFontSize reloads the font faces at the current FontSize scale and clears
// every size-dependent cache so the UI re-renders with the new metrics.
func (app *MiyooPod) applyFontSize() {
	app.loadFonts()

	// Glyph/label/measure caches are keyed by face id, not size, so they must
	// be discarded when the face sizes change.
	app.GlyphCache = make(map[GlyphKey]*Glyph)
	app.LabelCache = make(map[LabelKey]*LabelSprite)
	app.TextMeasureCache = make(map[string]float64)

	// Pre-rendered time digits depend on FontTime.
	app.initDigitSprites(app.FontTime)

	// Invalidate cached screen frames.
	app.MenuBG = nil
	app.NPCacheDirty = true

	// Lyrics wrap widths are cached per track; force a re-wrap.
	app.LyricsCachedTrack = ""
}

// cycleFontSize advances the font size preset (Small -> Medium -> Large -> Small),
// reapplies fonts, rebuilds the settings menu label, and persists the choice.
func (app *MiyooPod) cycleFontSize() {
	switch app.FontSize {
	case FontSizeSmall:
		app.FontSize = FontSizeMedium
	case FontSizeMedium:
		app.FontSize = FontSizeLarge
	default:
		app.FontSize = FontSizeSmall
	}

	app.applyFontSize()

	// Update the current menu item's label in place rather than rebuilding the
	// menu. Rebuilding resets SelIndex to 0 (Themes), which — combined with key
	// repeat — could cause the next RIGHT press to enter the Themes submenu.
	if len(app.MenuStack) > 0 {
		current := app.MenuStack[len(app.MenuStack)-1]
		if current.SelIndex >= 0 && current.SelIndex < len(current.Items) {
			current.Items[current.SelIndex].Label = "Font Size: " + fontSizeName(app.FontSize)
		}
	}

	app.drawCurrentScreen()

	if err := app.saveSettings(); err != nil {
		logMsg(fmt.Sprintf("ERROR: Failed to save font size preference: %v", err))
	}
}
