package main

import (
	"bytes"
	"fmt"
	"image"

	"github.com/fogleman/gg"
)

// DrawCoverflow renders the current album art centered on the Now Playing screen
func (app *MiyooPod) DrawCoverflow() {
	cf := app.Coverflow
	if cf == nil || len(cf.Albums) == 0 {
		return
	}

	if cf.CenterIndex < 0 || cf.CenterIndex >= len(cf.Albums) {
		return
	}

	album := cf.Albums[cf.CenterIndex]
	coverImg := app.getCachedCover(album, COVER_CENTER_SIZE)
	if coverImg == nil {
		return
	}

	// Use fast blit instead of gg.DrawImage
	app.fastBlitImage(coverImg, COVER_CENTER_X, COVER_CENTER_Y)

	// Border
	dc := app.DC
	dc.SetRGBA(0.3, 0.3, 0.3, 0.5)
	dc.SetLineWidth(1)
	dc.DrawRectangle(float64(COVER_CENTER_X), float64(COVER_CENTER_Y), float64(COVER_CENTER_SIZE), float64(COVER_CENTER_SIZE))
	dc.Stroke()
}

// getCachedCover returns a resized cover image from cache.
func (app *MiyooPod) getCachedCover(album *Album, size int) image.Image {
	key := fmt.Sprintf("%s|%s_%d", album.Artist, album.Name, size)

	if cached, ok := app.Coverflow.CoverCache[key]; ok {
		return cached
	}

	// Fast path: try RGBA pixel cache from disk
	rgbaPath := app.rgbaCachePath(album)
	if rgbaPath != "" {
		if img := app.loadRGBACache(rgbaPath, size); img != nil {
			app.Coverflow.CoverCache[key] = img
			return img
		}
	}

	if album.ArtImg == nil {
		// Try loading from disk if we have a saved path
		if album.ArtPath != "" {
			if err := app.loadAlbumArtwork(album); err == nil {
				reader := bytes.NewReader(album.ArtData)
				if img, _, err := image.Decode(reader); err == nil {
					album.ArtImg = img
					album.ArtData = nil
				}
			}
		}

		// Still no image? Use default
		if album.ArtImg == nil {
			defaultKey := fmt.Sprintf("__default__%d", size)
			if cached, ok := app.Coverflow.CoverCache[defaultKey]; ok {
				app.Coverflow.CoverCache[key] = cached
				return cached
			}
			return app.DefaultArt
		}
	}

	// Fallback: resize on demand
	dc := gg.NewContext(size, size)
	srcBounds := album.ArtImg.Bounds()
	sx := float64(size) / float64(srcBounds.Dx())
	sy := float64(size) / float64(srcBounds.Dy())
	dc.Scale(sx, sy)
	dc.DrawImage(album.ArtImg, 0, 0)

	result := dc.Image()
	app.Coverflow.CoverCache[key] = result

	// Save RGBA cache for next startup
	if rgbaPath != "" {
		if rgba, ok := result.(*image.RGBA); ok {
			app.saveRGBACache(rgbaPath, rgba)
		}
	}

	album.ArtImg = nil
	return result
}
