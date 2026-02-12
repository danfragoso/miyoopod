package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

const VERSION_CHECK_URL = "https://github.com/danfragoso/miyoopod/raw/refs/heads/main/version.json"

type VersionInfo struct {
	Version   string `json:"version"`
	Checksum  string `json:"checksum"`
	URL       string `json:"url"`
	Size      int64  `json:"size"`
	Changelog string `json:"changelog"`
}

// checkVersion fetches the latest version from GitHub and compares with current version
func (app *MiyooPod) checkVersion() string {
	client := getInsecureHTTPClient(5 * time.Second)

	resp, err := client.Get(VERSION_CHECK_URL)
	if err != nil {
		logMsg(fmt.Sprintf("WARNING: Failed to fetch version: %v", err))
		return "Failed to fetch version"
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		logMsg(fmt.Sprintf("WARNING: Version check returned status: %d", resp.StatusCode))
		return "Failed to fetch version"
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logMsg(fmt.Sprintf("WARNING: Failed to read version response: %v", err))
		return "Failed to fetch version"
	}

	var remoteVersion VersionInfo
	if err := json.Unmarshal(body, &remoteVersion); err != nil {
		logMsg(fmt.Sprintf("WARNING: Failed to parse version JSON: %v", err))
		return "Failed to fetch version"
	}

	// Compare versions — only prompt if remote is strictly newer
	if !isNewerVersion(APP_VERSION, remoteVersion.Version) {
		logMsg(fmt.Sprintf("INFO: Version check: Up to date (%s, remote: %s)", APP_VERSION, remoteVersion.Version))
		app.UpdateAvailable = false
		return "Up to date"
	}

	logMsg(fmt.Sprintf("INFO: Version check: Update available (current: %s, latest: %s)", APP_VERSION, remoteVersion.Version))
	app.UpdateAvailable = true
	app.UpdateInfo = &remoteVersion
	return fmt.Sprintf("Update available: v%s", remoteVersion.Version)
}

// isNewerVersion returns true if remote is strictly newer than current.
// Compares dot-separated numeric segments (e.g. "0.0.5" vs "0.0.6").
func isNewerVersion(current, remote string) bool {
	parseParts := func(v string) []int {
		parts := strings.Split(v, ".")
		nums := make([]int, len(parts))
		for i, p := range parts {
			n, _ := strconv.Atoi(p)
			nums[i] = n
		}
		return nums
	}

	cur := parseParts(current)
	rem := parseParts(remote)

	// Pad to equal length
	for len(cur) < len(rem) {
		cur = append(cur, 0)
	}
	for len(rem) < len(cur) {
		rem = append(rem, 0)
	}

	for i := range cur {
		if rem[i] > cur[i] {
			return true
		}
		if rem[i] < cur[i] {
			return false
		}
	}
	return false
}
