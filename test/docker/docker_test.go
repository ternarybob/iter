// Package docker provides Docker-based integration tests for the iter plugin.
//
// Test Execution Order:
// These tests use t.Run() subtests to guarantee sequential execution.
// The TestDockerIntegration parent test builds the Docker image once,
// then runs all subtests in order:
//
//  1. PluginInstallation - Full installation and /iter:run test
//  2. IterRunCommandLine - Tests `claude -p '/iter:run -v'`
//  3. IterRunInteractive - Tests `/iter:run -v` in interactive session
//  4. PluginSkillAutoprompt - Tests skill discoverability
//
// Run tests:
//
//	go test ./test/docker/... -v
//
// With API key for full Claude integration:
//
//	ANTHROPIC_API_KEY=sk-... go test ./test/docker/... -v
//
// Note: Tests run sequentially (not in parallel) to avoid Docker resource contention.
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

const dockerImage = "iter-plugin-test"

// pruneDocker cleans up Docker images and volumes before tests.
// This ensures a clean environment for testing.
func pruneDocker(t *testing.T) {
	t.Helper()
	t.Log("Pruning Docker images and volumes...")

	// Prune unused images
	pruneImages := exec.Command("docker", "image", "prune", "-af")
	if output, err := pruneImages.CombinedOutput(); err != nil {
		t.Logf("Warning: docker image prune failed: %v\n%s", err, output)
	}

	// Prune unused volumes
	pruneVolumes := exec.Command("docker", "volume", "prune", "-f")
	if output, err := pruneVolumes.CombinedOutput(); err != nil {
		t.Logf("Warning: docker volume prune failed: %v\n%s", err, output)
	}

	// Remove iter-plugin-test image if it exists
	removeImage := exec.Command("docker", "rmi", "-f", dockerImage)
	removeImage.Run() // Ignore errors - image may not exist

	t.Log("Docker cleanup complete")
}

// buildDockerImage builds the test Docker image once for all subtests.
func buildDockerImage(t *testing.T, projectRoot string) {
	t.Helper()
	t.Log("Building Docker test image...")

	buildCmd := exec.Command("docker", "build", "--no-cache", "-t", dockerImage,
		"-f", "test/docker/Dockerfile", ".")
	buildCmd.Dir = projectRoot
	buildOutput, err := buildCmd.CombinedOutput()

	if err != nil {
		t.Fatalf("Failed to build Docker image: %v\nOutput: %s", err, buildOutput)
	}
	t.Log("Docker image built successfully")
}

// TestDockerIntegration is the parent test that runs all Docker integration tests
// in a guaranteed sequential order. It builds the Docker image once and shares it
// across all subtests.
func TestDockerIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker integration test in short mode")
	}

	// Check if Docker is available
	dockerCheck := exec.Command("docker", "info")
	if err := dockerCheck.Run(); err != nil {
		t.Skip("Docker not available, skipping integration test")
	}

	// Clean up Docker before all tests
	pruneDocker(t)

	projectRoot := findProjectRoot(t)
	apiKey := loadAPIKey(t, projectRoot)

	if apiKey == "" {
		t.Log("WARNING: No API key found - tests requiring Claude will fail")
		t.Log("Set ANTHROPIC_API_KEY in environment or test/docker/.env")
	}

	// Build Docker image once for all subtests
	buildDockerImage(t, projectRoot)

	// Create results directory for this test run
	timestamp := time.Now().Format("20060102-150405")
	resultsDir := filepath.Join(projectRoot, "test", "results", timestamp+"-docker")
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		t.Fatalf("Failed to create results directory: %v", err)
	}
	t.Logf("Results will be saved to: %s", resultsDir)

	// Run subtests in order - Go guarantees sequential execution within t.Run
	t.Run("1_PluginInstallation", func(t *testing.T) {
		runPluginInstallationTest(t, projectRoot, apiKey, resultsDir)
	})

	t.Run("2_IterRunCommandLine", func(t *testing.T) {
		if apiKey == "" {
			t.Skip("ANTHROPIC_API_KEY required")
		}
		runIterRunCommandLineTest(t, projectRoot, apiKey)
	})

	t.Run("3_IterRunInteractive", func(t *testing.T) {
		if apiKey == "" {
			t.Skip("ANTHROPIC_API_KEY required")
		}
		runIterRunInteractiveTest(t, projectRoot, apiKey)
	})

	t.Run("4_PluginSkillAutoprompt", func(t *testing.T) {
		if apiKey == "" {
			t.Skip("ANTHROPIC_API_KEY required")
		}
		runPluginSkillAutopromptTest(t, projectRoot, apiKey)
	})
}

// runPluginInstallationTest tests that the iter plugin installs correctly
// in a fresh Docker container with Claude Code.
func runPluginInstallationTest(t *testing.T, projectRoot, apiKey, resultsDir string) {
	t.Helper()

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

// runIterRunCommandLineTest tests `claude -p '/iter:run -v'` command line invocation
func runIterRunCommandLineTest(t *testing.T, projectRoot, apiKey string) {
	t.Helper()

	// Run just the command line test
	runCmd := exec.Command("docker", "run", "--rm",
		"-e", "ANTHROPIC_API_KEY="+apiKey,
		"--entrypoint", "bash",
		dockerImage,
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
	if !strings.Contains(outputStr, "iter version") && !strings.Contains(outputStr, "ITERATIVE IMPLEMENTATION") &&
		!strings.Contains(outputStr, "2.") {
		t.Errorf("Expected /iter:run -v to execute and show iter output")
		t.Errorf("Got: %s", outputStr)
	}
}

// runIterRunInteractiveTest tests `/iter:run -v` in interactive Claude session
func runIterRunInteractiveTest(t *testing.T, projectRoot, apiKey string) {
	t.Helper()

	// Run interactive test
	runCmd := exec.Command("docker", "run", "--rm",
		"-e", "ANTHROPIC_API_KEY="+apiKey,
		"--entrypoint", "bash",
		dockerImage,
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
	if !strings.Contains(outputStr, "iter version") && !strings.Contains(outputStr, "ITERATIVE IMPLEMENTATION") &&
		!strings.Contains(outputStr, "2.") {
		t.Errorf("Expected /iter:run -v to execute and show iter output")
		t.Errorf("Got: %s", outputStr)
	}
}

// runPluginSkillAutopromptTest tests that plugin skills are discoverable and appear
// in Claude's skill system.
func runPluginSkillAutopromptTest(t *testing.T, projectRoot, apiKey string) {
	t.Helper()

	// Test script that checks:
	// 1. All expected skills are cached with valid SKILL.md
	// 2. Skills are discoverable (no "Unknown skill" error)
	// 3. Skills appear in plugin list --json output
	testScript := `
		set -e

		# Install plugin
		claude plugin marketplace add /home/testuser/iter-plugin
		claude plugin install iter@iter-local

		# Get plugin install path
		PLUGIN_PATH=$(claude plugin list --json | jq -r '.[] | select(.id == "iter@iter-local") | .installPath')
		echo "Plugin path: $PLUGIN_PATH"

		# Expected skills
		EXPECTED_SKILLS="iter run iter-workflow iter-index iter-search install"

		echo ""
		echo "=== Testing skill cache structure ==="
		for skill in $EXPECTED_SKILLS; do
			SKILL_FILE="$PLUGIN_PATH/skills/$skill/SKILL.md"
			if [ -f "$SKILL_FILE" ]; then
				# Check SKILL.md has required fields
				if grep -q "^name:" "$SKILL_FILE"; then
					echo "OK: $skill skill has valid SKILL.md with name field"
				else
					echo "FAIL: $skill skill SKILL.md missing name field"
					exit 1
				fi
			else
				echo "FAIL: $skill skill not found at $SKILL_FILE"
				exit 1
			fi
		done

		echo ""
		echo "=== Testing skill discoverability ==="

		# Create a test git repo (required for Claude)
		cd /tmp && git init -q && git config user.email "test@test.com" && git config user.name "Test"

		# Test that each plugin skill is recognized (no "Unknown skill" error)
		# Using -p mode to check skill recognition
		for skill in run iter iter-workflow; do
			echo "Testing /iter:$skill recognition..."
			OUTPUT=$(timeout 60 claude -p "/iter:$skill --help" 2>&1 || true)

			if echo "$OUTPUT" | grep -q "Unknown skill"; then
				echo "FAIL: /iter:$skill not recognized (Unknown skill error)"
				echo "Output: $OUTPUT"
				exit 1
			else
				echo "OK: /iter:$skill is recognized by Claude"
			fi
		done

		echo ""
		echo "=== Testing skill execution ==="

		# Test /iter:run -v specifically
		# Claude may format version with markdown like "version **2.1.xxx**"
		OUTPUT=$(timeout 60 claude -p "/iter:run -v" 2>&1)
		if echo "$OUTPUT" | grep -qE "(iter version|ITERATIVE|version.*[0-9]+\.[0-9]+\.[0-9]+|2\.[0-9]+\.[0-9]+-[0-9]+)"; then
			echo "OK: /iter:run -v executes correctly"
		else
			echo "FAIL: /iter:run -v did not execute correctly"
			echo "Output: $OUTPUT"
			exit 1
		fi

		echo ""
		echo "=== All skill autoprompt tests passed ==="
	`

	runCmd := exec.Command("docker", "run", "--rm",
		"-e", "ANTHROPIC_API_KEY="+apiKey,
		"--entrypoint", "bash",
		dockerImage,
		"-c", testScript)
	runCmd.Dir = projectRoot
	output, err := runCmd.CombinedOutput()

	t.Logf("Output:\n%s", output)

	outputStr := string(output)

	// Check for test success markers
	if !strings.Contains(outputStr, "All skill autoprompt tests passed") {
		t.Errorf("Skill autoprompt tests did not pass")
	}

	// Check no skills had "Unknown skill" errors
	if strings.Contains(outputStr, "Unknown skill") {
		t.Errorf("Some skills were not recognized by Claude")
	}

	if err != nil {
		t.Fatalf("Test failed: %v", err)
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
