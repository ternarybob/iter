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

// TestPluginInstallation tests that the iter plugin installs correctly
// in a fresh Docker container with Claude Code.
func TestPluginInstallation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker integration test in short mode")
	}

	// Check Docker availability
	dockerCheck := exec.Command("docker", "info")
	if err := dockerCheck.Run(); err != nil {
		t.Skip("Docker not available, skipping integration test")
	}

	// Get project root and API key
	projectRoot := findProjectRoot(t)
	apiKey := loadAPIKey(t, projectRoot)

	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY required")
	}

	// Build Docker image (reuses if exists)
	buildDockerImage(t, projectRoot)

	// Create results directory
	timestamp := time.Now().Format("20060102-150405")
	resultsDir := filepath.Join(projectRoot, "tests", "results", timestamp+"-plugin-installation")
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

	// Run Docker container
	t.Log("Running Docker test container...")
	logFile.WriteString("=== Docker Run ===\n")

	runArgs := []string{"run", "--rm"}
	if apiKey != "" {
		t.Log("API key found, running full Claude integration test")
		runArgs = append(runArgs, "-e", "ANTHROPIC_API_KEY="+apiKey)
	} else {
		t.Log("No API key found - test will FAIL (API key required)")
	}
	runArgs = append(runArgs, dockerImage)

	runCmd := exec.Command("docker", runArgs...)
	runCmd.Dir = projectRoot
	output, err := runCmd.CombinedOutput()

	// Write output to log file
	logFile.Write(output)

	// Log output for debugging
	t.Logf("Docker test output:\n%s", output)

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
		"OK: marketplace.json present",
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

	// Check SessionStart hook installed the /iter wrapper
	if !strings.Contains(outputStr, "OK: /iter wrapper skill installed") {
		missing = append(missing, "OK: /iter wrapper skill installed")
		status = "FAIL"
	}

	// Check /iter -v shortcut works
	if !strings.Contains(outputStr, "OK: /iter -v executed") {
		missing = append(missing, "OK: /iter -v executed")
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
		resultContent += "Missing:\n"
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
