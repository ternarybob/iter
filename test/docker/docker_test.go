// Package docker provides Docker-based integration tests for the iter plugin.
//
// These tests verify that the iter plugin installs correctly and that
// /iter:run commands execute properly in Claude.
//
// Run tests:
//
//	go test ./test/docker/... -v
//
// With API key for full Claude integration:
//
//	ANTHROPIC_API_KEY=sk-... go test ./test/docker/... -v
package docker

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestDockerPluginInstallation tests that the iter plugin installs correctly
// in a fresh Docker container with Claude Code.
func TestDockerPluginInstallation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker integration test in short mode")
	}

	// Check if Docker is available
	dockerCheck := exec.Command("docker", "info")
	if err := dockerCheck.Run(); err != nil {
		t.Skip("Docker not available, skipping integration test")
	}

	projectRoot := findProjectRoot(t)

	// Create results directory with timestamp
	timestamp := time.Now().Format("20060102-150405")
	resultsDir := filepath.Join(projectRoot, "test", "results", timestamp+"-docker")
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		t.Fatalf("Failed to create results directory: %v", err)
	}

	// Open log file
	logPath := filepath.Join(resultsDir, "test-output.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		t.Fatalf("Failed to create log file: %v", err)
	}
	defer logFile.Close()

	// Load .env file if it exists
	envFile := filepath.Join(projectRoot, "test", "docker", ".env")
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		if data, err := os.ReadFile(envFile); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "ANTHROPIC_API_KEY=") {
					apiKey = strings.TrimPrefix(line, "ANTHROPIC_API_KEY=")
					break
				}
			}
		}
	}

	// Build Docker image
	t.Log("Building Docker test image...")
	logFile.WriteString("=== Docker Build ===\n")

	buildCmd := exec.Command("docker", "build", "--no-cache", "-t", "iter-plugin-test",
		"-f", "test/docker/Dockerfile", ".")
	buildCmd.Dir = projectRoot
	buildOutput, err := buildCmd.CombinedOutput()
	logFile.Write(buildOutput)

	if err != nil {
		t.Fatalf("Failed to build Docker image: %v\nOutput: %s", err, buildOutput)
	}

	// Run Docker container
	t.Log("Running Docker test container...")
	logFile.WriteString("\n=== Docker Run ===\n")

	runArgs := []string{"run", "--rm"}
	if apiKey != "" {
		t.Log("API key found, running full Claude integration test")
		runArgs = append(runArgs, "-e", "ANTHROPIC_API_KEY="+apiKey)
	} else {
		t.Log("No API key found - test will FAIL (API key required)")
		t.Log("Set ANTHROPIC_API_KEY in environment or test/docker/.env")
	}
	runArgs = append(runArgs, "iter-plugin-test")

	runCmd := exec.Command("docker", runArgs...)
	runCmd.Dir = projectRoot
	output, err := runCmd.CombinedOutput()

	// Write output to log file
	logFile.Write(output)

	// Log output for debugging
	t.Logf("Docker test output:\n%s", output)
	t.Logf("Results saved to: %s", resultsDir)

	// Write result file
	outputStr := string(output)
	resultPath := filepath.Join(resultsDir, "result.txt")
	status := "PASS"

	// Check for expected outputs
	expectedStrings := []string{
		"Successfully added marketplace: iter-local",
		"Successfully installed plugin: iter@iter-local",
		"OK: iter@iter-local found in settings",
		"OK: SKILL.md has 'name' field",
		"OK: marketplace.json has 'skills' field",
		"OK: iter binary executes correctly",
		"OK: iter help works",
	}

	var missing []string
	for _, expected := range expectedStrings {
		if !strings.Contains(outputStr, expected) {
			missing = append(missing, expected)
			status = "FAIL"
		}
	}

	// Check Claude integration tests
	if !strings.Contains(outputStr, "OK: /iter:run -v executed via command line") {
		missing = append(missing, "OK: /iter:run -v executed via command line")
		status = "FAIL"
	}
	if !strings.Contains(outputStr, "OK: /iter:run -v executed in interactive mode") {
		missing = append(missing, "OK: /iter:run -v executed in interactive mode")
		status = "FAIL"
	}

	// Final check
	if !strings.Contains(outputStr, "ALL TESTS PASSED") {
		status = "FAIL"
	}

	// Write result file
	resultContent := fmt.Sprintf("Status: %s\nTimestamp: %s\nResultsDir: %s\n",
		status, time.Now().Format(time.RFC3339), resultsDir)
	if len(missing) > 0 {
		resultContent += fmt.Sprintf("Missing:\n")
		for _, m := range missing {
			resultContent += fmt.Sprintf("  - %s\n", m)
		}
	}
	os.WriteFile(resultPath, []byte(resultContent), 0644)

	// Report failures
	if err != nil {
		t.Fatalf("Docker integration test failed: %v", err)
	}

	for _, m := range missing {
		t.Errorf("Docker test output missing expected string: %q", m)
	}

	if status == "FAIL" {
		t.Errorf("Test failed - /iter:run command not executing properly in Claude")
	}
}

// TestIterRunCommandLine tests `claude -p '/iter:run -v'` command line invocation
func TestIterRunCommandLine(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker integration test in short mode")
	}

	dockerCheck := exec.Command("docker", "info")
	if err := dockerCheck.Run(); err != nil {
		t.Skip("Docker not available")
	}

	projectRoot := findProjectRoot(t)
	apiKey := loadAPIKey(t, projectRoot)

	if apiKey == "" {
		t.Fatal("ANTHROPIC_API_KEY required for this test. Set in environment or test/docker/.env")
	}

	// Build image first
	buildCmd := exec.Command("docker", "build", "-t", "iter-plugin-test", "-f", "test/docker/Dockerfile", ".")
	buildCmd.Dir = projectRoot
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Docker build failed: %v\n%s", err, output)
	}

	// Run just the command line test
	runCmd := exec.Command("docker", "run", "--rm",
		"-e", "ANTHROPIC_API_KEY="+apiKey,
		"--entrypoint", "bash",
		"iter-plugin-test",
		"-c", `
			claude plugin marketplace add /home/testuser/iter-plugin
			claude plugin install iter@iter-local
			cd /tmp && git init -q && git config user.email "test@test.com" && git config user.name "Test"
			claude -p '/iter:run -v'
		`)
	runCmd.Dir = projectRoot
	output, err := runCmd.CombinedOutput()

	t.Logf("Output:\n%s", output)

	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "iter version") && !strings.Contains(outputStr, "ITERATIVE IMPLEMENTATION") {
		t.Errorf("Expected /iter:run -v to execute and show iter output")
		t.Errorf("Got: %s", outputStr)
	}
}

// TestIterRunInteractive tests `/iter:run -v` in interactive Claude session
func TestIterRunInteractive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker integration test in short mode")
	}

	dockerCheck := exec.Command("docker", "info")
	if err := dockerCheck.Run(); err != nil {
		t.Skip("Docker not available")
	}

	projectRoot := findProjectRoot(t)
	apiKey := loadAPIKey(t, projectRoot)

	if apiKey == "" {
		t.Fatal("ANTHROPIC_API_KEY required for this test. Set in environment or test/docker/.env")
	}

	// Build image first
	buildCmd := exec.Command("docker", "build", "-t", "iter-plugin-test", "-f", "test/docker/Dockerfile", ".")
	buildCmd.Dir = projectRoot
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Docker build failed: %v\n%s", err, output)
	}

	// Run interactive test
	runCmd := exec.Command("docker", "run", "--rm",
		"-e", "ANTHROPIC_API_KEY="+apiKey,
		"--entrypoint", "bash",
		"iter-plugin-test",
		"-c", `
			claude plugin marketplace add /home/testuser/iter-plugin
			claude plugin install iter@iter-local
			cd /tmp && git init -q && git config user.email "test@test.com" && git config user.name "Test"
			echo "/iter:run -v" | claude --dangerously-skip-permissions
		`)
	runCmd.Dir = projectRoot
	output, err := runCmd.CombinedOutput()

	t.Logf("Output:\n%s", output)

	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "iter version") && !strings.Contains(outputStr, "ITERATIVE IMPLEMENTATION") {
		t.Errorf("Expected /iter:run -v to execute and show iter output")
		t.Errorf("Got: %s", outputStr)
	}
}

func findProjectRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	// Walk up looking for go.mod
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	t.Fatal("Could not find project root (go.mod)")
	return ""
}

func loadAPIKey(t *testing.T, projectRoot string) string {
	t.Helper()

	// First check environment
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		return key
	}

	// Try .env file
	envFile := filepath.Join(projectRoot, "test", "docker", ".env")
	if data, err := os.ReadFile(envFile); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "#") {
				continue
			}
			if strings.HasPrefix(line, "ANTHROPIC_API_KEY=") {
				return strings.TrimPrefix(line, "ANTHROPIC_API_KEY=")
			}
		}
	}

	return ""
}
