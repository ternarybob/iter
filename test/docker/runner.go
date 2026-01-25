// Package main provides a Docker integration test runner for the iter plugin.
//
// Usage:
//
//	go run ./test/docker/runner.go
//
// This program:
// 1. Builds a Docker image with Claude Code CLI
// 2. Runs the plugin installation test
// 3. Saves results to test/results/{timestamp}-docker/
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Find project root
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("find project root: %w", err)
	}

	// Create results directory with timestamp
	timestamp := time.Now().Format("20060102-150405")
	resultsDir := filepath.Join(projectRoot, "test", "results", timestamp+"-docker")
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		return fmt.Errorf("create results dir: %w", err)
	}

	// Open log file
	logPath := filepath.Join(resultsDir, "test-output.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("create log file: %w", err)
	}
	defer logFile.Close()

	// Log helper
	log := func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		fmt.Print(msg)
		logFile.WriteString(msg)
	}

	log("==========================================\n")
	log("ITER PLUGIN DOCKER INTEGRATION TEST\n")
	log("==========================================\n")
	log("Timestamp: %s\n", timestamp)
	log("Results: %s\n", resultsDir)
	log("\n")

	// Check Docker is available
	log("Checking Docker availability...\n")
	dockerCheck := exec.Command("docker", "info")
	if err := dockerCheck.Run(); err != nil {
		return fmt.Errorf("Docker not available: %w", err)
	}
	log("OK: Docker is available\n\n")

	// Build Docker image
	log("==========================================\n")
	log("Building Docker test image...\n")
	log("==========================================\n")

	buildCmd := exec.Command("docker", "build", "--no-cache", "-t", "iter-plugin-test",
		"-f", "test/docker/Dockerfile", ".")
	buildCmd.Dir = projectRoot
	buildOutput, err := buildCmd.CombinedOutput()
	logFile.WriteString(string(buildOutput))

	if err != nil {
		log("FAIL: Docker build failed\n")
		log("%s\n", buildOutput)
		return fmt.Errorf("docker build failed: %w", err)
	}
	log("OK: Docker image built successfully\n\n")

	// Run Docker container
	log("==========================================\n")
	log("Running test container...\n")
	log("==========================================\n")

	// Pass API key if available for full integration test
	runArgs := []string{"run", "--rm"}
	if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
		log("API key detected, will run full Claude integration test\n")
		runArgs = append(runArgs, "-e", "ANTHROPIC_API_KEY="+apiKey)
	} else {
		log("No API key, will run offline simulation test\n")
	}
	runArgs = append(runArgs, "iter-plugin-test")

	runCmd := exec.Command("docker", runArgs...)
	runCmd.Dir = projectRoot
	runOutput, err := runCmd.CombinedOutput()

	// Always write output to log
	logFile.WriteString("\n--- Container Output ---\n")
	logFile.WriteString(string(runOutput))
	logFile.WriteString("--- End Container Output ---\n")

	// Also print to stdout
	fmt.Print(string(runOutput))

	if err != nil {
		log("\nFAIL: Docker test failed\n")
		writeResultFile(resultsDir, "FAIL", string(runOutput))
		return fmt.Errorf("docker test failed: %w", err)
	}

	// Verify expected output
	outputStr := string(runOutput)
	expectedStrings := []string{
		"Successfully added marketplace: iter-local",
		"Successfully installed plugin: iter@iter-local",
		"OK: iter@iter-local found in settings",
		"OK: SKILL.md has 'name' field",
		"OK: marketplace.json has 'skills' field",
		"OK: iter binary executes correctly",
		"OK: iter help works",
		"OK: iter run command executes correctly",
		"ALL TESTS PASSED",
	}

	var missing []string
	for _, expected := range expectedStrings {
		if !strings.Contains(outputStr, expected) {
			missing = append(missing, expected)
		}
	}

	if len(missing) > 0 {
		log("\nFAIL: Missing expected output:\n")
		for _, m := range missing {
			log("  - %s\n", m)
		}
		writeResultFile(resultsDir, "FAIL", fmt.Sprintf("Missing: %v", missing))
		return fmt.Errorf("missing expected output")
	}

	log("\n==========================================\n")
	log("TEST RESULT: SUCCESS\n")
	log("==========================================\n")
	log("Results saved to: %s\n", resultsDir)

	writeResultFile(resultsDir, "PASS", "All checks passed")
	return nil
}

func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Walk up looking for go.mod
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("could not find project root (go.mod)")
}

func writeResultFile(resultsDir, status, details string) {
	resultPath := filepath.Join(resultsDir, "result.txt")
	content := fmt.Sprintf("Status: %s\nTimestamp: %s\nDetails: %s\n",
		status, time.Now().Format(time.RFC3339), details)
	os.WriteFile(resultPath, []byte(content), 0644)
}
