package main

import (
	"time"

	"github.com/skip2/go-qrcode"
)

// showAboutScreen displays the about page with version info, credits, and donation QR code
func (app *MiyooPod) showAboutScreen() {
	dc := app.DC

	// Set background
	dc.SetHexColor(app.CurrentTheme.BG)
	dc.Clear()

	// Title
	dc.SetFontFace(app.FontTitle)
	dc.SetHexColor(app.CurrentTheme.HeaderTxt)
	dc.DrawStringAnchored("About MiyooPod", SCREEN_WIDTH/2, 30, 0.5, 0.5)

	// Version and author info
	dc.SetFontFace(app.FontMenu)
	dc.SetHexColor(app.CurrentTheme.ItemTxt)

	yPos := 80
	dc.DrawStringAnchored("Version: "+APP_VERSION, SCREEN_WIDTH/2, float64(yPos), 0.5, 0.5)

	yPos += 30
	dc.DrawStringAnchored("Created by: "+APP_AUTHOR, SCREEN_WIDTH/2, float64(yPos), 0.5, 0.5)

	yPos += 40
	dc.SetFontFace(app.FontSmall)
	dc.SetHexColor(app.CurrentTheme.Dim)
	dc.DrawStringAnchored("A music player for Miyoo Mini", SCREEN_WIDTH/2, float64(yPos), 0.5, 0.5)

	yPos += 20
	dc.DrawStringAnchored("Inspired by classic music players", SCREEN_WIDTH/2, float64(yPos), 0.5, 0.5)

	// Generate QR code
	qr, err := qrcode.New(SUPPORT_URL, qrcode.Medium)
	if err == nil {
		qrSize := 150
		qrImg := qr.Image(qrSize)

		// Draw QR code
		qrX := (SCREEN_WIDTH - qrSize) / 2
		qrY := 220
		app.fastBlitImage(qrImg, qrX, qrY)

		// QR code label
		dc.SetFontFace(app.FontSmall)
		dc.SetHexColor(app.CurrentTheme.ItemTxt)
		dc.DrawStringAnchored("Support this project", SCREEN_WIDTH/2, float64(qrY+qrSize+20), 0.5, 0.5)
	}

	// Instructions
	dc.SetFontFace(app.FontSmall)
	dc.SetHexColor(app.CurrentTheme.Dim)
	dc.DrawStringAnchored("Press B to return", SCREEN_WIDTH/2, SCREEN_HEIGHT-20, 0.5, 0.5)

	app.triggerRefresh()

	// Wait for B button to return
	app.waitForAboutExit()
}

// waitForAboutExit waits for the user to press B to exit the about screen
func (app *MiyooPod) waitForAboutExit() {
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
	}
}
