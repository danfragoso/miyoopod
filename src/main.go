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

	// Default: local logs disabled
	app.LocalLogsEnabled = true
	app.SentryEnabled = true // Default: developer logs enabled

	// Initialize PostHog client early so C logs during SDL init are captured
	app.initSentry()

	logMsg("INFO: Initializing MiyooPod...")
	logMsg("INFO: SDL init...")
	if C.init() != 0 {
		logMsg("FATAL: SDL init failed!")
		return
	}
	logMsg("INFO: SDL init ok!")

	// Init audio
	logMsg("INFO: Audio init...")
	if C.audio_init() != 0 {
		logMsg("WARNING: Failed to init SDL2_mixer audio")
	} else {
		logMsg("INFO: Audio init ok!")
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
	app.RedrawChan = make(chan struct{}, 1)
	app.LockKey = Y // Default lock key

	// Power management defaults
	app.AutoLockMinutes = 3 // Auto-lock after 3 minutes of inactivity
	app.ScreenPeekEnabled = true
	app.UpdateNotifications = true // Default: show update prompts
	app.LastActivityTime = time.Now()

	// Pre-render digit sprites for fast time display (bypass gg in hot path)
	app.initDigitSprites(app.FontTime)

	// Default volume/brightness (overridden by loadSettings if saved)
	app.SystemVolume = 50
	app.SystemBrightness = 50

	// Always max SDL2_mixer volume — MI_AO controls actual hardware volume
	audioSetVolume(100)

	// Load settings (theme and lock key) before showing splash - fast parse
	if err := app.loadSettings(); err != nil {
		logMsg(fmt.Sprintf("WARNING: Could not load settings: %v (using defaults)", err))
	}

	// Draw splash screen with logo (now using restored theme if available)
	app.drawLogoSplash()

	// Check for updates asynchronously (don't block startup)
	app.VersionCheckDone = make(chan struct{})
	go func() {
		app.checkVersion()
		close(app.VersionCheckDone)
	}()

	// Start power button monitor (reads directly from /dev/input/event0)
	app.startPowerButtonMonitor()

	// Generate initial icon PNG with current theme
	if err := app.generateIconPNG(); err != nil {
		logMsg(fmt.Sprintf("ERROR: Failed to generate icon: %v", err))
	}

	logMsg("INFO: MiyooPod init OK!")
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

// requestRedraw signals the main loop to call drawCurrentScreen on the next iteration.
// Safe to call from any goroutine. Non-blocking.
func (app *MiyooPod) requestRedraw() {
	select {
	case app.RedrawChan <- struct{}{}:
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
	// Global panic recovery — sends crash report synchronously before dying
	defer func() {
		if r := recover(); r != nil {
			panicMsg := fmt.Sprintf("%v", r)
			buf := make([]byte, 16384)
			n := runtime.Stack(buf, true)
			stackTrace := string(buf[:n])

			// Log locally
			logMsg(fmt.Sprintf("FATAL: Application crashed with panic: %s", panicMsg))
			logMsg(fmt.Sprintf("FATAL: Stack trace:\n%s", stackTrace))
			if logFile != nil {
				logFile.Sync()
			}

			// Send crash report synchronously — blocks until HTTP completes
			sendCrashReport("go_panic", panicMsg, stackTrace)

			panic(r) // Re-panic to show error
		}
	}()

	// Install signal handlers for C-level crashes (SIGSEGV, SIGABRT, SIGBUS)
	installCrashHandler()

	logMsg("\n\n\n-----------")
	logMsg("INFO: MiyooPod started!")

	app := createApp()
	app.Init()

	go app.RunUI()
	time.Sleep(100 * time.Millisecond)

	// Load library from JSON or perform full scan
	err := app.loadLibraryJSON()
	if err != nil {
		logMsg(fmt.Sprintf("WARNING: Could not load library from JSON: %v", err))
		logMsg("INFO: Performing full library scan...")

		// Launch scan as background goroutine with UI — wait for completion via channel
		scanDone := make(chan struct{})
		app.startLibraryScan(func() {
			close(scanDone)
		})

		// Wait for scan to complete (main loop continues polling keys/redraws)
		for {
			select {
			case <-scanDone:
				goto scanFinished
			case <-app.RedrawChan:
				app.drawCurrentScreen()
			default:
			}
			// Poll SDL events so the scan screen renders
			keyEvent := C_GetKeyPress()
			if keyEvent != -1 {
				if keyEvent < 0 {
					keyReleased := Key(-keyEvent - 1)
					app.handleKeyRelease(keyReleased)
				} else {
					app.handleKey(Key(keyEvent))
				}
			}
			time.Sleep(33 * time.Millisecond)
		}
	scanFinished:
	}

	// Restore saved playback state (queue, position) before building menu
	app.restorePlaybackState()

	// Build menu
	app.RootMenu = app.buildRootMenu()
	app.MenuStack = []*MenuScreen{app.RootMenu}

	// Start playback poller
	go app.startPlaybackPoller()

	// Extract album art from MP3s for albums without cached art (background)
	go app.deferredArtExtraction()

	// Start inactivity monitor for auto-lock
	go app.startInactivityMonitor()

	// Track app opened
	TrackAppLifecycle("app_opened", nil)

	// Draw initial menu
	app.drawCurrentScreen()

	// Check for update status from a previous OTA update
	app.handleUpdateStatus()

	// Wait for async version check to complete, then show update prompt if needed
	go func() {
		<-app.VersionCheckDone
		if app.UpdateAvailable && app.UpdateNotifications && app.Running {
			app.showUpdatePrompt()
		}
	}()

	// Main loop: poll SDL events on main thread (required by SDL2)
	// SDL_PollEvent MUST run on the thread that called SDL_Init (LockOSThread in init)
	// Sleep between polls to keep CPU usage low (replaces old runtime.Gosched spin loop)
	for app.Running {
		keyEvent := C_GetKeyPress()
		if keyEvent != -1 {
			// Log the keycode for debugging
			if keyEvent < 0 {
				// Key release event (negative value)
				// Convert back: -(keycode + 1) -> keycode
				keyReleased := Key(-keyEvent - 1)
				app.handleKeyRelease(keyReleased)
			} else {
				// Key press event (positive value)
				app.handleKey(Key(keyEvent))
			}
		}
		app.pollSeek()
		app.pollMarquee()
		// Check if a background goroutine requested a redraw (non-blocking)
		select {
		case <-app.RedrawChan:
			app.drawCurrentScreen()
		default:
		}
		time.Sleep(33 * time.Millisecond) // ~30Hz polling, main thread sleeps most of the time
	}

	// Save playback state before exit
	app.savePlaybackState()

	// Track app closed
	TrackAppLifecycle("app_closed", nil)

	// Cleanup: close refresh channel to unblock RunUI goroutine
	close(app.RefreshChan)

	sdlCleanup()
}

// setScreen changes the current screen and tracks the page view
func (app *MiyooPod) setScreen(screen ScreenType) {
	app.setScreenWithContext(screen, nil)
}

// setScreenWithContext changes screen with additional tracking context
func (app *MiyooPod) setScreenWithContext(screen ScreenType, properties map[string]interface{}) {
	if app.CurrentScreen != screen {
		app.CurrentScreen = screen
		TrackPageView(screen.String(), properties)
	}
}

// handleKeyRelease handles key release events
func (app *MiyooPod) handleKeyRelease(key Key) {
	// Handle seek release on Now Playing screen
	if (key == L || key == R) && app.SeekHeld {
		direction := app.seekKeyReleased()
		if direction != 0 {
			// Was a short tap — do prev/next track
			if direction < 0 {
				app.prevTrack()
			} else {
				app.nextTrack()
			}
			app.drawCurrentScreen()
		}
		return
	}
}

func (app *MiyooPod) handleKey(key Key) {
	// Update last activity time for any key press
	app.LastActivityTime = time.Now()

	// If update prompt is showing, it consumes all keys
	if app.handleUpdatePromptKey(key) {
		return
	}

	// If locked, handle peek or unlock
	if app.Locked {
		// Check for double-press of lock key to unlock
		if key == app.LockKey {
			now := time.Now()
			if now.Sub(app.LastYTime) < 500*time.Millisecond {
				// Double press detected - unlock
				app.toggleLock()
			} else {
				// Single press - just peek
				app.peekScreen()
			}
			app.LastYTime = now
		} else {
			// Any other key - just peek the screen
			app.peekScreen()
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

	// If search panel is active, it consumes most keys
	if app.SearchActive && app.handleSearchKey(key) {
		return
	}

	// Global keys (work from any screen)
	switch key {
	case START:
		// Go to Now Playing screen (not allowed during library scan or album art fetch)
		if app.CurrentScreen == ScreenLibraryScan || app.CurrentScreen == ScreenAlbumArt {
			return
		}
		if app.Playing != nil && app.Playing.Track != nil {
			if app.SearchActive {
				app.cancelSearch()
			}
			app.setScreen(ScreenNowPlaying)
			app.drawCurrentScreen()
		}
		return
	case L:
		if app.CurrentScreen == ScreenLibraryScan || app.CurrentScreen == ScreenAlbumArt {
			return
		}
		if app.CurrentScreen == ScreenNowPlaying && app.Playing != nil && app.Playing.State != StateStopped {
			app.seekKeyPressed(-1)
			return
		}
		app.prevTrack()
		app.drawCurrentScreen()
		return
	case R:
		if app.CurrentScreen == ScreenLibraryScan || app.CurrentScreen == ScreenAlbumArt {
			return
		}
		if app.CurrentScreen == ScreenNowPlaying && app.Playing != nil && app.Playing.State != StateStopped {
			app.seekKeyPressed(1)
			return
		}
		app.nextTrack()
		app.drawCurrentScreen()
		return
	case SELECT:
		if app.CurrentScreen == ScreenLibraryScan || app.CurrentScreen == ScreenAlbumArt {
			return
		}
		// On queue screen, SELECT clears queue (handled by handleQueueKey)
		if app.CurrentScreen == ScreenQueue {
			break // Fall through to screen-specific handler
		}
		// On searchable menu screens, toggle search instead of shuffle
		if app.CurrentScreen == ScreenMenu && app.isSearchableMenu() {
			app.toggleSearch()
			app.drawCurrentScreen()
			return
		}
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
	case ScreenAlbumArt:
		app.handleAlbumArtKey(key)
	case ScreenLibraryScan:
		app.handleLibraryScanKey(key)
	}
}

func (app *MiyooPod) handleNowPlayingKey(key Key) {
	switch key {
	case LEFT, B:
		app.setScreen(ScreenMenu)
		app.drawCurrentScreen()
	case RIGHT:
		// Show queue
		app.setScreen(ScreenQueue)
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
	case ScreenAlbumArt:
		app.drawAlbumArtScreen()
		app.drawAlbumArtStatusBar()
	case ScreenLibraryScan:
		app.drawLibraryScanScreen()
		app.drawLibraryScanStatusBar()
	}

	// Draw lock overlay if locked
	if app.Locked {
		app.drawLockOverlay()
	}

	// Draw volume/brightness overlay if visible
	if app.OverlayVisible {
		app.drawVolumeOrBrightnessOverlay()
	}

	// Draw error popup overlay if active
	app.drawErrorPopup()

	// Draw update prompt overlay if showing
	if app.ShowingUpdatePrompt {
		app.drawUpdatePromptOverlay()
		// drawUpdatePromptOverlay calls triggerRefresh, so skip the one below
		return
	}

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

func sdlCleanup() {
	C.audio_quit()
	C.quit()
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
