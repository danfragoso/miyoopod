# MiyooPod

MP3 player for the Miyoo Mini Plus, Mini v4, and Mini Flip running OnionOS. Inspired by the classic iPod interface.

![MiyooPod](screenshots/hero.png)

## Features

- iPod-inspired user interface with multiple themes
- Browse by Artists, Albums, and Songs
- Search/filter lists with on-screen A-Z keyboard
- Album art display with automatic fetching from MusicBrainz
- Shuffle and repeat modes
- Seek/fast-forward/rewind with accelerating speed
- Over-the-air updates
- Session persistence (queue, position, shuffle/repeat restored on launch)
- Native 640×480 resolution optimized for Miyoo Mini
- 17 customizable themes (Classic iPod, Dark Blue, Nord, Cyberpunk, and more)
- MP3, FLAC, and OGG/Vorbis playback
- Lyrics display with LRC timed sync and auto-scroll

## Installation

1. Download [MiyooPod.zip](https://github.com/danfragoso/miyoopod/raw/refs/heads/main/releases/MiyooPod.zip)
2. Extract the MiyooPod.zip file
3. Connect your Miyoo Mini Plus SD card to your computer
4. Copy the `MiyooPod` folder to `/App` on your SD card
5. Safely eject the SD card and insert it back into your Miyoo Mini Plus
6. MiyooPod will appear in your Apps menu

## Adding Songs

MiyooPod reads music files from your SD card's Music folder:

```
/mnt/SDCARD/Media/Music/
```

1. Connect your Miyoo Mini SD card to your computer
2. Navigate to `/Media/Music/` folder
3. Copy your MP3 files and folders into this directory
4. Organize your music by artist/album folders for better library organization
5. Launch MiyooPod - it will automatically scan and index your music library

## Supported Formats

MiyooPod supports **MP3**, **FLAC**, and **OGG/Vorbis** audio files.

**Recommended Format:** MP3 @ 256kbps

- **Format:** MP3 (MPEG-1 Audio Layer 3)
- **Bitrate:** 256kbps CBR (Constant Bitrate) or VBR V0
- **Sample Rate:** 44.1kHz
- **Channels:** Stereo

> **Note:** The Miyoo Mini Plus audio output is not high quality enough to justify higher bitrate files. Playback might be choppy with higher bitrate files. FLAC files are supported but MP3 is recommended for best performance on the Miyoo Mini's limited hardware.

## Album Artwork

### Embedded Artwork
MiyooPod automatically extracts album art embedded in your MP3 files' ID3 tags.

### Automatic Download from MusicBrainz
For albums without embedded artwork, MiyooPod can automatically fetch album covers:

1. Navigate to **Settings** from the main menu
2. Select **"Fetch Album Art"**
3. MiyooPod will scan your library and download missing album artwork

> **Note:** Requires internet connection via WiFi. Artwork is stored in `/mnt/SDCARD/Media/Music/.miyoopod_artwork/`

## Settings

- **Themes** - Choose from 17 visual themes (Classic iPod, Dark, Dark Blue, Light, Nord, Solarized Dark, Matrix Green, Retro Amber, Purple Haze, Cyberpunk, Coffee, Ocean, Forest, Sunset, Neon, Midnight, Gruvbox, Candy)
- **Screen Lock** - Press the POWER button to lock/unlock the screen, preventing accidental presses during playback. Hold POWER for a few seconds to quit the app
- **Auto Screen Lock** - Automatically lock after a period of inactivity (1/3/5/10 min, or disabled)
- **Fetch Album Art** - Automatically download missing album artwork from MusicBrainz
- **Check for Updates** - Manually check for and install OTA updates
- **Update Notifications** - Toggle automatic update prompts on/off
- **Clear App Data** - Reset library cache, settings, and artwork
- **Toggle Logs** - Enable or disable debug logging
- **Rescan Library** - Force a complete rescan of your music library
- **About** - View app version and check for updates

## Technical Details

Built using **Go 1.22.2** with native C bindings (CGO) for graphics and audio.

### Architecture
- **Platform:** Miyoo Mini Plus, Mini v4, Mini Flip / OnionOS
- **CPU:** ARM Cortex-A7 (dual-core)
- **Resolution:** 640×480 native
- **Cross-compilation:** arm-linux-gnueabihf-gcc

### Key Libraries
- **SDL2** - Graphics, input handling, window management
- **SDL2_mixer** - Audio playback with MP3 decoding (libmpg123)
- **fogleman/gg** - 2D graphics rendering
- **dhowden/tag** - ID3 tag parsing
- **golang.org/x/image** - Image processing and font rendering

### Performance Optimizations
- Dual-core utilization (UI and audio on separate cores)
- Pre-rendered digit sprites for time display
- Text measurement and album art caching
- Library metadata cached as JSON for fast startup

## Troubleshooting

### App does not launch / crashes on startup

The library cache files may be corrupted. Connect your SD card to your computer and delete the hidden JSON files in the music folder:

```
/mnt/SDCARD/Media/Music/.miyoopod_library.json
/mnt/SDCARD/Media/Music/.miyoopod_state.json
```

MiyooPod will rescan your library on the next launch.

### Album art not displaying correctly

Open **Settings** → **Clear App Data** to wipe the artwork cache and library metadata. MiyooPod will rebuild everything on the next launch. You can then use **Fetch Album Art** again to re-download missing covers.

### Checking logs

If the app is misbehaving, enable logging from **Settings** → **Toggle Logs**, then reproduce the issue and check the log file on your SD card:

```
/mnt/SDCARD/App/MiyooPod/miyoopod.log
```

Attach this file when reporting a bug on [GitHub Issues](https://github.com/danfragoso/miyoopod/issues).

### A broken OTA update made the app unusable

If an update left the app in a broken state, manually reinstall the latest version:

1. Download [MiyooPod.zip](https://github.com/danfragoso/miyoopod/raw/refs/heads/main/releases/MiyooPod.zip)
2. Extract and copy the `MiyooPod` folder to `/App` on your SD card, overwriting the existing files
3. Your music library and settings are stored separately and will not be affected

## Building from Source

```bash
# Cross-compile for ARM
make go
```

The build process uses CGO to compile Go source with C bindings and bundles all required shared libraries.

## Changelog

### Version 0.0.8

**New Features**
- Adjustable font size (Small/Medium/Large) from Settings
- CPU on Lock setting to keep the performance governor while the screen is locked (instead of dropping to power save)

### Version 0.0.7

**New Features**
- Single-button screen lock: press POWER to lock/unlock, hold POWER to quit the app
- Bitrate and sample rate shown on the Now Playing format badge
- Released under the MIT License, with third-party license notices included

**Improvements**
- Faster text rendering with a glyph and label sprite cache (glyph atlas)
- Smoother menu navigation via frame caching and direct-pixel drawing in hot paths
- Better battery life: adaptive polling, CPU governor lowered while locked, event-driven power button
- Audio loaded into RAM to reduce SD card I/O during playback
- Faster library scanning; album art no longer freezes the UI (pre-scaled artwork cache)
- Now Playing layout refinements with centered controls and better use of space
- Library scan skips macOS and Windows metadata files
- Removed the screen peek feature — the screen stays off while locked

**Bug Fixes**
- Fixed track duration showing 0 on first play
- Fixed zero volume when resuming a saved, paused session
- Fixed brightness flash when unlocking
- Fixed shuffle so selecting a song plays that exact song
- Fixed the About screen reopening immediately after closing it
- Fixed the marquee and progress bar drawing over the lock and power-exit overlays
- Fixed race conditions that could corrupt the framebuffer during background work

### Version 0.0.6
- FLAC and OGG/Vorbis playback support (decoded via statically linked drflac and stb_vorbis in SDL2_mixer)
- Lyrics support: embedded lyrics (ID3 USLT, Vorbis comments) displayed with word-wrap and scroll
- LRC timed lyrics: synced highlighting of the current line with auto-scroll and manual scroll override
- Hold ↑/↓ to scroll lists continuously without repeated presses
- SELECT + START to quit the app

### Version 0.0.5
- Over-the-air updates with download progress, checksum verification, and automatic rollback on failure
- Search: filter Artists, Albums, and Songs with an on-screen A-Z keyboard (press SELECT on any list)
- Seek/fast-forward/rewind: hold L or R on the Now Playing screen with accelerating speed
- Session persistence: queue, playback position, shuffle/repeat state, and current track restored across launches
- Header marquee: now-playing track info scrolls in the header bar when browsing menus
- Crash reporting: fatal panics and C-level signals are logged and reported automatically
- Clear App Data option in Settings to reset library cache, settings, and artwork
- Faster startup: version check no longer blocks splash screen; album art uses fast RGBA pixel cache on disk
- Non-blocking library scan with dedicated progress screen showing track count, current folder, and phase
- Non-blocking album art fetch with progress bar, percentage, and cancel/retry support
- Volume and brightness persisted across app launches
- Background album art extraction from MP3 tags after startup
- Toggle update notifications on/off from Settings
- Manual "Check for Updates" option in Settings
- Fixed race conditions where background goroutines corrupted the framebuffer causing panics
- Fixed volume/brightness overlay screen flash caused by partial framebuffer updates
- Fixed volume resetting on every launch

### Version 0.0.4
- Fixed volume control using MI_AO ioctl (correct indirect buffer layout matching Onion/keymon)
- Fixed volume icon SVG being cut off in the overlay
- Added screen lock with power button
- Added auto screen lock setting (1/3/5/10 min or disabled)
- Added screen peek toggle setting (enable/disable screen wake on button press while locked)
- Fixed brightness and volume being adjustable while screen is locked
- Fixed now playing progress bar drawing over the lock overlay

### Version 0.0.3
- Fixed PostHog logging initialization order
- C logs from SDL initialization now properly captured
- Device model detection and reporting (Mini Plus, Mini v4, Mini Flip)
- Display resolution metrics sent to analytics
- Independent local logs and developer logs settings

### Version 0.0.2
- Added support for Miyoo Mini v4 (750×560 resolution)
- Added support for Miyoo Mini Flip (750×560 resolution)
- Automatic resolution detection via framebuffer device
- UI scaling adapts to different screen sizes while maintaining aspect ratio
- Disabled local logs by default (developer logs still enabled)

### Version 0.0.1
- Initial release
- iPod-inspired user interface
- 11 customizable themes
- Album art display and automatic fetching from MusicBrainz
- Shuffle and repeat modes
- Optimized for Miyoo Mini Plus (640×480)

## Contributing

MiyooPod is open-source! Contributions are welcome.

- **Report bugs:** [GitHub Issues](https://github.com/danfragoso/miyoopod/issues)
- **Request features:** [New Issue](https://github.com/danfragoso/miyoopod/issues/new)
- **Submit PRs:** [Pull Requests](https://github.com/danfragoso/miyoopod/pulls)

## AI Generated Content

MiyooPod was developed with the help of AI coding assistant tools, and portions of its codebase include AI-generated code.

## User Data

MiyooPod uses PostHog to capture error logs, exceptions, crashes, and user interactions. If you are not comfortable with this, you can go into **Settings** and disable the **Developer Logs** option.

## License

MiyooPod is released under the [MIT License](LICENSE).

This project bundles and depends on third-party software under their own licenses. See [THIRD_PARTY_LICENSES.md](THIRD_PARTY_LICENSES.md) for details.

## Author

Created by [Danilo Fragoso](https://github.com/danfragoso)

---

