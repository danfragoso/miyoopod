package main

import (
	"bytes"
	"fmt"
	"image"

	xdraw "golang.org/x/image/draw"
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

	app.Coverflow.CoverCacheMu.Lock()
	if cached, ok := app.Coverflow.CoverCache[key]; ok {
		app.Coverflow.CoverCacheMu.Unlock()
		return cached
	}
	app.Coverflow.CoverCacheMu.Unlock()

	// Fast path: try exact-size RGBA pixel cache from disk
	rgbaPath := app.rgbaCachePath(album)
	if rgbaPath != "" {
		if img := app.loadRGBACache(rgbaPath, size); img != nil {
			app.Coverflow.CoverCacheMu.Lock()
			app.Coverflow.CoverCache[key] = img
			app.Coverflow.CoverCacheMu.Unlock()
			return img
		}
		// Not the exact size — try loading the cached 200px version and downscale
		if size != COVER_CENTER_SIZE {
			if img := app.loadRGBACache(rgbaPath, COVER_CENTER_SIZE); img != nil {
				dst := image.NewRGBA(image.Rect(0, 0, size, size))
				xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), img, img.Bounds(), xdraw.Over, nil)
				app.Coverflow.CoverCacheMu.Lock()
				app.Coverflow.CoverCache[key] = dst
				app.Coverflow.CoverCacheMu.Unlock()
				return dst
			}
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
			app.Coverflow.CoverCacheMu.Lock()
			if cached, ok := app.Coverflow.CoverCache[defaultKey]; ok {
				app.Coverflow.CoverCache[key] = cached
				app.Coverflow.CoverCacheMu.Unlock()
				return cached
			}
			app.Coverflow.CoverCacheMu.Unlock()
			return app.DefaultArt
		}
	}

	// Cold path: full-res decode, runs once per album then cached forever
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	srcBounds := album.ArtImg.Bounds()
	xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), album.ArtImg, srcBounds, xdraw.Over, nil)

	app.Coverflow.CoverCacheMu.Lock()
	app.Coverflow.CoverCache[key] = dst
	app.Coverflow.CoverCacheMu.Unlock()

	// Save 200px RGBA cache for next startup (the canonical cached size)
	if rgbaPath != "" && size == COVER_CENTER_SIZE {
		app.saveRGBACache(rgbaPath, dst)
	}

	album.ArtImg = nil
	return dst
}
