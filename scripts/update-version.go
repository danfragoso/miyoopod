package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type VersionJSON struct {
	Version   string `json:"version"`
	Checksum  string `json:"checksum"`
	URL       string `json:"url"`
	Size      int64  `json:"size"`
	Changelog string `json:"changelog"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Error: Version not provided")
		fmt.Fprintln(os.Stderr, "Usage: go run update-version.go <version> [zip-path] [changelog]")
		os.Exit(1)
	}

	version := os.Args[1]

	// Validate version format (basic semver check)
	matched, _ := regexp.MatchString(`^\d+\.\d+\.\d+$`, version)
	if !matched {
		fmt.Fprintln(os.Stderr, "Error: Version must be in format X.Y.Z (e.g., 1.0.0)")
		os.Exit(1)
	}

	fmt.Printf("Updating version to %s...\n", version)

	// Get current working directory (should be project root when run from Makefile)
	projectRoot, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting working directory: %v\n", err)
		os.Exit(1)
	}

	// Build version.json data
	vj := VersionJSON{
		Version: version,
		URL:     "https://github.com/danfragoso/miyoopod/raw/refs/heads/main/releases/MiyooPod.zip",
	}

	// If zip path provided, compute checksum and size
	if len(os.Args) >= 3 {
		zipPath := os.Args[2]
		data, err := os.ReadFile(zipPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading zip file: %v\n", err)
			os.Exit(1)
		}

		hash := sha256.Sum256(data)
		vj.Checksum = "sha256:" + hex.EncodeToString(hash[:])
		vj.Size = int64(len(data))

		fmt.Printf("  Zip size: %d bytes\n", vj.Size)
		fmt.Printf("  Checksum: %s\n", vj.Checksum)
	}

	// If changelog provided
	if len(os.Args) >= 4 {
		vj.Changelog = strings.Join(os.Args[3:], " ")
		fmt.Printf("  Changelog: %s\n", vj.Changelog)
	}

	// Write version.json
	versionJsonPath := filepath.Join(projectRoot, "version.json")
	jsonData, err := json.MarshalIndent(vj, "", "    ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
		os.Exit(1)
	}
	jsonData = append(jsonData, '\n')
	if err := os.WriteFile(versionJsonPath, jsonData, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing version.json: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Updated version.json")

	// Update types.go
	typesGoPath := filepath.Join(projectRoot, "src", "types.go")
	typesContent, err := os.ReadFile(typesGoPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading types.go: %v\n", err)
		os.Exit(1)
	}

	re := regexp.MustCompile(`APP_VERSION\s*=\s*"[^"]+"`)
	updatedContent := re.ReplaceAllString(string(typesContent), fmt.Sprintf(`APP_VERSION = "%s"`, version))

	if err := os.WriteFile(typesGoPath, []byte(updatedContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing types.go: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Updated src/types.go")

	fmt.Printf("\nVersion successfully updated to %s\n", version)
}
