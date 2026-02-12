package main

/*
#cgo CFLAGS: -I/root/include/SDL2 -O2 -w -D_GNU_SOURCE=1 -D_REENTRANT
#cgo LDFLAGS: -L/root/lib -Wl,-rpath-link,/root/lib -Wl,-rpath,'$ORIGIN' -Wl,--unresolved-symbols=ignore-in-shared-libs -lSDL2 -lpthread
#include <stdlib.h>
#include "sdl.c"
*/
import "C"

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/fogleman/gg"
)

const (
	SCREEN_WIDTH  = 640
	SCREEN_HEIGHT = 480
)

type UpdateRequest struct {
	Version  string `json:"version"`
	URL      string `json:"url"`
	Checksum string `json:"checksum"`
	Size     int64  `json:"size"`
}

type UpdateStatus struct {
	Success bool   `json:"success"`
	Version string `json:"version"`
	Error   string `json:"error,omitempty"`
}

type UpdaterTheme struct {
	BG        string
	HeaderTxt string
	ItemTxt   string
	Accent    string
	Dim       string
	Progress  string
	ProgBG    string
}

type SettingsFile struct {
	Theme string `json:"theme"`
}

var (
	defaultTheme = UpdaterTheme{
		BG:        "#B8B8B8",
		HeaderTxt: "#000000",
		ItemTxt:   "#000000",
		Accent:    "#4A90E2",
		Dim:       "#808080",
		Progress:  "#4A90E2",
		ProgBG:    "#909090",
	}

	// Map theme names to their colors (matching MiyooPod themes)
	themeMap = map[string]UpdaterTheme{
		"Classic iPod":    {BG: "#B8B8B8", HeaderTxt: "#000000", ItemTxt: "#000000", Accent: "#4A90E2", Dim: "#808080", Progress: "#4A90E2", ProgBG: "#909090"},
		"Dark Blue":       {BG: "#1A1A2E", HeaderTxt: "#E0E0E0", ItemTxt: "#FFFFFF", Accent: "#5390D9", Dim: "#666666", Progress: "#5390D9", ProgBG: "#333333"},
		"Dark":            {BG: "#1C1C1C", HeaderTxt: "#FFFFFF", ItemTxt: "#FFFFFF", Accent: "#FFFFFF", Dim: "#777777", Progress: "#FFFFFF", ProgBG: "#444444"},
		"Matrix Green":    {BG: "#0D0D0D", HeaderTxt: "#00FF41", ItemTxt: "#00FF41", Accent: "#00FF41", Dim: "#004400", Progress: "#00FF41", ProgBG: "#002200"},
		"Retro Amber":     {BG: "#1A0F00", HeaderTxt: "#FFAA00", ItemTxt: "#FFAA00", Accent: "#FFAA00", Dim: "#664400", Progress: "#FFAA00", ProgBG: "#442200"},
		"Purple Haze":     {BG: "#1A0A2E", HeaderTxt: "#E0C0FF", ItemTxt: "#E0C0FF", Accent: "#AA66FF", Dim: "#6633AA", Progress: "#AA66FF", ProgBG: "#331155"},
		"Light":           {BG: "#FFFFFF", HeaderTxt: "#000000", ItemTxt: "#000000", Accent: "#007AFF", Dim: "#888888", Progress: "#007AFF", ProgBG: "#DDDDDD"},
		"Nord":            {BG: "#2E3440", HeaderTxt: "#ECEFF4", ItemTxt: "#ECEFF4", Accent: "#88C0D0", Dim: "#4C566A", Progress: "#88C0D0", ProgBG: "#434C5E"},
		"Solarized Dark":  {BG: "#002B36", HeaderTxt: "#93A1A1", ItemTxt: "#839496", Accent: "#2AA198", Dim: "#586E75", Progress: "#268BD2", ProgBG: "#073642"},
		"Cyberpunk":       {BG: "#0A0E27", HeaderTxt: "#00FFFF", ItemTxt: "#FF00FF", Accent: "#00FFFF", Dim: "#4B0082", Progress: "#FF00FF", ProgBG: "#2A2F4A"},
		"Coffee":          {BG: "#2C1810", HeaderTxt: "#E8D4B8", ItemTxt: "#D4A574", Accent: "#C1986B", Dim: "#5C4033", Progress: "#D2B48C", ProgBG: "#4A3528"},
		"Ocean":           {BG: "#001F3F", HeaderTxt: "#7FDBFF", ItemTxt: "#B8E6F5", Accent: "#00D4FF", Dim: "#336B87", Progress: "#00D4FF", ProgBG: "#003D5C"},
		"Forest":          {BG: "#0F2027", HeaderTxt: "#C5E1A5", ItemTxt: "#A5D6A7", Accent: "#76FF03", Dim: "#4A6B3F", Progress: "#8BC34A", ProgBG: "#1B5E20"},
		"Sunset":          {BG: "#1A0A0A", HeaderTxt: "#FFD700", ItemTxt: "#FFA500", Accent: "#FF4500", Dim: "#8B4513", Progress: "#FF8C00", ProgBG: "#4A2A2A"},
		"Neon":            {BG: "#000000", HeaderTxt: "#FF10F0", ItemTxt: "#00FF00", Accent: "#00FFFF", Dim: "#444444", Progress: "#FF10F0", ProgBG: "#1A1A1A"},
		"Midnight":        {BG: "#0C0C1E", HeaderTxt: "#C8C8FF", ItemTxt: "#B0B0E8", Accent: "#6E6EFF", Dim: "#4A4A7E", Progress: "#8080FF", ProgBG: "#2A2A5E"},
		"Gruvbox":         {BG: "#282828", HeaderTxt: "#EBDBB2", ItemTxt: "#EBDBB2", Accent: "#FABD2F", Dim: "#7C6F64", Progress: "#83A598", ProgBG: "#3C3836"},
		"Candy":           {BG: "#FFE5F4", HeaderTxt: "#8B008B", ItemTxt: "#C71585", Accent: "#FF1493", Dim: "#BA55D3", Progress: "#FF69B4", ProgBG: "#FFC0E5"},
	}

	// Shared cancel flag for download goroutine
	cancelled int32

	// Power button warning state
	powerWarningVisible int32
)

const UPDATE_INFO_PATH = "/mnt/SDCARD/Media/Music/.miyoopod_update.json"
const UPDATE_STATUS_PATH = "/mnt/SDCARD/Media/Music/.miyoopod_update_status"
const SETTINGS_PATH = "/mnt/SDCARD/Media/Music/.miyoopod_settings.json"
const BACKUP_DIR = ".miyoopod_backup"

func init() {
	runtime.GOMAXPROCS(2)
	runtime.LockOSThread()
}

func pollKey() int {
	return int(C.updater_poll_event())
}

// Linux input event constants
const (
	EV_KEY    = 0x01
	KEY_POWER = 116
)

type inputEvent struct {
	TimeSec  int32
	TimeUsec int32
	Type     uint16
	Code     uint16
	Value    int32
}

// startPowerButtonMonitor reads the power button from /dev/input/event0
// and sets a flag to show a warning overlay instead of allowing shutdown.
func startPowerButtonMonitor() {
	go func() {
		file, err := os.Open("/dev/input/event0")
		if err != nil {
			return // Can't monitor, not critical
		}
		defer file.Close()

		var ev inputEvent
		for {
			err := binary.Read(file, binary.LittleEndian, &ev)
			if err != nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			if ev.Type == EV_KEY && ev.Code == KEY_POWER && ev.Value == 1 {
				// Show warning for 3 seconds
				atomic.StoreInt32(&powerWarningVisible, 1)
				go func() {
					time.Sleep(3 * time.Second)
					atomic.StoreInt32(&powerWarningVisible, 0)
				}()
			}
		}
	}()
}

func main() {
	// Keep heap small on the Miyoo Mini's limited RAM (~64MB total)
	// GC runs more often but prevents OOM during download
	debug.SetGCPercent(20)

	// Clean up any leftover temp files from previous failed updates
	os.Remove(".update_download.zip")
	os.Remove(".update_download.zip.tmp")
	os.Remove(".update_tmp.zip") // Legacy temp file from older updater versions

	// Read update request
	reqData, err := os.ReadFile(UPDATE_INFO_PATH)
	if err != nil {
		writeStatus(false, "", fmt.Sprintf("Failed to read update info: %v", err))
		relaunch()
		return
	}

	var req UpdateRequest
	if err := json.Unmarshal(reqData, &req); err != nil {
		writeStatus(false, "", fmt.Sprintf("Failed to parse update info: %v", err))
		relaunch()
		return
	}

	// Load theme from settings
	theme := loadTheme()

	// Init SDL
	if C.updater_init() != 0 {
		writeStatus(false, req.Version, "Failed to init display")
		relaunch()
		return
	}
	defer C.updater_quit()

	// Start monitoring power button to show warning
	startPowerButtonMonitor()

	dc := gg.NewContext(SCREEN_WIDTH, SCREEN_HEIGHT)
	fb, _ := dc.Image().(*image.RGBA)

	// Load font
	fontPath := "./assets/ui_font.ttf"
	fontTitle, err := gg.LoadFontFace(fontPath, 26)
	if err != nil {
		writeStatus(false, req.Version, "Failed to load font")
		relaunch()
		return
	}
	fontMenu, _ := gg.LoadFontFace(fontPath, 24)
	fontSmall, _ := gg.LoadFontFace(fontPath, 16)

	refresh := func() {
		pixels := (*C.uchar)(unsafe.Pointer(&fb.Pix[0]))
		C.updater_refresh(pixels)
	}

	drawProgress := func(status string, progress float64, hint string) {
		dc.SetHexColor(theme.BG)
		dc.Clear()

		// Title
		dc.SetFontFace(fontTitle)
		dc.SetHexColor(theme.HeaderTxt)
		dc.DrawStringAnchored("Updating MiyooPod", SCREEN_WIDTH/2, 160, 0.5, 0.5)

		// Version
		dc.SetFontFace(fontSmall)
		dc.SetHexColor(theme.Accent)
		dc.DrawStringAnchored(fmt.Sprintf("v%s", req.Version), SCREEN_WIDTH/2, 195, 0.5, 0.5)

		// Progress bar background
		barX := 120.0
		barY := 240.0
		barW := 400.0
		barH := 20.0

		dc.SetHexColor(theme.ProgBG)
		dc.DrawRoundedRectangle(barX, barY, barW, barH, 4)
		dc.Fill()

		// Progress bar fill
		if progress > 0 {
			fillW := barW * progress
			if fillW < 8 {
				fillW = 8
			}
			dc.SetHexColor(theme.Progress)
			dc.DrawRoundedRectangle(barX, barY, fillW, barH, 4)
			dc.Fill()
		}

		// Percentage
		dc.SetFontFace(fontMenu)
		dc.SetHexColor(theme.ItemTxt)
		dc.DrawStringAnchored(fmt.Sprintf("%.0f%%", progress*100), SCREEN_WIDTH/2, barY+barH+30, 0.5, 0.5)

		// Status text
		dc.SetFontFace(fontSmall)
		dc.SetHexColor(theme.Dim)
		dc.DrawStringAnchored(status, SCREEN_WIDTH/2, barY+barH+60, 0.5, 0.5)

		// Button hint
		if hint != "" {
			dc.SetHexColor(theme.Dim)
			dc.DrawStringAnchored(hint, SCREEN_WIDTH/2, SCREEN_HEIGHT-30, 0.5, 0.5)
		}

		// Power button warning overlay
		if atomic.LoadInt32(&powerWarningVisible) == 1 {
			dc.SetRGBA(0, 0, 0, 0.85)
			dc.DrawRectangle(0, 0, SCREEN_WIDTH, SCREEN_HEIGHT)
			dc.Fill()

			// Warning box
			boxW := 460.0
			boxH := 120.0
			boxX := (SCREEN_WIDTH - boxW) / 2
			boxY := (SCREEN_HEIGHT - boxH) / 2

			dc.SetHexColor(theme.BG)
			dc.DrawRoundedRectangle(boxX, boxY, boxW, boxH, 12)
			dc.Fill()

			// Red border
			dc.SetHexColor("#FF4444")
			dc.SetLineWidth(3)
			dc.DrawRoundedRectangle(boxX, boxY, boxW, boxH, 12)
			dc.Stroke()

			dc.SetFontFace(fontTitle)
			dc.SetHexColor("#FF4444")
			dc.DrawStringAnchored("Do not power off!", SCREEN_WIDTH/2, boxY+40, 0.5, 0.5)

			dc.SetFontFace(fontSmall)
			dc.SetHexColor(theme.ItemTxt)
			dc.DrawStringAnchored("Update in progress, please wait...", SCREEN_WIDTH/2, boxY+80, 0.5, 0.5)
		}

		refresh()
	}

	// Poll keys in the main thread to check for cancel during download
	// The download runs in a goroutine so we can poll SDL events here
	checkCancel := func() bool {
		key := pollKey()
		// Miyoo Mini SDL2 keycodes:
		// B = SDLK_LCTRL = 1073742048 (SDL_SCANCODE_LCTRL | SDLK_SCANCODE_MASK)
		// MENU = SDLK_ESCAPE = 27
		if key == 1073742048 || key == 27 {
			return true
		}
		return false
	}

	// Free any init-phase memory before starting download
	runtime.GC()

	// Step 1: Download to disk (stream with 64KB buffer, never hold full file in RAM)
	drawProgress("Downloading update...", 0, "B Cancel")

	appDir, _ := os.Getwd()
	zipPath := filepath.Join(appDir, ".update_download.zip")

	dlChan := make(chan error, 1)

	go func() {
		err := downloadToFile(req.URL, zipPath, req.Size, func(downloaded, total int64) {
			if total > 0 {
				drawProgress("Downloading update...", float64(downloaded)/float64(total), "B Cancel")
			}
		})
		dlChan <- err
	}()

	// Poll for cancel or download completion on the main thread
	downloading := true
	for downloading {
		select {
		case err := <-dlChan:
			if err != nil {
				if atomic.LoadInt32(&cancelled) == 1 {
					drawProgress("Update cancelled", 0, "")
				} else {
					drawProgress(fmt.Sprintf("Download failed: %v", err), 0, "")
				}
				time.Sleep(2 * time.Second)
				if atomic.LoadInt32(&cancelled) != 1 {
					writeStatus(false, req.Version, fmt.Sprintf("Download failed: %v", err))
				}
				os.Remove(UPDATE_INFO_PATH)
				os.Remove(zipPath)
				relaunch()
				return
			}
			downloading = false
		default:
			if checkCancel() {
				atomic.StoreInt32(&cancelled, 1)
				drawProgress("Cancelling...", 0, "")
				<-dlChan
				drawProgress("Update cancelled", 0, "")
				time.Sleep(1 * time.Second)
				os.Remove(UPDATE_INFO_PATH)
				os.Remove(zipPath)
				relaunch()
				return
			}
			time.Sleep(33 * time.Millisecond)
		}
	}

	// Step 2: Verify checksum (reads file with 64KB buffer, not loaded into RAM)
	drawProgress("Verifying checksum...", 1.0, "")

	if req.Checksum != "" {
		expectedHash := strings.TrimPrefix(req.Checksum, "sha256:")
		actualHashStr, err := checksumFile(zipPath)
		if err != nil {
			drawProgress(fmt.Sprintf("Checksum error: %v", err), 0, "")
			time.Sleep(2 * time.Second)
			writeStatus(false, req.Version, fmt.Sprintf("Checksum error: %v", err))
			os.Remove(zipPath)
			relaunch()
			return
		}

		if actualHashStr != expectedHash {
			drawProgress("Checksum mismatch!", 0, "")
			time.Sleep(2 * time.Second)
			writeStatus(false, req.Version, "Checksum mismatch")
			os.Remove(zipPath)
			relaunch()
			return
		}
	}

	// Step 3: Installation phase - reset bar and show slow progress
	drawProgress("Installing update...", 0, "")

	// Start a fake progress ticker that advances ~1% per second
	installProgress := float64(0)
	installDone := make(chan error, 1)
	stopTicker := make(chan struct{})

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				installProgress += 0.01
				if installProgress > 0.90 {
					installProgress = 0.90 // Cap at 90% until actually done
				}
				drawProgress("Installing update...", installProgress, "")
			case <-stopTicker:
				return
			}
		}
	}()

	// Backup current files
	backupDir := filepath.Join(appDir, BACKUP_DIR)
	os.RemoveAll(backupDir) // Clean any old backup
	os.MkdirAll(backupDir, 0755)

	filesToBackup := []string{"MiyooPod", "launch.sh", "config.json"}
	for _, f := range filesToBackup {
		src := filepath.Join(appDir, f)
		dst := filepath.Join(backupDir, f)
		if data, err := os.ReadFile(src); err == nil {
			os.WriteFile(dst, data, 0755)
		}
	}

	// Backup assets directory
	assetsBackup := filepath.Join(backupDir, "assets")
	os.MkdirAll(assetsBackup, 0755)
	if entries, err := os.ReadDir(filepath.Join(appDir, "assets")); err == nil {
		for _, e := range entries {
			src := filepath.Join(appDir, "assets", e.Name())
			dst := filepath.Join(assetsBackup, e.Name())
			if data, err := os.ReadFile(src); err == nil {
				os.WriteFile(dst, data, 0644)
			}
		}
	}

	// Extract zip (the actual install)
	go func() {
		installDone <- extractZip(zipPath, appDir)
	}()

	err = <-installDone
	close(stopTicker)

	if err != nil {
		// Restore from backup and clean up staged .new files
		drawProgress("Install failed, restoring backup...", 0, "")
		restoreBackup(backupDir, appDir)
		cleanupStagedFiles(appDir)
		os.Remove(zipPath)
		time.Sleep(2 * time.Second)
		writeStatus(false, req.Version, fmt.Sprintf("Extract failed: %v", err))
		relaunch()
		return
	}

	// Remove downloaded zip - no longer needed
	os.Remove(zipPath)

	// Fill bar to 100% and show completion
	drawProgress("Update complete!", 1.0, "")

	os.Remove(UPDATE_INFO_PATH)
	os.RemoveAll(backupDir)

	writeStatus(true, req.Version, "")

	time.Sleep(1 * time.Second)
	relaunch()
}

func loadTheme() UpdaterTheme {
	data, err := os.ReadFile(SETTINGS_PATH)
	if err != nil {
		return defaultTheme
	}

	var settings SettingsFile
	if err := json.Unmarshal(data, &settings); err != nil {
		return defaultTheme
	}

	if t, ok := themeMap[settings.Theme]; ok {
		return t
	}

	return defaultTheme
}

// downloadToFile uses wget to download the file, avoiding Go's heavy net/http + TLS stack.
// wget is a tiny native binary that uses almost no RAM compared to Go's HTTP client (~10MB+).
// Progress is tracked by polling the output file size.
func downloadToFile(dlURL string, destPath string, expectedSize int64, onProgress func(downloaded, total int64)) error {
	tmpPath := destPath + ".tmp"
	os.Remove(tmpPath) // Clean any previous attempt

	// Try wget first (busybox wget on Miyoo Mini), fall back to curl.
	// --no-check-certificate is needed because the device has no CA bundle for HTTPS.
	cmd := exec.Command("wget", "--no-check-certificate", "-q", "-O", tmpPath, dlURL)
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		// wget not available, try curl (-k to skip cert verification)
		cmd = exec.Command("curl", "-k", "-s", "-L", "-o", tmpPath, dlURL)
		cmd.Stdout = nil
		cmd.Stderr = nil
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("no wget or curl available: %v", err)
		}
	}

	// Monitor file size for progress while wget runs
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	for {
		select {
		case err := <-done:
			// wget finished
			if err != nil {
				os.Remove(tmpPath)
				return fmt.Errorf("download failed: %v", err)
			}
			// Final progress update
			if info, statErr := os.Stat(tmpPath); statErr == nil && onProgress != nil {
				onProgress(info.Size(), expectedSize)
			}
			// Rename to final path
			if err := os.Rename(tmpPath, destPath); err != nil {
				os.Remove(tmpPath)
				return fmt.Errorf("rename: %v", err)
			}
			return nil
		default:
			// Check for cancellation
			if atomic.LoadInt32(&cancelled) == 1 {
				cmd.Process.Kill()
				<-done // Wait for process to exit
				os.Remove(tmpPath)
				return fmt.Errorf("cancelled")
			}

			// Poll file size for progress
			if info, err := os.Stat(tmpPath); err == nil && onProgress != nil {
				total := expectedSize
				if total <= 0 {
					total = info.Size() // Unknown total, just show downloaded
				}
				onProgress(info.Size(), total)
			}
			time.Sleep(200 * time.Millisecond)
		}
	}
}

// checksumFile computes SHA256 of a file using a small read buffer (64KB).
func checksumFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	buf := make([]byte, 64*1024)
	if _, err := io.CopyBuffer(h, f, buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func extractZip(zipPath string, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	buf := make([]byte, 64*1024) // Reuse a single 64KB buffer for all entries

	for _, f := range r.File {
		// The zip contains a MiyooPod/ directory prefix - strip it
		name := f.Name
		parts := strings.SplitN(name, "/", 2)
		if len(parts) < 2 || parts[1] == "" {
			continue // Skip the root directory entry
		}
		relPath := parts[1]

		targetPath := filepath.Join(destDir, relPath)

		// Prevent path traversal
		if !strings.HasPrefix(targetPath, destDir) {
			continue
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(targetPath, 0755)
			continue
		}

		// Shared libraries in libs/ are loaded by the running updater process.
		// Writing over them crashes the process. Stage them as .new files
		// and let launch.sh swap them in on next boot.
		if strings.HasPrefix(relPath, "libs/") {
			targetPath = targetPath + ".new"
		}

		// The updater binary itself can't be replaced while running either
		if relPath == "updater" {
			targetPath = filepath.Join(destDir, "updater_new")
		}

		// Ensure parent directory exists
		os.MkdirAll(filepath.Dir(targetPath), 0755)

		if err := extractZipEntry(f, targetPath, buf); err != nil {
			return fmt.Errorf("extract %s: %v", relPath, err)
		}
	}

	return nil
}

// extractZipEntry streams a single zip entry to disk using the provided buffer.
// Never loads the full file into RAM - critical for the Miyoo Mini's ~64MB.
func extractZipEntry(f *zip.File, targetPath string, buf []byte) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	// Write to temp file first, rename on success
	tmpPath := targetPath + ".tmp"
	outFile, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	_, err = io.CopyBuffer(outFile, rc, buf)
	if err != nil {
		outFile.Close()
		os.Remove(tmpPath)
		return err
	}

	if err := outFile.Sync(); err != nil {
		outFile.Close()
		os.Remove(tmpPath)
		return err
	}
	outFile.Close()

	// Set permissions before rename
	perm := f.Mode().Perm()
	os.Chmod(tmpPath, perm)

	// Atomic rename
	if err := os.Rename(tmpPath, targetPath); err != nil {
		os.Remove(tmpPath)
		return err
	}

	return nil
}

// cleanupStagedFiles removes any .new files left from a failed extraction.
// These are staged libs/updater that never got swapped in by launch.sh.
func cleanupStagedFiles(appDir string) {
	// Clean staged libs
	libsDir := filepath.Join(appDir, "libs")
	if entries, err := os.ReadDir(libsDir); err == nil {
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".new") {
				os.Remove(filepath.Join(libsDir, e.Name()))
			}
		}
	}
	// Clean staged updater
	os.Remove(filepath.Join(appDir, "updater_new"))
}

func restoreBackup(backupDir, appDir string) {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return
	}

	for _, e := range entries {
		src := filepath.Join(backupDir, e.Name())
		dst := filepath.Join(appDir, e.Name())

		if e.IsDir() {
			// Restore directory recursively
			os.RemoveAll(dst)
			os.MkdirAll(dst, 0755)
			subEntries, _ := os.ReadDir(src)
			for _, se := range subEntries {
				subSrc := filepath.Join(src, se.Name())
				subDst := filepath.Join(dst, se.Name())
				if data, err := os.ReadFile(subSrc); err == nil {
					os.WriteFile(subDst, data, 0644)
				}
			}
		} else {
			if data, err := os.ReadFile(src); err == nil {
				os.WriteFile(dst, data, 0755)
			}
		}
	}
}

func writeStatus(success bool, version, errMsg string) {
	status := UpdateStatus{
		Success: success,
		Version: version,
		Error:   errMsg,
	}

	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return
	}

	os.WriteFile(UPDATE_STATUS_PATH, data, 0644)
}

func relaunch() {
	launchPath := "./launch.sh"
	syscall.Exec("/bin/sh", []string{"/bin/sh", launchPath}, os.Environ())
}
