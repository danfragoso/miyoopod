package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func main() {
	// Get current working directory (should be project root when run from Makefile)
	projectRoot, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting working directory: %v\n", err)
		os.Exit(1)
	}

	// Load .env file
	envPath := filepath.Join(projectRoot, ".env")
	posthogToken := ""

	if data, err := os.ReadFile(envPath); err == nil {
		envContent := string(data)

		// Extract PostHog token
		re := regexp.MustCompile(`POSTHOG_TOKEN=(.+)`)
		if matches := re.FindStringSubmatch(envContent); len(matches) > 1 {
			posthogToken = strings.TrimSpace(matches[1])
			fmt.Println("✓ Loaded PostHog token from .env")
		}
	}

	if posthogToken == "" {
		fmt.Println("⚠ No PostHog token found in .env, building without observability")
	}

	// Read types.go to get current version
	typesGoPath := filepath.Join(projectRoot, "src", "types.go")
	typesContent, err := os.ReadFile(typesGoPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading types.go: %v\n", err)
		os.Exit(1)
	}

	version := "0.0.0"
	re := regexp.MustCompile(`APP_VERSION\s*=\s*"([^"]+)"`)
	if matches := re.FindStringSubmatch(string(typesContent)); len(matches) > 1 {
		version = matches[1]
	}

	fmt.Printf("✓ Building version %s\n", version)

	// Update posthog.go with token
	posthogGoPath := filepath.Join(projectRoot, "src", "posthog.go")
	posthogContent, err := os.ReadFile(posthogGoPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading posthog.go: %v\n", err)
		os.Exit(1)
	}

	re = regexp.MustCompile(`const POSTHOG_TOKEN = ".*" // Injected at build time`)
	updatedContent := re.ReplaceAllString(string(posthogContent), fmt.Sprintf(`const POSTHOG_TOKEN = "%s" // Injected at build time`, posthogToken))

	if err := os.WriteFile(posthogGoPath, []byte(updatedContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing posthog.go: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Injected PostHog token into posthog.go")

	fmt.Println("\n✓ Build preparation complete\n")
}
