#!/bin/sh
cd "$(dirname "$0")"
export LD_LIBRARY_PATH=".:$LD_LIBRARY_PATH:/customer/lib:/mnt/SDCARD/miyoo/lib"
export SDL_VIDEODRIVER=mmiyoo
export SDL_AUDIODRIVER=mmiyoo
export EGL_VIDEODRIVER=mmiyoo

# Save current volume
curvol=$(cat /proc/mi_modules/mi_ao/mi_ao0 2>/dev/null | awk '/LineOut/ {print $8}' | sed 's/,//')

# Kill all MI_AO holders (MainUI owns Dev0)
killall -9 MainUI 2>/dev/null
killall -9 audioserver 2>/dev/null
killall -9 audioserver.mod 2>/dev/null

# Wait for MI_AO device to be released
sleep 1

./MiyooPod

# Restart MainUI
cd /mnt/SDCARD/miyoo/app
./MainUI &
