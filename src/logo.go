package main

import (
	"fmt"
	"image/png"
	"os"

	"github.com/fogleman/gg"
)

// drawLogo draws the MiyooPod logo with the given theme colors
// x, y: center position of the logo
// scale: size multiplier (1.0 = original 249x330 size)
// colorScreen: if true, colors the screen area with theme background color instead of black
func (app *MiyooPod) drawLogo(x, y float64, scale float64, colorScreen bool) {
	dc := app.DC

	// Save current state
	dc.Push()

	// Translate to center position
	dc.Translate(x, y)
	dc.Scale(scale, scale)

	// Center the logo (original size is 249x330)
	logoWidth := 249.0
	logoHeight := 330.0
	dc.Translate(-logoWidth/2, -logoHeight/2)

	// Get theme colors
	logoBodyColor := app.CurrentTheme.ItemTxt // Logo body uses text color

	// Button and screen colors for proper contrast
	var screenColor, buttonColor string
	if colorScreen {
		// For icon: screen uses theme BG, buttons use theme BG (same as screen)
		screenColor = app.CurrentTheme.BG
		buttonColor = app.CurrentTheme.BG
	} else {
		// For splash: screen uses theme BG (not black), buttons use theme BG
		screenColor = app.CurrentTheme.BG
		buttonColor = app.CurrentTheme.BG
	}

	// Main body (rounded rectangle) - use theme text color
	dc.DrawRoundedRectangle(0, 0, logoWidth, logoHeight, 40)
	dc.SetHexColor(logoBodyColor)
	dc.Fill()

	// Screen area (rectangle with rounded corners)
	dc.DrawRoundedRectangle(24, 23, 201, 132, 20)
	dc.SetHexColor(screenColor)
	dc.Fill()

	// Play button (triangle) - matches screen color for contrast with body
	dc.SetHexColor(buttonColor)
	dc.MoveTo(37.875, 184.167)
	dc.LineTo(110.062, 242.5)
	dc.LineTo(37.875, 300.833)
	dc.ClosePath()
	dc.Fill()

	// Pause button (two bars) - matches screen color for contrast with body
	dc.SetHexColor(buttonColor)
	dc.DrawRectangle(138.938, 184.167, 28.874, 116.666)
	dc.Fill()

	dc.DrawRectangle(182.25, 184.167, 28.875, 116.666)
	dc.Fill()

	// Restore state
	dc.Pop()
}

// generateIconPNG creates a 64x64 icon PNG with the current theme colors
func (app *MiyooPod) generateIconPNG() error {
	// Create a new context for the icon
	iconSize := 64
	iconDC := gg.NewContext(iconSize, iconSize)

	// Clear with transparent background
	iconDC.SetRGBA(0, 0, 0, 0)
	iconDC.Clear()

	// Draw white border
	borderWidth := 2.0
	iconDC.SetRGB(1, 1, 1) // White
	iconDC.DrawRoundedRectangle(0, 0, float64(iconSize), float64(iconSize), 8)
	iconDC.Fill()

	// Create a temporary app-like struct to use drawLogo
	// We'll draw directly on the icon context
	originalDC := app.DC
	app.DC = iconDC

	// Draw the logo centered in the icon, scaled down to fit (with padding for border)
	// Original logo is 249x330, we want to fit in 64x64 with border
	// Scale factor accounts for border padding
	scale := (float64(iconSize) - borderWidth*4) / 330.0
	app.drawLogo(float64(iconSize)/2, float64(iconSize)/2, scale, true)

	// Restore original context
	app.DC = originalDC

	// Ensure assets directory exists
	assetsDir := "./assets"
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		return fmt.Errorf("failed to create assets directory: %v", err)
	}

	// Save to file
	iconPath := assetsDir + "/icn.png"
	file, err := os.Create(iconPath)
	if err != nil {
		return fmt.Errorf("failed to create icon file: %v", err)
	}
	defer file.Close()

	img := iconDC.Image()
	if err := png.Encode(file, img); err != nil {
		return fmt.Errorf("failed to encode icon PNG: %v", err)
	}

	logMsg(fmt.Sprintf("Generated icon: %s", iconPath))
	return nil
}

// drawLogoSplash draws the logo on the splash screen with theme-tinted background
func (app *MiyooPod) drawLogoSplash() {
	// Clear with theme background
	app.DC.SetHexColor(app.CurrentTheme.BG)
	app.DC.Clear()

	// Draw the logo in the upper portion
	logoScale := 0.6 // Scale down to ~150px width
	app.drawLogo(SCREEN_WIDTH/2, SCREEN_HEIGHT/2-60, logoScale, false)

	// Draw "MiyooPod" title below the logo
	app.DC.SetFontFace(app.FontTitle)
	app.DC.SetHexColor(app.CurrentTheme.HeaderTxt)
	app.DC.DrawStringWrapped("MiyooPod", SCREEN_WIDTH/2, SCREEN_HEIGHT/2+100, 0.5, 0.5, 600, 1.5, gg.AlignCenter)

	// Draw "Loading..." text
	app.DC.SetFontFace(app.FontSmall)
	app.DC.SetHexColor(app.CurrentTheme.Dim)
	app.DC.DrawStringWrapped("Loading...", SCREEN_WIDTH/2, SCREEN_HEIGHT/2+140, 0.5, 0.5, 600, 1.5, gg.AlignCenter)

	app.triggerRefresh()
}

