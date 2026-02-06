package main

import (
	"fmt"
	"time"

	"github.com/fogleman/gg"
)

func (app *MiyooPod) UpdateFPS() {
	now := time.Now()
	app.FrameCount++

	elapsed := now.Sub(app.FPSStartTime).Seconds()
	if elapsed >= 3.0 {
		app.CurrentFPS = float64(app.FrameCount) / elapsed
		app.FrameCount = 0
		app.FPSStartTime = now
	}
}

func (app *MiyooPod) DrawFPS() {
	if !app.ShowFPS {
		return
	}

	app.UpdateFPS()
	app.DC.Push()
	app.DC.SetFontFace(app.FPSFontFace)
	app.DC.SetRGB(1, 1, 0)
	fpsText := fmt.Sprintf("FPS: %.1f", app.CurrentFPS)
	app.DC.DrawStringWrapped(fpsText, 540, 30, 0, 0, 100, 1.5, gg.AlignLeft)
	app.DC.Pop()
}
