#!/bin/sh
cd "$(dirname "$0")"
export LD_LIBRARY_PATH="./libs:.:$LD_LIBRARY_PATH:/customer/lib:/mnt/SDCARD/miyoo/lib"
export SDL_VIDEODRIVER=mmiyoo
export SDL_AUDIODRIVER=mmiyoo
export EGL_VIDEODRIVER=mmiyoo

# Bootstrap: swap in any staged files from OTA update
# Updater can't replace libs or itself while running, so they're saved as .new
if [ -f "./updater_new" ]; then
    mv ./updater_new ./updater
    chmod +x ./updater
fi

# Swap staged shared libraries from OTA update
for newlib in ./libs/*.new; do
    [ -f "$newlib" ] || continue
    target="${newlib%.new}"
    mv "$newlib" "$target"
    chmod +x "$target"
done

# Use Onion's proper audio server stop script if available
if [ -f "/mnt/SDCARD/.tmp_update/script/stop_audioserver.sh" ]; then
    . /mnt/SDCARD/.tmp_update/script/stop_audioserver.sh
else
    # Fallback to manual audio server killing
    killall -9 MainUI 2>/dev/null
    killall -9 audioserver 2>/dev/null
    killall -9 audioserver.mod 2>/dev/null
    sleep 1
fi

# Set CPU to performance mode for better audio decoding
echo performance > /sys/devices/system/cpu/cpu0/cpufreq/scaling_governor 2>/dev/null

# Kill keymon to prevent power button interference
# We'll handle power button in the app and restart keymon on exit
killall -9 keymon 2>/dev/null

./MiyooPod

# Restart keymon
keymon &

# Reset CPU governor to ondemand to save battery
echo ondemand > /sys/devices/system/cpu/cpu0/cpufreq/scaling_governor 2>/dev/null

# Restart MainUI
cd /mnt/SDCARD/miyoo/app
./MainUI &
