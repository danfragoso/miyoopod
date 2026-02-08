package main

/*
#cgo CFLAGS: -I/root/include/SDL2 -O2 -w -D_GNU_SOURCE=1 -D_REENTRANT
#cgo LDFLAGS: -L/root/lib -Wl,-rpath-link,/root/lib -Wl,-rpath,'$ORIGIN' -Wl,--unresolved-symbols=ignore-in-shared-libs -lSDL2 -lSDL2_mixer -lpthread
#include <stdlib.h>
#include "main.c"
#include "audio.c"
*/
import "C"

import (
	"fmt"
	"image"
	"os"
	"runtime"
	"time"
	"unsafe"

	"github.com/fogleman/gg"
)

func init() {
	// Use both Cortex-A7 cores so audio decode thread gets its own core
	runtime.GOMAXPROCS(2)
	// Pin main goroutine to a stable OS thread for CGO/SDL calls
	runtime.LockOSThread()
}

func (app *MiyooPod) Init() {
	// Set global reference for logger
	globalApp = app

	// Default: logs enabled
	app.WriteLogsEnabled = true

	logMsg("Initializing MiyooPod...")
	logMsg("SDL init...")
	if C.init() != 0 {
		logMsg("FATAL: SDL init failed!")
		return
	}
	logMsg("SDL init ok!")

	// Init audio
	logMsg("Audio init...")
	if C.audio_init() != 0 {
		logMsg("WARNING: Failed to init SDL2_mixer audio")
	} else {
		logMsg("Audio init ok!")
	}

	// Create render context at native resolution (640x480)
	app.DC = gg.NewContext(SCREEN_WIDTH, SCREEN_HEIGHT)
	app.FB, _ = app.DC.Image().(*image.RGBA)

	// Load fonts
	app.loadFonts()

	// Init state
	app.Running = true
	app.CurrentScreen = ScreenMenu
	app.CurrentTheme = ThemeClassic // Set default theme
	app.RepeatDelay = 300 * time.Millisecond
	app.RepeatRate = 80 * time.Millisecond
	app.Playing = &NowPlaying{State: StateStopped, Volume: 100}
	app.Queue = &PlaybackQueue{Repeat: RepeatOff}
	app.Coverflow = &CoverflowState{
		CoverCache: make(map[string]image.Image),
	}
	app.TextMeasureCache = make(map[string]float64)
	app.RefreshChan = make(chan struct{}, 1)
	app.LockKey = Y // Default lock key

	// Pre-render digit sprites for fast time display (bypass gg in hot path)
	app.initDigitSprites(app.FontTime)

	// Set initial volume (scale 0-100 to 0-128)
	audioSetVolume(int(app.Playing.Volume))

	// Load settings (theme and lock key) before showing splash - fast parse
	if err := app.loadSettings(); err != nil {
		logMsg(fmt.Sprintf("Could not load settings: %v (using defaults)", err))
	}

	// Draw splash screen with logo (now using restored theme if available)
	app.drawLogoSplash()

	// Check for updates in background
	versionStatus := app.checkVersion()
	app.drawLogoSplashWithVersion(versionStatus)

	// Give user time to see version status
	time.Sleep(1500 * time.Millisecond)

	// Generate initial icon PNG with current theme
	if err := app.generateIconPNG(); err != nil {
		logMsg(fmt.Sprintf("Failed to generate icon: %v", err))
	}

	logMsg("MiyooPod init OK!")
}

func (app *MiyooPod) loadFonts() {
	fontPath := "./assets/ui_font.ttf"

	var err error
	app.FontHeader, err = gg.LoadFontFace(fontPath, FONT_SIZE_HEADER)
	if err != nil {
		panic(fmt.Sprintf("Failed to load font: %v", err))
	}

	app.FontMenu, _ = gg.LoadFontFace(fontPath, FONT_SIZE_MENU)
	app.FontTitle, _ = gg.LoadFontFace(fontPath, FONT_SIZE_TITLE)
	app.FontArtist, _ = gg.LoadFontFace(fontPath, FONT_SIZE_ARTIST)
	app.FontAlbum, _ = gg.LoadFontFace(fontPath, FONT_SIZE_ALBUM)
	app.FontTime, _ = gg.LoadFontFace(fontPath, FONT_SIZE_TIME)
	app.FontSmall, _ = gg.LoadFontFace(fontPath, FONT_SIZE_SMALL)
}

func (app *MiyooPod) RunUI() {
	for range app.RefreshChan {
		if !app.Running {
			break
		}
		pixels := (*C.uchar)(unsafe.Pointer(&app.FB.Pix[0]))
		C.refreshScreenPtr(pixels)
	}
}

// triggerRefresh signals the UI goroutine to present the framebuffer.
// Non-blocking: if a refresh is already pending, this is a no-op.
func (app *MiyooPod) triggerRefresh() {
	select {
	case app.RefreshChan <- struct{}{}:
	default:
	}
}

func createApp() *MiyooPod {
	return &MiyooPod{
		Running: true,
		FB:      image.NewRGBA(image.Rect(0, 0, SCREEN_WIDTH, SCREEN_HEIGHT)),
	}
}

func main() {
	logMsg("\n\n\n-----------")
	logMsg("MiyooPod started!")

	app := createApp()
	app.Init()

	go app.RunUI()
	time.Sleep(1 * time.Second)

	// Load library from JSON or perform full scan
	err := app.loadLibraryJSON()
	if err != nil {
		logMsg(fmt.Sprintf("Could not load library from JSON: %v", err))
		logMsg("Performing full library scan...")
		app.ScanLibrary()
	}

	// Build menu
	app.RootMenu = app.buildRootMenu()
	app.MenuStack = []*MenuScreen{app.RootMenu}

	// Start playback poller
	go app.startPlaybackPoller()

	// Draw initial menu
	app.drawCurrentScreen()

	// Main loop: poll SDL events on main thread (required by SDL2)
	// SDL_PollEvent MUST run on the thread that called SDL_Init (LockOSThread in init)
	// Sleep between polls to keep CPU usage low (replaces old runtime.Gosched spin loop)
	for app.Running {
		key := Key(C_GetKeyPress())
		if key != NONE {
			app.handleKey(key)
		}
		time.Sleep(33 * time.Millisecond) // ~30Hz polling, main thread sleeps most of the time
	}

	// Cleanup: close refresh channel to unblock RunUI goroutine
	close(app.RefreshChan)
	C.audio_quit()
	C.quit()
}

func (app *MiyooPod) handleKey(key Key) {
	// If locked, only allow double-press of the lock key to unlock
	if app.Locked {
		if key == app.LockKey {
			now := time.Now()
			if now.Sub(app.LastYTime) < 500*time.Millisecond {
				// Double press detected - unlock
				app.toggleLock()
			}
			app.LastYTime = now
		}
		return
	}

	// Handle lock key double-press for locking when unlocked
	if key == app.LockKey {
		now := time.Now()
		if now.Sub(app.LastYTime) < 500*time.Millisecond {
			// Double press detected - lock
			app.toggleLock()
			return
		}
		app.LastYTime = now
		// Fall through to normal key handling
	}

	// Global keys (work from any screen)
	switch key {
	case START:
		// Go to Now Playing screen
		if app.Playing != nil && app.Playing.Track != nil {
			app.CurrentScreen = ScreenNowPlaying
			app.drawCurrentScreen()
		}
		return
	case L:
		app.prevTrack()
		app.drawCurrentScreen()
		return
	case R:
		app.nextTrack()
		app.drawCurrentScreen()
		return
	case SELECT:
		app.toggleShuffle()
		app.drawCurrentScreen()
		return
	}

	// Screen-specific keys
	switch app.CurrentScreen {
	case ScreenMenu:
		app.handleMenuKey(key)
	case ScreenNowPlaying:
		app.handleNowPlayingKey(key)
	case ScreenQueue:
		app.handleQueueKey(key)
	}
}

func (app *MiyooPod) handleNowPlayingKey(key Key) {
	switch key {
	case LEFT, B:
		app.CurrentScreen = ScreenMenu
		app.drawCurrentScreen()
	case RIGHT:
		// Show queue
		app.CurrentScreen = ScreenQueue
		app.QueueScrollOffset = 0
		app.QueueSelectedIndex = app.Queue.CurrentIndex // Start at currently playing track
		app.drawCurrentScreen()
	case A:
		// Toggle play/pause
		app.togglePlayPause()
		app.drawCurrentScreen()
	case X:
		// Cycle repeat mode
		app.cycleRepeat()
		app.drawCurrentScreen()
	}
}

func (app *MiyooPod) drawCurrentScreen() {
	switch app.CurrentScreen {
	case ScreenMenu:
		app.drawMenuScreen()
		app.drawStatusBar()
	case ScreenNowPlaying:
		// Status bar is included in the NowPlayingBG cache, skip drawing it separately
		app.drawNowPlayingScreen()
	case ScreenQueue:
		app.drawQueueScreen()
		app.drawStatusBar()
	}

	// Draw lock overlay if locked
	if app.Locked {
		app.drawLockOverlay()
	}

	// Draw error popup overlay if active
	app.drawErrorPopup()

	app.triggerRefresh()
}

func C_GetKeyPress() int {
	return int(C.pollEvents())
}

// Audio C wrappers

func audioLoadFile(path string) error {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	if C.audio_load(cpath) != 0 {
		return fmt.Errorf("failed to load audio: %s", path)
	}
	return nil
}

// audioLoadFileToMemory loads entire audio file into RAM to avoid SD card I/O during playback
func audioLoadFileToMemory(path string) error {
	// Read entire file into Go memory
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read audio file: %v", err)
	}

	// Allocate C memory and copy data
	cdata := C.malloc(C.size_t(len(data)))
	if cdata == nil {
		return fmt.Errorf("failed to allocate memory for audio")
	}

	// Copy Go bytes to C memory
	C.memcpy(cdata, unsafe.Pointer(&data[0]), C.size_t(len(data)))

	// audio_load_mem takes ownership of cdata, will free it on next load or quit
	if C.audio_load_mem(cdata, C.int(len(data))) != 0 {
		// audio_load_mem already freed cdata on error
		return fmt.Errorf("failed to load audio from memory")
	}

	return nil
}

func audioPlay() error {
	if C.audio_play() != 0 {
		return fmt.Errorf("failed to play audio")
	}
	return nil
}

func audioStop() {
	C.audio_stop()
}

func audioTogglePause() {
	C.audio_toggle_pause()
}

func audioPause() {
	C.audio_pause()
}

func audioResume() {
	C.audio_resume()
}

func audioIsPlaying() bool {
	return C.audio_is_playing() != 0
}

func audioIsPaused() bool {
	return C.audio_is_paused() != 0
}

func audioGetPosition() float64 {
	return float64(C.audio_get_position())
}

func audioGetDuration() float64 {
	return float64(C.audio_get_duration())
}

func audioGetDurationForFile(path string) float64 {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	return float64(C.audio_get_file_duration(cpath))
}

func audioSeek(position float64) {
	C.audio_seek(C.double(position))
}

func audioSetVolume(volume int) {
	C.audio_set_volume(C.int(volume * 128 / 100))
}

func audioCheckFinished() bool {
	return C.audio_check_finished() != 0
}

type AudioStateSnapshot struct {
	Position  float64
	Duration  float64
	IsPlaying bool
	IsPaused  bool
	Finished  bool
}

func audioFlushBuffers() {
	C.audio_flush_buffers()
}

func audioGetState() AudioStateSnapshot {
	var state C.AudioState
	C.audio_get_state(&state)
	return AudioStateSnapshot{
		Position:  float64(state.position),
		Duration:  float64(state.duration),
		IsPlaying: state.is_playing != 0,
		IsPaused:  state.is_paused != 0,
		Finished:  state.finished != 0,
	}
}
