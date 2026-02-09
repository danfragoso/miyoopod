package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Error: Version not provided")
		fmt.Fprintln(os.Stderr, "Usage: go run update-version.go <version>")
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

	// Update version.json
	versionJsonPath := filepath.Join(projectRoot, "version.json")
	versionJson := map[string]string{"version": version}
	data, err := json.MarshalIndent(versionJson, "", "    ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
		os.Exit(1)
	}
	data = append(data, '\n')
	if err := os.WriteFile(versionJsonPath, data, 0644); err != nil {
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
