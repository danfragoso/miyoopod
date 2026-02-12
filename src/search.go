package main

import (
	"strings"
)

// Search grid layout: 7 columns, 5 rows
// Row 0: A B C D E F G
// Row 1: H I J K L M N
// Row 2: O P Q R S T U
// Row 3: V W X Y Z _ .
// Row 4: 0 1 2 3 4 5 6 7 8 9
var searchGrid = [][]string{
	{"A", "B", "C", "D", "E", "F", "G"},
	{"H", "I", "J", "K", "L", "M", "N"},
	{"O", "P", "Q", "R", "S", "T", "U"},
	{"V", "W", "X", "Y", "Z", " ", "."},
	{"0", "1", "2", "3", "4", "5", "6"},
	{"7", "8", "9"},
}

const (
	searchGridCols   = 7
	searchGridRows   = 6
	searchPanelWidth = 200 // Width of the search panel on the right
)

// isSearchableMenu returns true if the current menu shows items that can be searched
// (Artists, Albums, Songs, or Playlists lists — not the root menu or settings)
func (app *MiyooPod) isSearchableMenu() bool {
	if len(app.MenuStack) < 2 {
		// Don't search the root menu
		return false
	}
	current := app.MenuStack[len(app.MenuStack)-1]
	if len(current.Items) == 0 {
		return false
	}
	first := current.Items[0]
	return first.Track != nil || first.Album != nil || first.Artist != nil
}

// toggleSearch activates or deactivates the search panel
func (app *MiyooPod) toggleSearch() {
	if app.SearchActive {
		// Panel is open — close it (keep filter)
		app.closeSearchPanel()
		return
	}

	if app.SearchAllItems != nil {
		// Panel was closed but filter still active — re-open the panel
		app.SearchActive = true
		return
	}

	if !app.isSearchableMenu() {
		return
	}

	current := app.MenuStack[len(app.MenuStack)-1]

	app.SearchActive = true
	app.SearchQuery = ""
	app.SearchGridRow = 0
	app.SearchGridCol = 0
	app.SearchMenuTitle = current.Title
	// Save original items for filtering
	app.SearchAllItems = make([]*MenuItem, len(current.Items))
	copy(app.SearchAllItems, current.Items)
}

// closeSearchPanel hides the search grid but keeps the filtered results.
// The user can then navigate the filtered list normally.
func (app *MiyooPod) closeSearchPanel() {
	app.SearchActive = false
	// Keep SearchAllItems and SearchQuery so we can restore later if needed
}

// cancelSearch fully exits search mode, restoring the original unfiltered list.
func (app *MiyooPod) cancelSearch() {
	if app.SearchAllItems == nil {
		app.SearchActive = false
		return
	}

	current := app.MenuStack[len(app.MenuStack)-1]
	current.Items = app.SearchAllItems
	current.Title = app.SearchMenuTitle
	current.SelIndex = 0
	current.ScrollOff = 0

	app.SearchActive = false
	app.SearchQuery = ""
	app.SearchAllItems = nil
}

// filterMenuItems filters the current menu items based on the search query
func (app *MiyooPod) filterMenuItems() {
	if len(app.MenuStack) == 0 || app.SearchAllItems == nil {
		return
	}

	current := app.MenuStack[len(app.MenuStack)-1]
	query := strings.ToLower(app.SearchQuery)

	if query == "" {
		current.Items = app.SearchAllItems
	} else {
		filtered := make([]*MenuItem, 0)
		for _, item := range app.SearchAllItems {
			label := strings.ToLower(item.Label)
			// Also search artist name for tracks
			artistMatch := false
			if item.Track != nil {
				artistMatch = strings.Contains(strings.ToLower(item.Track.Artist), query)
			}
			if item.Album != nil {
				artistMatch = strings.Contains(strings.ToLower(item.Album.Artist), query)
			}
			if strings.Contains(label, query) || artistMatch {
				filtered = append(filtered, item)
			}
		}
		current.Items = filtered
	}

	if app.SearchQuery != "" {
		current.Title = app.SearchMenuTitle + " \"" + app.SearchQuery + "\""
	} else {
		current.Title = app.SearchMenuTitle
	}

	current.SelIndex = 0
	current.ScrollOff = 0
}

// handleSearchKey processes key input when the search panel is active.
// Returns true if the key was consumed.
func (app *MiyooPod) handleSearchKey(key Key) bool {
	if !app.SearchActive {
		return false
	}

	switch key {
	case UP:
		app.SearchGridRow--
		if app.SearchGridRow < 0 {
			app.SearchGridRow = searchGridRows - 1
		}
		// Clamp column if the new row is shorter
		rowLen := len(searchGrid[app.SearchGridRow])
		if app.SearchGridCol >= rowLen {
			app.SearchGridCol = rowLen - 1
		}
	case DOWN:
		app.SearchGridRow++
		if app.SearchGridRow >= searchGridRows {
			app.SearchGridRow = 0
		}
		rowLen := len(searchGrid[app.SearchGridRow])
		if app.SearchGridCol >= rowLen {
			app.SearchGridCol = rowLen - 1
		}
	case LEFT:
		app.SearchGridCol--
		rowLen := len(searchGrid[app.SearchGridRow])
		if app.SearchGridCol < 0 {
			app.SearchGridCol = rowLen - 1
		}
	case RIGHT:
		app.SearchGridCol++
		rowLen := len(searchGrid[app.SearchGridRow])
		if app.SearchGridCol >= rowLen {
			app.SearchGridCol = 0
		}
	case A:
		// Add selected character to query
		char := searchGrid[app.SearchGridRow][app.SearchGridCol]
		app.SearchQuery += strings.ToLower(char)
		app.filterMenuItems()
	case X:
		// Delete last character
		if len(app.SearchQuery) > 0 {
			app.SearchQuery = app.SearchQuery[:len(app.SearchQuery)-1]
			app.filterMenuItems()
		}
	case B:
		// Close search panel, keep filtered results
		app.closeSearchPanel()
	default:
		return false
	}

	app.drawCurrentScreen()
	return true
}

// drawSearchPanel renders the search grid on the right side of the screen
func (app *MiyooPod) drawSearchPanel(panelX, panelY, panelW int) {
	dc := app.DC

	// Search input field at top
	inputY := panelY + 8
	inputH := 30
	inputX := panelX + 8
	inputW := panelW - 16

	// Input background
	dc.SetHexColor(app.CurrentTheme.ProgBG)
	dc.DrawRoundedRectangle(float64(inputX), float64(inputY), float64(inputW), float64(inputH), 4)
	dc.Fill()

	// Input text or placeholder
	dc.SetFontFace(app.FontMenu)
	if app.SearchQuery != "" {
		dc.SetHexColor(app.CurrentTheme.ItemTxt)
		queryText := app.truncateText(app.SearchQuery, float64(inputW-12), app.FontMenu)
		dc.DrawStringAnchored(queryText, float64(inputX+6), float64(inputY)+float64(inputH)/2, 0, 0.5)
	} else {
		dc.SetHexColor(app.CurrentTheme.Dim)
		dc.DrawStringAnchored("Search...", float64(inputX+6), float64(inputY)+float64(inputH)/2, 0, 0.5)
	}

	// Draw the character grid below the input
	gridStartY := inputY + inputH + 12
	cellW := (panelW - 16) / searchGridCols
	cellH := 32

	dc.SetFontFace(app.FontMenu)

	for row := 0; row < searchGridRows; row++ {
		for col := 0; col < len(searchGrid[row]); col++ {
			char := searchGrid[row][col]
			cx := panelX + 8 + col*cellW
			cy := gridStartY + row*cellH

			isSelected := row == app.SearchGridRow && col == app.SearchGridCol

			if isSelected {
				// Highlight selected cell
				dc.SetHexColor(app.CurrentTheme.SelBG)
				dc.DrawRoundedRectangle(float64(cx), float64(cy), float64(cellW), float64(cellH-2), 3)
				dc.Fill()
				dc.SetHexColor(app.CurrentTheme.SelTxt)
			} else {
				dc.SetHexColor(app.CurrentTheme.ItemTxt)
			}

			// Display character (show "␣" for space)
			displayChar := char
			if char == " " {
				displayChar = "_"
			}
			dc.DrawStringAnchored(displayChar, float64(cx)+float64(cellW)/2, float64(cy)+float64(cellH-2)/2, 0.5, 0.5)
		}
	}

	// Draw hints at bottom of panel — stacked vertically with button legend styling
	hintsY := gridStartY + searchGridRows*cellH + 8
	dc.SetFontFace(app.FontSmall)
	hintX := panelX + 8
	lineH := 24.0
	app.drawButtonLegend(hintX, float64(hintsY), "A", "Add Character")
	app.drawButtonLegend(hintX, float64(hintsY)+lineH, "X", "Delete")
	app.drawButtonLegend(hintX, float64(hintsY)+lineH*2, "B", "Close")
}
