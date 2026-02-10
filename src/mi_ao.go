package main

import (
	"fmt"
	"math"
	"syscall"
	"unsafe"
)

const (
	MI_AO_SETVOLUME = 0x4008690b
	MI_AO_GETVOLUME = 0xc008690c
	MI_AO_SETMUTE   = 0x4008690d
	MI_AO_PATH      = "/dev/mi_ao"

	MIN_RAW_VALUE = -60
	MAX_RAW_VALUE = 30
)

// miAOIoctl performs an MI_AO ioctl call with the expected buffer layout:
// buf1 = [size_of_buf2, pointer_to_buf2], buf2 = [channel, value]
func miAOIoctl(fd int, cmd uintptr, channel int32, value int32) (int32, error) {
	buf2 := [2]int32{channel, value}
	buf1 := [2]uint64{uint64(unsafe.Sizeof(buf2)), uint64(uintptr(unsafe.Pointer(&buf2[0])))}

	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), cmd, uintptr(unsafe.Pointer(&buf1[0]))); errno != 0 {
		return 0, errno
	}
	return buf2[1], nil
}

// setMiAOVolume sets system volume using MI_AO ioctl (matches Onion/keymon curve)
func setMiAOVolume(percent int) {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	// Map 0-100% to 0-20 volume levels
	vol := int(math.Round(float64(percent) * 20.0 / 100.0))
	if vol > 20 {
		vol = 20
	}
	if vol < 0 {
		vol = 0
	}

	// Apply logarithmic volume curve (same as Onion)
	var volumeRaw int
	if vol != 0 {
		volumeRaw = int(math.Round(48 * math.Log10(1+float64(vol))))
	}

	// Offset by MIN_RAW_VALUE (same as setVolumeRaw in Onion)
	rawValue := int32(volumeRaw + MIN_RAW_VALUE)
	if rawValue > MAX_RAW_VALUE {
		rawValue = MAX_RAW_VALUE
	}
	if rawValue < MIN_RAW_VALUE {
		rawValue = MIN_RAW_VALUE
	}

	logMsg(fmt.Sprintf("DEBUG: Volume %d%% -> level %d -> raw %d", percent, vol, rawValue))

	fd, err := syscall.Open(MI_AO_PATH, syscall.O_RDWR, 0)
	if err != nil {
		logMsg(fmt.Sprintf("ERROR: Could not open MI_AO device: %v", err))
		return
	}
	defer syscall.Close(fd)

	// Get current volume to check for mute transitions
	prevValue, err := miAOIoctl(fd, MI_AO_GETVOLUME, 0, 0)
	if err != nil {
		logMsg(fmt.Sprintf("ERROR: MI_AO GETVOLUME failed: %v", err))
	}

	// Set volume
	_, err = miAOIoctl(fd, MI_AO_SETVOLUME, 0, rawValue)
	if err != nil {
		logMsg(fmt.Sprintf("ERROR: MI_AO SETVOLUME failed: %v", err))
		return
	}

	// Handle mute/unmute transitions
	if prevValue <= MIN_RAW_VALUE && rawValue > MIN_RAW_VALUE {
		miAOIoctl(fd, MI_AO_SETMUTE, 0, 0) // unmute
		logMsg("DEBUG: Unmuted audio")
	} else if prevValue > MIN_RAW_VALUE && rawValue <= MIN_RAW_VALUE {
		miAOIoctl(fd, MI_AO_SETMUTE, 0, 1) // mute
		logMsg("DEBUG: Muted audio")
	}

	logMsg(fmt.Sprintf("SUCCESS: MI_AO volume set: %d%% (raw %d)", percent, rawValue))
}
