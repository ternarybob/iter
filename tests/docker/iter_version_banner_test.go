package docker

import (
	"os/exec"
	"strings"
	"testing"
)

// TestIterVersionBanner tests that `/iter:iter -v` displays the version banner.
// The banner format is:
//
//	******************
//	iter v.{version}
//	******************
//
// This test:
// 1. Starts with a clean container
// 2. Installs Claude Code
// 3. Installs the iter plugin from marketplace
// 4. Runs `/iter:iter -v`
// 5. Verifies the banner is present
func TestIterVersionBanner(t *testing.T) {
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

	// Create result directory
	resultDir := createTestResultDir(t, projectRoot, "iter-version-banner")
	defer resultDir.Close()

	// Prune Docker to ensure clean environment
	pruneDocker(t)

	// Build Docker image fresh
	buildDockerImage(t, projectRoot)

	// Run the version command and check for banner
	testScript := `
		set -e

		# Install plugin from marketplace
		claude plugin marketplace add /home/testuser/iter-plugin
		claude plugin install iter@iter-local

		# Create test git repo (required for Claude)
		cd /tmp && git init -q && git config user.email "test@test.com" && git config user.name "Test"

		# Run /iter:iter -v and capture output
		echo "=== Running /iter:iter -v ==="
		OUTPUT=$(claude -p '/iter:iter -v' 2>&1)
		echo "$OUTPUT"
		echo "=== End output ==="

		# Check for banner markers (asterisks)
		if echo "$OUTPUT" | grep -q '^\*\*\*\*\*\*\*\*\*\*\*\*\*\*\*\*\*\*$'; then
			echo "BANNER_FOUND: yes"
		else
			echo "BANNER_FOUND: no"
		fi

		# Check for version line within banner
		if echo "$OUTPUT" | grep -qE 'iter v\.[0-9]+\.[0-9]+\.[0-9]+-[0-9]+'; then
			echo "VERSION_IN_BANNER: yes"
		else
			echo "VERSION_IN_BANNER: no"
		fi
	`

	runCmd := exec.Command("docker", "run", "--rm",
		"-e", "ANTHROPIC_API_KEY="+apiKey,
		"--entrypoint", "bash",
		dockerImage,
		"-c", testScript)
	runCmd.Dir = projectRoot
	output, err := runCmd.CombinedOutput()

	// Write output to log file
	resultDir.WriteLog(output)

	t.Logf("Output:\n%s", output)

	// Determine test result
	status := "PASS"
	var missing []string
	outputStr := string(output)

	if err != nil {
		status = "FAIL"
		missing = append(missing, "command execution succeeded")
	}

	// Check for banner asterisks
	if !strings.Contains(outputStr, "******************") {
		status = "FAIL"
		missing = append(missing, "banner asterisks (******************)")
	}

	// Check for BANNER_FOUND marker from test script
	if strings.Contains(outputStr, "BANNER_FOUND: no") {
		status = "FAIL"
		missing = append(missing, "banner found in output")
	}

	// Check for VERSION_IN_BANNER marker from test script
	if strings.Contains(outputStr, "VERSION_IN_BANNER: no") {
		status = "FAIL"
		missing = append(missing, "version line in banner format")
	}

	// Check that the version command still works (iter version line present)
	if !strings.Contains(outputStr, "iter version") && !strings.Contains(outputStr, "iter v.") {
		status = "FAIL"
		missing = append(missing, "version output")
	}

	// Write result summary
	resultDir.WriteResult(status, missing)

	// Report failures
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	if !strings.Contains(outputStr, "******************") {
		t.Errorf("Expected banner asterisks (******************) in output")
	}

	if strings.Contains(outputStr, "BANNER_FOUND: no") {
		t.Errorf("Banner not found in expected format")
	}

	if strings.Contains(outputStr, "VERSION_IN_BANNER: no") {
		t.Errorf("Version line not in banner format (iter v.X.X.X-XXXX)")
	}

	if len(missing) > 0 {
		t.Errorf("Missing checks: %v", missing)
		t.Errorf("Full output:\n%s", outputStr)
	}
}
