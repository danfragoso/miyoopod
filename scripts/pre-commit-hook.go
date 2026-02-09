package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

func main() {
	// Get list of staged files
	cmd := exec.Command("git", "diff", "--cached", "--name-only")
	output, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting staged files: %v\n", err)
		os.Exit(1)
	}

	stagedFiles := strings.Split(strings.TrimSpace(string(output)), "\n")

	// Check if posthog.go is staged
	posthogStaged := false
	for _, file := range stagedFiles {
		if strings.HasSuffix(file, "src/posthog.go") {
			posthogStaged = true
			break
		}
	}

	if !posthogStaged {
		os.Exit(0) // Nothing to check
	}

	// Read the staged version of posthog.go
	cmd = exec.Command("git", "show", ":src/posthog.go")
	content, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading staged posthog.go: %v\n", err)
		os.Exit(1)
	}

	// Check if POSTHOG_TOKEN contains a non-empty value
	re := regexp.MustCompile(`const POSTHOG_TOKEN = "([^"]*)"`)
	matches := re.FindStringSubmatch(string(content))

	if len(matches) > 1 && matches[1] != "" {
		fmt.Fprintf(os.Stderr, "\n❌ COMMIT REJECTED: PostHog token found in src/posthog.go\n")
		fmt.Fprintf(os.Stderr, "The POSTHOG_TOKEN constant must be empty before committing.\n")
		fmt.Fprintf(os.Stderr, "\nFound: const POSTHOG_TOKEN = \"%s\"\n", matches[1])
		fmt.Fprintf(os.Stderr, "Expected: const POSTHOG_TOKEN = \"\"\n")
		fmt.Fprintf(os.Stderr, "\nTo fix this:\n")
		fmt.Fprintf(os.Stderr, "  1. Change POSTHOG_TOKEN to empty string in src/posthog.go\n")
		fmt.Fprintf(os.Stderr, "  2. Stage the file again: git add src/posthog.go\n")
		fmt.Fprintf(os.Stderr, "  3. Retry your commit\n\n")
		fmt.Fprintf(os.Stderr, "Note: The token is injected at build time via make\n\n")
		os.Exit(1)
	}

	fmt.Println("✓ PostHog token check passed")
	os.Exit(0)
}
