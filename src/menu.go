package main

import (
	"fmt"
	"sort"

	"github.com/fogleman/gg"
)

// buildRootMenu creates the top-level Music menu
func (app *MiyooPod) buildRootMenu() *MenuScreen {
	root := &MenuScreen{
		Title: "MiyooPod",
	}

	items := []*MenuItem{}

	// Now Playing (only shown when something is playing)
	// This will be dynamically added/removed in refreshRootMenu

	// Playlists
	if len(app.Library.Playlists) > 0 {
		playlistMenu := &MenuScreen{
			Title:  "Playlists",
			Parent: root,
			Builder: func() []*MenuItem {
				return app.buildPlaylistMenuItems(root)
			},
		}
		items = append(items, &MenuItem{
			Label:      "Playlists",
			HasSubmenu: true,
			Submenu:    playlistMenu,
		})
	}

	// Artists
	if len(app.Library.Artists) > 0 {
		artistMenu := &MenuScreen{
			Title:  "Artists",
			Parent: root,
			Builder: func() []*MenuItem {
				return app.buildArtistMenuItems(root)
			},
		}
		items = append(items, &MenuItem{
			Label:      "Artists",
			HasSubmenu: true,
			Submenu:    artistMenu,
		})
	}

	// Albums
	if len(app.Library.Albums) > 0 {
		albumMenu := &MenuScreen{
			Title:  "Albums",
			Parent: root,
			Builder: func() []*MenuItem {
				return app.buildAlbumMenuItems(root)
			},
		}
		items = append(items, &MenuItem{
			Label:      "Albums",
			HasSubmenu: true,
			Submenu:    albumMenu,
		})
	}

	// Songs
	if len(app.Library.Tracks) > 0 {
		songMenu := &MenuScreen{
			Title:  "Songs",
			Parent: root,
			Builder: func() []*MenuItem {
				return app.buildSongMenuItems()
			},
		}
		items = append(items, &MenuItem{
			Label:      "Songs",
			HasSubmenu: true,
			Submenu:    songMenu,
		})

		// Shuffle All
		items = append(items, &MenuItem{
			Label: "Shuffle All",
			Action: func() {
				app.shuffleAllAndPlay()
			},
		})
	}

	// About
	items = append(items, &MenuItem{
		Label: "About",
		Action: func() {
			app.showAboutScreen()
		},
	})

	// Settings
	settingsMenu := &MenuScreen{
		Title:  "Settings",
		Parent: root,
		Builder: func() []*MenuItem {
			return app.buildSettingsMenuItems(root)
		},
	}
	items = append(items, &MenuItem{
		Label:      "Settings",
		HasSubmenu: true,
		Submenu:    settingsMenu,
	})

	// Scan Library
	items = append(items, &MenuItem{
		Label: "Scan Library",
		Action: func() {
			app.rescanLibrary()
		},
	})

	// Fetch Album Art
	items = append(items, &MenuItem{
		Label: "Fetch Album Art",
		Action: func() {
			app.scanAlbumArt()
		},
	})

	// Exit
	items = append(items, &MenuItem{
		Label: "Exit",
		Action: func() {
			audioStop()
			app.Running = false
		},
	})

	root.Items = items
	root.Built = true
	return root
}

func (app *MiyooPod) buildPlaylistMenuItems(root *MenuScreen) []*MenuItem {
	items := make([]*MenuItem, 0, len(app.Library.Playlists))
	for _, pl := range app.Library.Playlists {
		playlist := pl // capture
		trackMenu := &MenuScreen{
			Title:  pl.Name,
			Parent: root,
			Builder: func() []*MenuItem {
				return app.buildTrackMenuItems(playlist.Tracks)
			},
		}
		items = append(items, &MenuItem{
			Label:      pl.Name,
			HasSubmenu: true,
			Submenu:    trackMenu,
		})
	}
	return items
}

func (app *MiyooPod) buildArtistMenuItems(root *MenuScreen) []*MenuItem {
	items := make([]*MenuItem, 0, len(app.Library.Artists))
	for _, artist := range app.Library.Artists {
		a := artist // capture
		albumMenu := &MenuScreen{
			Title:  a.Name,
			Parent: root,
			Builder: func() []*MenuItem {
				albumItems := make([]*MenuItem, 0, len(a.Albums))
				for _, album := range a.Albums {
					alb := album // capture
					trackMenu := &MenuScreen{
						Title:  alb.Name,
						Parent: root,
						Builder: func() []*MenuItem {
							return app.buildTrackMenuItemsWithNumbers(alb.Tracks, true)
						},
					}
					albumItems = append(albumItems, &MenuItem{
						Label:      alb.Name,
						HasSubmenu: true,
						Submenu:    trackMenu,
						Album:      alb, // Store album reference for preview
					})
				}
				return albumItems
			},
		}
		items = append(items, &MenuItem{
			Label:      a.Name,
			HasSubmenu: true,
			Submenu:    albumMenu,
			Artist:     a, // Store artist reference for Y-key action
		})
	}
	return items
}

func (app *MiyooPod) buildAlbumMenuItems(root *MenuScreen) []*MenuItem {
	items := make([]*MenuItem, 0, len(app.Library.Albums))
	for _, album := range app.Library.Albums {
		alb := album // capture
		trackMenu := &MenuScreen{
			Title:  alb.Name + " - " + alb.Artist,
			Parent: root,
			Builder: func() []*MenuItem {
				return app.buildTrackMenuItemsWithNumbers(alb.Tracks, true)
			},
		}
		items = append(items, &MenuItem{
			Label:      alb.Name + " - " + alb.Artist,
			HasSubmenu: true,
			Submenu:    trackMenu,
			Album:      alb, // Store album reference for preview
		})
	}
	return items
}

func (app *MiyooPod) buildSongMenuItems() []*MenuItem {
	tracks := make([]*Track, len(app.Library.Tracks))
	copy(tracks, app.Library.Tracks)
	sort.Slice(tracks, func(i, j int) bool {
		return tracks[i].Title < tracks[j].Title
	})

	return app.buildTrackMenuItems(tracks)
}

func (app *MiyooPod) buildThemeMenuItems(root *MenuScreen) []*MenuItem {
	themes := AllThemes()
	items := make([]*MenuItem, 0, len(themes))

	for _, theme := range themes {
		t := theme // capture
		items = append(items, &MenuItem{
			Label: t.Name,
			Action: func() {
				app.setTheme(t)
			},
		})
	}

	return items
}

func (app *MiyooPod) buildSettingsMenuItems(root *MenuScreen) []*MenuItem {
	items := []*MenuItem{}

	// Themes submenu
	themesMenu := &MenuScreen{
		Title:  "Themes",
		Parent: root,
		Builder: func() []*MenuItem {
			return app.buildThemeMenuItems(root)
		},
	}
	items = append(items, &MenuItem{
		Label:      "Themes",
		HasSubmenu: true,
		Submenu:    themesMenu,
	})

	// Lock Key option
	lockKeyName := app.getLockKeyName()
	items = append(items, &MenuItem{
		Label: "Lock Key: " + lockKeyName,
		Action: func() {
			app.cycleLockKey()
		},
	})

	// Local Logs option
	localLogStatus := "Off"
	if app.LocalLogsEnabled {
		localLogStatus = "On"
	}
	items = append(items, &MenuItem{
		Label: "Local Logs: " + localLogStatus,
		Action: func() {
			app.toggleLocalLogs()
		},
	})

	// Developer Logs (Sentry) option
	sentryStatus := "Off"
	if app.SentryEnabled {
		sentryStatus = "On"
	}
	items = append(items, &MenuItem{
		Label: "Developer Logs: " + sentryStatus,
		Action: func() {
			app.toggleSentry()
		},
	})

	return items
}

func (app *MiyooPod) buildTrackMenuItems(tracks []*Track) []*MenuItem {
	return app.buildTrackMenuItemsWithNumbers(tracks, false)
}

func (app *MiyooPod) buildTrackMenuItemsWithNumbers(tracks []*Track, showTrackNum bool) []*MenuItem {
	// Sort by disc/track number if showing track numbers (album view)
	if showTrackNum {
		tracksToSort := make([]*Track, len(tracks))
		copy(tracksToSort, tracks)
		sort.Slice(tracksToSort, func(i, j int) bool {
			if tracksToSort[i].DiscNum != tracksToSort[j].DiscNum {
				return tracksToSort[i].DiscNum < tracksToSort[j].DiscNum
			}
			return tracksToSort[i].TrackNum < tracksToSort[j].TrackNum
		})
		tracks = tracksToSort
	}

	items := make([]*MenuItem, 0, len(tracks))
	for i, track := range tracks {
		t := track   // capture
		idx := i     // capture
		ts := tracks // capture

		label := t.Title
		if showTrackNum && t.TrackNum > 0 {
			label = fmt.Sprintf("%d. %s", t.TrackNum, t.Title)
		}

		items = append(items, &MenuItem{
			Label: label,
			Track: t,
			Action: func() {
				app.playTrackFromList(ts, idx)
			},
		})
	}
	return items
}

// handleMenuKey processes key input when on the menu screen
func (app *MiyooPod) handleMenuKey(key Key) {
	if len(app.MenuStack) == 0 {
		return
	}

	current := app.MenuStack[len(app.MenuStack)-1]

	// Ensure the menu is built
	if !current.Built && current.Builder != nil {
		current.Items = current.Builder()
		current.Built = true
	}

	switch key {
	case UP:
		if current.SelIndex > 0 {
			current.SelIndex--
		} else if len(current.Items) > 0 {
			// Wrap to bottom
			current.SelIndex = len(current.Items) - 1
		}
		current.adjustScroll()
	case DOWN:
		if current.SelIndex < len(current.Items)-1 {
			current.SelIndex++
		} else if len(current.Items) > 0 {
			// Wrap to top
			current.SelIndex = 0
		}
		current.adjustScroll()
	case RIGHT, A:
		if len(current.Items) == 0 {
			return
		}
		item := current.Items[current.SelIndex]
		if item.Submenu != nil {
			if !item.Submenu.Built && item.Submenu.Builder != nil {
				item.Submenu.Items = item.Submenu.Builder()
				item.Submenu.Built = true
			}
			item.Submenu.Parent = current
			app.MenuStack = append(app.MenuStack, item.Submenu)
			// Track menu navigation with title as screen name
			screenName := item.Submenu.Title
			if screenName == "MiyooPod" || screenName == "" {
				screenName = "home"
			}
			TrackPageView(screenName, map[string]interface{}{
				"menu_depth": len(app.MenuStack),
			})
		} else if item.Action != nil {
			item.Action()
		}
	case LEFT, B:
		if len(app.MenuStack) > 1 {
			app.MenuStack = app.MenuStack[:len(app.MenuStack)-1]
			// Track navigation back
			if len(app.MenuStack) > 0 {
				current := app.MenuStack[len(app.MenuStack)-1]
				screenName := current.Title
				if screenName == "MiyooPod" || screenName == "" {
					screenName = "home"
				}
				TrackPageView(screenName, map[string]interface{}{
					"menu_depth": len(app.MenuStack),
					"action":     "back",
				})
			}
		}
	case MENU:
		if len(app.MenuStack) > 1 {
			app.MenuStack = app.MenuStack[:1]
		} else {
			app.Running = false
			return
		}
	case Y:
		// Add to queue: track, album, or all artist tracks
		if len(current.Items) == 0 {
			return
		}
		item := current.Items[current.SelIndex]
		if item.Track != nil {
			app.addToQueue(item.Track)
		} else if item.Album != nil {
			// Add all album tracks
			for _, track := range item.Album.Tracks {
				app.addToQueue(track)
			}
		} else if item.Artist != nil {
			// Add all tracks from all albums by this artist
			for _, album := range item.Artist.Albums {
				for _, track := range album.Tracks {
					app.addToQueue(track)
				}
			}
		}
	}

	app.drawCurrentScreen()
}

// adjustScroll ensures the selected item is visible
func (ms *MenuScreen) adjustScroll() {
	if ms.SelIndex < ms.ScrollOff {
		ms.ScrollOff = ms.SelIndex
	}
	if ms.SelIndex >= ms.ScrollOff+VISIBLE_ITEMS {
		ms.ScrollOff = ms.SelIndex - VISIBLE_ITEMS + 1
	}
}

// drawMenuScreen renders the current menu
func (app *MiyooPod) drawMenuScreen() {
	if len(app.MenuStack) == 0 {
		return
	}

	current := app.MenuStack[len(app.MenuStack)-1]

	// Ensure the menu is built
	if !current.Built && current.Builder != nil {
		current.Items = current.Builder()
		current.Built = true
	}

	// Check if this is an album list (has albums in menu items)
	isAlbumList := len(current.Items) > 0 && current.Items[0].Album != nil

	if isAlbumList {
		app.drawAlbumListWithPreview(current)
	} else {
		app.drawStandardMenu(current)
	}
}

// drawStandardMenu renders a normal menu without preview panel
func (app *MiyooPod) drawStandardMenu(current *MenuScreen) {
	dc := app.DC

	// Background
	dc.SetHexColor(app.CurrentTheme.BG)
	dc.Clear()

	// Header
	app.drawHeader(current.Title)

	// Menu items
	if len(current.Items) == 0 {
		dc.SetFontFace(app.FontMenu)
		dc.SetHexColor(app.CurrentTheme.Dim)
		dc.DrawStringWrapped("No items", SCREEN_WIDTH/2, SCREEN_HEIGHT/2, 0.5, 0.5, 400, 1.5, gg.AlignCenter)
		return
	}

	endIdx := current.ScrollOff + VISIBLE_ITEMS
	if endIdx > len(current.Items) {
		endIdx = len(current.Items)
	}

	for i := current.ScrollOff; i < endIdx; i++ {
		item := current.Items[i]
		y := MENU_TOP_Y + (i-current.ScrollOff)*MENU_ITEM_HEIGHT
		selected := i == current.SelIndex
		isPlaying := item.Track != nil && app.Playing != nil && app.Playing.Track != nil && item.Track.Path == app.Playing.Track.Path
		isInQueue := item.Track != nil && app.isTrackInQueue(item.Track)

		app.drawMenuItem(y, item.Label, selected, item.HasSubmenu, isPlaying, isInQueue)
	}

	// Scroll bar
	app.drawScrollBar(len(current.Items), current.ScrollOff, VISIBLE_ITEMS)
}

// drawAlbumListWithPreview renders the album list with preview panel on the right
func (app *MiyooPod) drawAlbumListWithPreview(current *MenuScreen) {
	dc := app.DC

	// Background
	dc.SetHexColor(app.CurrentTheme.BG)
	dc.Clear()

	// Header
	app.drawHeader(current.Title)

	if len(current.Items) == 0 {
		dc.SetFontFace(app.FontMenu)
		dc.SetHexColor(app.CurrentTheme.Dim)
		dc.DrawStringWrapped("No items", SCREEN_WIDTH/2, SCREEN_HEIGHT/2, 0.5, 0.5, 400, 1.5, gg.AlignCenter)
		return
	}

	// Split screen: 70% for list, 30% for preview
	listWidth := int(float64(SCREEN_WIDTH) * 0.7)
	previewX := listWidth + 10

	// Draw menu items (left half)
	endIdx := current.ScrollOff + VISIBLE_ITEMS
	if endIdx > len(current.Items) {
		endIdx = len(current.Items)
	}

	for i := current.ScrollOff; i < endIdx; i++ {
		item := current.Items[i]
		y := MENU_TOP_Y + (i-current.ScrollOff)*MENU_ITEM_HEIGHT
		selected := i == current.SelIndex

		// Draw item in left half only
		if selected {
			dc.SetHexColor(app.CurrentTheme.SelBG)
			dc.DrawRectangle(0, float64(y), float64(listWidth), MENU_ITEM_HEIGHT)
			dc.Fill()
		}

		dc.SetFontFace(app.FontMenu)
		if selected {
			dc.SetHexColor(app.CurrentTheme.SelTxt)
		} else {
			dc.SetHexColor(app.CurrentTheme.ItemTxt)
		}

		// Truncate text to fit left panel
		maxWidth := float64(listWidth - 30)
		displayText := app.truncateText(item.Label, maxWidth, app.FontMenu)
		dc.DrawString(displayText, 15, float64(y+MENU_ITEM_HEIGHT/2+6))

		if item.HasSubmenu {
			dc.DrawString(">", float64(listWidth-20), float64(y+MENU_ITEM_HEIGHT/2+6))
		}
	}

	// Draw vertical separator
	dc.SetHexColor(app.CurrentTheme.Dim)
	dc.SetLineWidth(1)
	dc.DrawLine(float64(listWidth), HEADER_HEIGHT, float64(listWidth), SCREEN_HEIGHT)
	dc.Stroke()

	// Draw scroll bar for list
	if len(current.Items) > VISIBLE_ITEMS {
		scrollBarHeight := float64(SCREEN_HEIGHT-HEADER_HEIGHT) * float64(VISIBLE_ITEMS) / float64(len(current.Items))
		scrollBarY := HEADER_HEIGHT + float64(SCREEN_HEIGHT-HEADER_HEIGHT)*float64(current.ScrollOff)/float64(len(current.Items))

		dc.SetHexColor(app.CurrentTheme.Dim)
		dc.DrawRectangle(float64(listWidth-5), scrollBarY, 3, scrollBarHeight)
		dc.Fill()
	}

	// Draw album preview (right half)
	if current.SelIndex >= 0 && current.SelIndex < len(current.Items) {
		selectedItem := current.Items[current.SelIndex]
		if selectedItem.Album != nil {
			app.drawAlbumPreview(selectedItem.Album, previewX, HEADER_HEIGHT+10, SCREEN_WIDTH-previewX-10)
		}
	}
}

// drawAlbumPreview draws album art and info in the preview panel
func (app *MiyooPod) drawAlbumPreview(album *Album, x, y, width int) {
	dc := app.DC

	centerX := x + width/2
	yPos := y

	// Draw album art
	artSize := 200
	if artSize > width-20 {
		artSize = width - 20
	}

	coverImg := app.getCachedCover(album, artSize)
	if coverImg == nil {
		// Use default art
		coverImg = app.DefaultArt
	}

	artX := centerX - artSize/2
	app.fastBlitImage(coverImg, artX, yPos)

	// Border around art
	dc.SetHexColor(app.CurrentTheme.Dim)
	dc.SetLineWidth(1)
	dc.DrawRectangle(float64(artX), float64(yPos), float64(artSize), float64(artSize))
	dc.Stroke()

	yPos += artSize + 30

	// Album name
	dc.SetFontFace(app.FontMenu)
	dc.SetHexColor(app.CurrentTheme.ItemTxt)
	albumText := app.truncateText(album.Name, float64(width-20), app.FontMenu)
	dc.DrawStringAnchored(albumText, float64(centerX), float64(yPos), 0.5, 0)
	yPos += 25

	// Artist
	dc.SetFontFace(app.FontArtist)
	dc.SetHexColor(app.CurrentTheme.Dim)
	artistText := app.truncateText(album.Artist, float64(width-20), app.FontArtist)
	dc.DrawStringAnchored(artistText, float64(centerX), float64(yPos), 0.5, 0)
	yPos += 30

	// Get year from first track
	year := 0
	if len(album.Tracks) > 0 && album.Tracks[0] != nil {
		year = album.Tracks[0].Year
	}

	// Info: tracks and year
	dc.SetFontFace(app.FontSmall)
	dc.SetHexColor(app.CurrentTheme.ItemTxt)

	trackText := fmt.Sprintf("%d track", len(album.Tracks))
	if len(album.Tracks) != 1 {
		trackText += "s"
	}

	if year > 0 {
		dc.DrawStringAnchored(fmt.Sprintf("%s • %d", trackText, year), float64(centerX), float64(yPos), 0.5, 0)
	} else {
		dc.DrawStringAnchored(trackText, float64(centerX), float64(yPos), 0.5, 0)
	}
}

// refreshRootMenu updates the root menu to include/exclude Now Playing
func (app *MiyooPod) refreshRootMenu() {
	if app.RootMenu == nil {
		return
	}

	hasNowPlaying := false
	for _, item := range app.RootMenu.Items {
		if item.Label == "Now Playing" {
			hasNowPlaying = true
			break
		}
	}

	isPlaying := app.Playing != nil && app.Playing.State != StateStopped

	if isPlaying && !hasNowPlaying {
		// Insert Now Playing at the top
		nowPlayingItem := &MenuItem{
			Label: "Now Playing",
			Action: func() {
				app.setScreen(ScreenNowPlaying)
				app.drawCurrentScreen()
			},
		}
		app.RootMenu.Items = append([]*MenuItem{nowPlayingItem}, app.RootMenu.Items...)
		// Adjust selection if needed
		if app.RootMenu.SelIndex >= 0 {
			app.RootMenu.SelIndex++
		}
	} else if !isPlaying && hasNowPlaying {
		// Remove Now Playing
		for i, item := range app.RootMenu.Items {
			if item.Label == "Now Playing" {
				app.RootMenu.Items = append(app.RootMenu.Items[:i], app.RootMenu.Items[i+1:]...)
				// Adjust selection to prevent going negative
				if app.RootMenu.SelIndex > i {
					app.RootMenu.SelIndex--
				} else if app.RootMenu.SelIndex == i {
					// If Now Playing was selected, move to first item
					app.RootMenu.SelIndex = 0
				}
				// Ensure selection is valid
				if app.RootMenu.SelIndex < 0 {
					app.RootMenu.SelIndex = 0
				}
				if app.RootMenu.SelIndex >= len(app.RootMenu.Items) {
					app.RootMenu.SelIndex = len(app.RootMenu.Items) - 1
				}
				break
			}
		}
	}
}

// rescanLibrary performs a full library scan and rebuilds the menu
func (app *MiyooPod) rescanLibrary() {
	// Stop any current playback
	if app.Playing.State == StatePlaying {
		audioStop()
		app.Playing.State = StateStopped
	}

	// Perform the scan
	app.ScanLibrary()

	// Rebuild the root menu with the new library
	app.RootMenu = app.buildRootMenu()
	app.MenuStack = []*MenuScreen{app.RootMenu}

	// Redraw the screen
	app.drawCurrentScreen()
}

// getLockKeyName returns the display name of the current lock key
func (app *MiyooPod) getLockKeyName() string {
	switch app.LockKey {
	case Y:
		return "Y"
	case X:
		return "X"
	case SELECT:
		return "SELECT"
	case MENU:
		return "MENU"
	case L2:
		return "L2"
	case R2:
		return "R2"
	default:
		return "Y"
	}
}

// cycleLockKey cycles to the next available lock key
func (app *MiyooPod) cycleLockKey() {
	switch app.LockKey {
	case Y:
		app.LockKey = X
	case X:
		app.LockKey = SELECT
	case SELECT:
		app.LockKey = MENU
	case MENU:
		app.LockKey = L2
	case L2:
		app.LockKey = R2
	case R2:
		app.LockKey = Y
	default:
		app.LockKey = Y
	}

	// Rebuild the settings menu to update the label
	app.RootMenu = app.buildRootMenu()
	app.MenuStack = []*MenuScreen{app.RootMenu}

	// Navigate to settings menu
	for _, item := range app.RootMenu.Items {
		if item.Label == "Settings" {
			app.MenuStack = append(app.MenuStack, item.Submenu)
			break
		}
	}

	app.drawCurrentScreen()

	// Save lock key preference to settings file (fast)
	if err := app.saveSettings(); err != nil {
		logMsg(fmt.Sprintf("ERROR: Failed to save lock key preference: %v", err))
	}
}
