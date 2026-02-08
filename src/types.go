package main

import (
	"image"
	"time"

	"github.com/fogleman/gg"
	"golang.org/x/image/font"
)

// App metadata
const (
	APP_VERSION = "0.0.1"
	APP_AUTHOR  = "Danilo Fragoso"
	DONATE_URL  = "https://github.com/danfragoso/miyoopod"
)

// Display constants - identical to GaugeBoy
const SCREEN_WIDTH = 640
const SCREEN_HEIGHT = 480

// Music source
const MUSIC_ROOT = "/mnt/SDCARD/Media/Music/"
const LIBRARY_JSON_PATH = "/mnt/SDCARD/Media/Music/.miyoopod_library.json"
const ARTWORK_DIR = "/mnt/SDCARD/Media/Music/.miyoopod_artwork/"

// UI layout constants (at 640x480 native resolution)
const (
	HEADER_HEIGHT     = 40
	STATUS_BAR_HEIGHT = 36
	MENU_ITEM_HEIGHT  = 44
	MENU_TOP_Y        = 44
	MENU_LEFT_PAD     = 16
	MENU_RIGHT_PAD    = 16
	ARROW_RIGHT_X     = 610
	VISIBLE_ITEMS     = 9 // (480 - 40 header - 36 status bar) / 44 = 9
	SCROLL_BAR_WIDTH  = 6
	PROGRESS_BAR_Y    = 400
	PROGRESS_BAR_H    = 8
)

// Font sizes at native resolution
const (
	FONT_SIZE_HEADER = 22.0
	FONT_SIZE_MENU   = 24.0
	FONT_SIZE_TITLE  = 26.0
	FONT_SIZE_ARTIST = 22.0
	FONT_SIZE_ALBUM  = 20.0
	FONT_SIZE_TIME   = 18.0
	FONT_SIZE_SMALL  = 16.0
)

// Theme defines a color scheme
type Theme struct {
	Name      string
	BG        string
	HeaderBG  string
	HeaderTxt string
	ItemTxt   string
	SelBG     string
	SelTxt    string
	Arrow     string
	Accent    string
	Dim       string
	Progress  string
	ProgBG    string
}

// Available themes
var (
	ThemeClassic = Theme{
		Name:      "Classic iPod",
		BG:        "#B8B8B8",
		HeaderBG:  "#A0A0A0",
		HeaderTxt: "#000000",
		ItemTxt:   "#000000",
		SelBG:     "#4A90E2",
		SelTxt:    "#FFFFFF",
		Arrow:     "#666666",
		Accent:    "#4A90E2",
		Dim:       "#808080",
		Progress:  "#4A90E2",
		ProgBG:    "#909090",
	}

	ThemeDarkBlue = Theme{
		Name:      "Dark Blue",
		BG:        "#1A1A2E",
		HeaderBG:  "#16213E",
		HeaderTxt: "#E0E0E0",
		ItemTxt:   "#FFFFFF",
		SelBG:     "#0F3460",
		SelTxt:    "#FFFFFF",
		Arrow:     "#888888",
		Accent:    "#5390D9",
		Dim:       "#666666",
		Progress:  "#5390D9",
		ProgBG:    "#333333",
	}

	ThemeDark = Theme{
		Name:      "Dark",
		BG:        "#1C1C1C",
		HeaderBG:  "#0A0A0A",
		HeaderTxt: "#FFFFFF",
		ItemTxt:   "#FFFFFF",
		SelBG:     "#333333",
		SelTxt:    "#FFFFFF",
		Arrow:     "#666666",
		Accent:    "#FFFFFF",
		Dim:       "#777777",
		Progress:  "#FFFFFF",
		ProgBG:    "#444444",
	}

	ThemeGreen = Theme{
		Name:      "Matrix Green",
		BG:        "#0D0D0D",
		HeaderBG:  "#001100",
		HeaderTxt: "#00FF41",
		ItemTxt:   "#00FF41",
		SelBG:     "#003300",
		SelTxt:    "#00FF41",
		Arrow:     "#006600",
		Accent:    "#00FF41",
		Dim:       "#004400",
		Progress:  "#00FF41",
		ProgBG:    "#002200",
	}

	ThemeRetro = Theme{
		Name:      "Retro Amber",
		BG:        "#1A0F00",
		HeaderBG:  "#0F0800",
		HeaderTxt: "#FFAA00",
		ItemTxt:   "#FFAA00",
		SelBG:     "#332200",
		SelTxt:    "#FFAA00",
		Arrow:     "#885500",
		Accent:    "#FFAA00",
		Dim:       "#664400",
		Progress:  "#FFAA00",
		ProgBG:    "#442200",
	}

	ThemePurple = Theme{
		Name:      "Purple Haze",
		BG:        "#1A0A2E",
		HeaderBG:  "#0F0520",
		HeaderTxt: "#E0C0FF",
		ItemTxt:   "#E0C0FF",
		SelBG:     "#4A2070",
		SelTxt:    "#FFFFFF",
		Arrow:     "#8844AA",
		Accent:    "#AA66FF",
		Dim:       "#6633AA",
		Progress:  "#AA66FF",
		ProgBG:    "#331155",
	}

	ThemeLight = Theme{
		Name:      "Light",
		BG:        "#FFFFFF",
		HeaderBG:  "#F0F0F0",
		HeaderTxt: "#000000",
		ItemTxt:   "#000000",
		SelBG:     "#007AFF",
		SelTxt:    "#FFFFFF",
		Arrow:     "#999999",
		Accent:    "#007AFF",
		Dim:       "#888888",
		Progress:  "#007AFF",
		ProgBG:    "#DDDDDD",
	}

	ThemeNord = Theme{
		Name:      "Nord",
		BG:        "#2E3440",
		HeaderBG:  "#3B4252",
		HeaderTxt: "#ECEFF4",
		ItemTxt:   "#ECEFF4",
		SelBG:     "#5E81AC",
		SelTxt:    "#ECEFF4",
		Arrow:     "#4C566A",
		Accent:    "#88C0D0",
		Dim:       "#4C566A",
		Progress:  "#88C0D0",
		ProgBG:    "#434C5E",
	}

	ThemeSolarized = Theme{
		Name:      "Solarized Dark",
		BG:        "#002B36",
		HeaderBG:  "#073642",
		HeaderTxt: "#93A1A1",
		ItemTxt:   "#839496",
		SelBG:     "#586E75",
		SelTxt:    "#FDF6E3",
		Arrow:     "#657B83",
		Accent:    "#2AA198",
		Dim:       "#586E75",
		Progress:  "#268BD2",
		ProgBG:    "#073642",
	}

	ThemeCyberPunk = Theme{
		Name:      "Cyberpunk",
		BG:        "#0A0E27",
		HeaderBG:  "#1A1F3A",
		HeaderTxt: "#00FFFF",
		ItemTxt:   "#FF00FF",
		SelBG:     "#FF1493",
		SelTxt:    "#FFFFFF",
		Arrow:     "#00CED1",
		Accent:    "#00FFFF",
		Dim:       "#4B0082",
		Progress:  "#FF00FF",
		ProgBG:    "#2A2F4A",
	}

	ThemeCoffee = Theme{
		Name:      "Coffee",
		BG:        "#2C1810",
		HeaderBG:  "#1A0F08",
		HeaderTxt: "#E8D4B8",
		ItemTxt:   "#D4A574",
		SelBG:     "#6F4E37",
		SelTxt:    "#FFFFFF",
		Arrow:     "#8B6F47",
		Accent:    "#C1986B",
		Dim:       "#5C4033",
		Progress:  "#D2B48C",
		ProgBG:    "#4A3528",
	}

	ThemeOcean = Theme{
		Name:      "Ocean",
		BG:        "#001F3F",
		HeaderBG:  "#001529",
		HeaderTxt: "#7FDBFF",
		ItemTxt:   "#B8E6F5",
		SelBG:     "#0074D9",
		SelTxt:    "#FFFFFF",
		Arrow:     "#39CCCC",
		Accent:    "#00D4FF",
		Dim:       "#336B87",
		Progress:  "#00D4FF",
		ProgBG:    "#003D5C",
	}

	ThemeForest = Theme{
		Name:      "Forest",
		BG:        "#0F2027",
		HeaderBG:  "#1A3A2E",
		HeaderTxt: "#C5E1A5",
		ItemTxt:   "#A5D6A7",
		SelBG:     "#2E7D32",
		SelTxt:    "#FFFFFF",
		Arrow:     "#558B2F",
		Accent:    "#76FF03",
		Dim:       "#4A6B3F",
		Progress:  "#8BC34A",
		ProgBG:    "#1B5E20",
	}

	ThemeSunset = Theme{
		Name:      "Sunset",
		BG:        "#1A0A0A",
		HeaderBG:  "#2A1A1A",
		HeaderTxt: "#FFD700",
		ItemTxt:   "#FFA500",
		SelBG:     "#DC143C",
		SelTxt:    "#FFFFFF",
		Arrow:     "#FF6347",
		Accent:    "#FF4500",
		Dim:       "#8B4513",
		Progress:  "#FF8C00",
		ProgBG:    "#4A2A2A",
	}

	ThemeNeon = Theme{
		Name:      "Neon",
		BG:        "#000000",
		HeaderBG:  "#0A0A0A",
		HeaderTxt: "#FF10F0",
		ItemTxt:   "#00FF00",
		SelBG:     "#330033",
		SelTxt:    "#FFFFFF",
		Arrow:     "#FF00FF",
		Accent:    "#00FFFF",
		Dim:       "#444444",
		Progress:  "#FF10F0",
		ProgBG:    "#1A1A1A",
	}

	ThemeMidnight = Theme{
		Name:      "Midnight",
		BG:        "#0C0C1E",
		HeaderBG:  "#1A1A3E",
		HeaderTxt: "#C8C8FF",
		ItemTxt:   "#B0B0E8",
		SelBG:     "#3A3A6E",
		SelTxt:    "#FFFFFF",
		Arrow:     "#5A5A8E",
		Accent:    "#6E6EFF",
		Dim:       "#4A4A7E",
		Progress:  "#8080FF",
		ProgBG:    "#2A2A5E",
	}

	ThemeGruvbox = Theme{
		Name:      "Gruvbox",
		BG:        "#282828",
		HeaderBG:  "#1D2021",
		HeaderTxt: "#EBDBB2",
		ItemTxt:   "#EBDBB2",
		SelBG:     "#504945",
		SelTxt:    "#FBF1C7",
		Arrow:     "#665C54",
		Accent:    "#FABD2F",
		Dim:       "#7C6F64",
		Progress:  "#83A598",
		ProgBG:    "#3C3836",
	}

	ThemeCandy = Theme{
		Name:      "Candy",
		BG:        "#FFE5F4",
		HeaderBG:  "#FFB3E6",
		HeaderTxt: "#8B008B",
		ItemTxt:   "#C71585",
		SelBG:     "#FF69B4",
		SelTxt:    "#FFFFFF",
		Arrow:     "#DA70D6",
		Accent:    "#FF1493",
		Dim:       "#BA55D3",
		Progress:  "#FF69B4",
		ProgBG:    "#FFC0E5",
	}
)

// AllThemes returns all available themes in order
func AllThemes() []Theme {
	return []Theme{
		ThemeClassic,
		ThemeDarkBlue,
		ThemeDark,
		ThemeGreen,
		ThemeRetro,
		ThemePurple,
		ThemeLight,
		ThemeNord,
		ThemeSolarized,
		ThemeCyberPunk,
		ThemeCoffee,
		ThemeOcean,
		ThemeForest,
		ThemeSunset,
		ThemeNeon,
		ThemeMidnight,
		ThemeGruvbox,
		ThemeCandy,
	}
}

// Coverflow constants (at 640x480 native resolution)
const (
	COVER_CENTER_SIZE = 280
	COVER_SIDE_SIZE   = 140
	COVER_FAR_SIZE    = 100
	COVER_CENTER_Y    = 60
	COVER_CENTER_X    = 30
	COVER_SIDE_OFFSET = 170
	COVER_SIDE_Y      = 100
	COVER_FAR_OFFSET  = 290
	COVER_FAR_Y       = 110
	REFLECT_HEIGHT    = 60
	VISIBLE_COVERS    = 5
)

// --- Playback state ---

type PlayState int

const (
	StateStopped PlayState = iota
	StatePlaying
	StatePaused
)

type RepeatMode int

const (
	RepeatOff RepeatMode = iota
	RepeatAll
	RepeatOne
)

// --- Music Library types ---

type Track struct {
	Path        string  `json:"path"`
	Title       string  `json:"title"`
	Artist      string  `json:"artist"`
	Album       string  `json:"album"`
	AlbumArtist string  `json:"album_artist"`
	TrackNum    int     `json:"track_num"`
	TrackTotal  int     `json:"track_total"`
	DiscNum     int     `json:"disc_num"`
	Year        int     `json:"year"`
	Genre       string  `json:"genre"`
	Duration    float64 `json:"duration"`
	HasArt      bool    `json:"has_art"`
}

type Album struct {
	Name    string      `json:"name"`
	Artist  string      `json:"artist"`
	Tracks  []*Track    `json:"-"` // Reconstructed from library tracks
	ArtData []byte      `json:"-"` // Don't store in JSON, only used temporarily during extraction
	ArtExt  string      `json:"-"`
	ArtPath string      `json:"artPath,omitempty"` // Path to saved artwork file (from MusicBrainz)
	ArtImg  image.Image `json:"-"`                 // Decoded at runtime
}

type Artist struct {
	Name   string   `json:"name"`
	Albums []*Album `json:"-"` // Reconstructed from library albums
}

type Playlist struct {
	Name   string   `json:"name"`
	Path   string   `json:"path"`
	Tracks []*Track `json:"-"` // Reconstructed from playlist file
}

type Library struct {
	Tracks    []*Track    `json:"tracks"`
	Albums    []*Album    `json:"albums"`
	Artists   []*Artist   `json:"artists"`
	Playlists []*Playlist `json:"playlists"`

	TracksByPath  map[string]*Track  `json:"-"` // Reconstructed on load
	AlbumsByKey   map[string]*Album  `json:"-"` // Reconstructed on load
	ArtistsByName map[string]*Artist `json:"-"` // Reconstructed on load
}

// --- Playback queue ---

type PlaybackQueue struct {
	Tracks       []*Track
	CurrentIndex int
	Shuffle      bool
	Repeat       RepeatMode
	ShuffleOrder []int
}

// --- Menu system ---

type ScreenType int

const (
	ScreenMenu ScreenType = iota
	ScreenNowPlaying
	ScreenQueue
)

type MenuItem struct {
	Label      string
	HasSubmenu bool
	Action     func()
	Submenu    *MenuScreen
	Track      *Track
	Album      *Album  // For album preview display
	Artist     *Artist // For artist track queuing
}

type MenuScreen struct {
	Title     string
	Items     []*MenuItem
	SelIndex  int
	ScrollOff int
	Parent    *MenuScreen
	Builder   func() []*MenuItem
	Built     bool
}

// --- Now Playing state ---

type NowPlaying struct {
	Track    *Track
	State    PlayState
	Position float64
	Duration float64
	Volume   float64
}

// --- Coverflow state ---

type CoverflowState struct {
	Albums      []*Album
	CenterIndex int
	CoverCache  map[string]image.Image
}

// --- Main application ---

type MiyooPod struct {
	Running       bool
	ShouldRefresh bool
	RefreshChan   chan struct{} // Signal channel for screen refresh

	// Display
	DC *gg.Context
	FB *image.RGBA

	// Fonts (pre-loaded, cached)
	FontHeader font.Face
	FontMenu   font.Face
	FontTitle  font.Face
	FontArtist font.Face
	FontAlbum  font.Face
	FontTime   font.Face
	FontSmall  font.Face

	// Theme
	CurrentTheme Theme

	// Music data
	Library *Library
	Queue   *PlaybackQueue
	Playing *NowPlaying

	// Navigation
	CurrentScreen ScreenType
	MenuStack     []*MenuScreen
	RootMenu      *MenuScreen

	// Coverflow
	Coverflow *CoverflowState

	// Default album art
	DefaultArt image.Image

	// Now Playing screen cache (pre-rendered background)
	NowPlayingBG *image.RGBA
	NPCacheDirty bool

	// Performance optimization: text measurement cache
	// Key: text+font.Face pointer, Value: width in pixels
	TextMeasureCache map[string]float64

	// Pre-rendered digit sprites for fast time display (bypass gg)
	Digits *DigitSprites

	// Key repeat state
	LastKeyTime time.Time
	LastKey     Key
	RepeatDelay time.Duration
	RepeatRate  time.Duration

	// Error popup state
	ErrorMessage string
	ErrorTime    time.Time

	// Screen lock state
	Locked               bool
	BrightnessBeforeLock int
	LastYTime            time.Time
	LockKey              Key // Which key is used for lock/unlock (default Y)

	// Queue view state
	QueueScrollOffset int // Scroll position for queue view

	// Album art status callback
	albumArtStatusFunc func(string)
	QueueSelectedIndex int // Selected track in queue view

	// Settings
	WriteLogsEnabled bool // Whether to write logs to file
}
