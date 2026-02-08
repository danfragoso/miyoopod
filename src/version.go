package main

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

const VERSION_CHECK_URL = "https://raw.githubusercontent.com/danfragoso/miyoopod/refs/heads/main/version.json"

type VersionInfo struct {
	Version string `json:"version"`
}

// checkVersion fetches the latest version from GitHub and compares with current version
func (app *MiyooPod) checkVersion() string {
	client := getInsecureHTTPClient(5 * time.Second)

	resp, err := client.Get(VERSION_CHECK_URL)
	if err != nil {
		logMsg(fmt.Sprintf("Failed to fetch version: %v", err))
		return "Failed to fetch version"
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		logMsg(fmt.Sprintf("Version check returned status: %d", resp.StatusCode))
		return "Failed to fetch version"
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logMsg(fmt.Sprintf("Failed to read version response: %v", err))
		return "Failed to fetch version"
	}

	var remoteVersion VersionInfo
	if err := json.Unmarshal(body, &remoteVersion); err != nil {
		logMsg(fmt.Sprintf("Failed to parse version JSON: %v", err))
		return "Failed to fetch version"
	}

	// Compare versions
	if remoteVersion.Version == APP_VERSION {
		logMsg(fmt.Sprintf("Version check: Up to date (%s)", APP_VERSION))
		return "Up to date"
	}

	logMsg(fmt.Sprintf("Version check: Update available (current: %s, latest: %s)", APP_VERSION, remoteVersion.Version))
	return "New update available! Visit miyoopod.fragoso.dev for instructions"
}
