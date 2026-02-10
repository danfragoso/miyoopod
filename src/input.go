package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"time"
)

// Linux input event structure
type inputEvent struct {
	Time  syscallTimeval
	Type  uint16
	Code  uint16
	Value int32
}

type syscallTimeval struct {
	Sec  int32
	Usec int32
}

const (
	EV_KEY         = 0x01
	KEY_POWER      = 116
	KEY_VOLUMEUP   = 115
	KEY_VOLUMEDOWN = 114
	KEY_ESC        = 1 // MENU button (ESC key)
)

// startPowerButtonMonitor reads power button and volume events directly from /dev/input/event0
// This bypasses SDL2 which doesn't have power button mapping in the prebuilt library
func (app *MiyooPod) startPowerButtonMonitor() {
	go func() {
		file, err := os.Open("/dev/input/event0")
		if err != nil {
			logMsg(fmt.Sprintf("WARNING: Could not open /dev/input/event0: %v", err))
			return
		}
		defer file.Close()

		logMsg("Power button monitor started on /dev/input/event0")

		var ev inputEvent
		for app.Running {
			err := binary.Read(file, binary.LittleEndian, &ev)
			if err != nil {
				logMsg(fmt.Sprintf("ERROR: Reading input event: %v", err))
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// Only process key events
			if ev.Type != EV_KEY {
				continue
			}

			// Handle power button
			if ev.Code == KEY_POWER {
				if ev.Value == 1 { // Key pressed
					logMsg("DEBUG: Power button PRESSED (from /dev/input/event0)")
					app.handlePowerButtonPress()
				} else if ev.Value == 0 { // Key released
					logMsg("DEBUG: Power button RELEASED (from /dev/input/event0)")
					app.handlePowerButtonRelease()
				}
			}

			// Handle volume up
			if ev.Code == KEY_VOLUMEUP && ev.Value == 1 {
				app.handleVolumeUp()
			}

			// Handle volume down
			if ev.Code == KEY_VOLUMEDOWN && ev.Value == 1 {
				app.handleVolumeDown()
			}

			// Track MENU key state for brightness control
			if ev.Code == KEY_ESC {
				if ev.Value == 1 { // Pressed
					app.MenuKeyPressed = true
				} else if ev.Value == 0 { // Released
					app.MenuKeyPressed = false
				}
			}
		}
	}()
}

func (app *MiyooPod) handlePowerButtonPress() {
	if !app.PowerButtonPressed {
		app.PowerButtonPressed = true
		app.PowerButtonPressTime = time.Now()
		// Start monitoring for long hold
		go app.monitorPowerButtonHold()
	}
}

func (app *MiyooPod) handlePowerButtonRelease() {
	if app.PowerButtonPressed {
		holdDuration := time.Since(app.PowerButtonPressTime)
		app.PowerButtonPressed = false

		// If held for less than 5 seconds, toggle lock
		if holdDuration < 5*time.Second {
			app.toggleLock()
		}
		// If held for 5+ seconds, monitorPowerButtonHold already handled shutdown
	}
}

func (app *MiyooPod) handleVolumeUp() {
	if app.Locked {
		return
	}
	if app.MenuKeyPressed {
		app.adjustBrightness(10)
	} else {
		app.adjustVolume(5)
	}
}

func (app *MiyooPod) handleVolumeDown() {
	if app.Locked {
		return
	}
	if app.MenuKeyPressed {
		app.adjustBrightness(-10)
	} else {
		app.adjustVolume(-5)
	}
}
