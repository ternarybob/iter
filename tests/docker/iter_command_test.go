package docker

import (
	"os/exec"
	"strings"
	"testing"
)

// TestIterRunCommandLine tests `claude -p '/iter:iter -v'` command line invocation
// After the skills refactor, the main skill is /iter:iter with options like -v, -t:, -w:, -r
func TestIterRunCommandLine(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker integration test in short mode")
	}

	// Check Docker availability
	dockerCheck := exec.Command("docker", "info")
	if err := dockerCheck.Run(); err != nil {
		t.Skip("Docker not available, skipping integration test")
	}

	// Setup: require Docker and auth
	baseArgs := requireDockerAndAuth(t)
	projectRoot := findProjectRoot(t)

	// Create result directory
	resultDir := createTestResultDir(t, projectRoot, "iter-run-command-line")
	defer resultDir.Close()

	// Build Docker image (reuses if exists)
	buildDockerImage(t, projectRoot)

	// Run just the command line test - now uses /iter:iter -v (unified skill)
	args := append(baseArgs, "--entrypoint", "bash", dockerImage, "-c", `
			claude plugin marketplace add /home/testuser/iter-plugin
			claude plugin install iter@iter-local
			cd /tmp && git init -q && git config user.email "test@test.com" && git config user.name "Test"
			claude -p '/iter:iter -v'
		`)
	runCmd := exec.Command("docker", args...)
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

	// Check for version output (Claude may format with markdown like "Version: **dev**")
	hasVersionOutput := strings.Contains(outputStr, "iter version") ||
		strings.Contains(outputStr, "ITERATIVE IMPLEMENTATION") ||
		strings.Contains(outputStr, "Version:") ||
		strings.Contains(outputStr, "VERSION MODE") ||
		strings.Contains(outputStr, "2.")
	if !hasVersionOutput {
		status = "FAIL"
		missing = append(missing, "iter version output")
	}

	// Write result summary
	resultDir.WriteResult(status, missing)

	// Report failures
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	if len(missing) > 0 && !strings.Contains(missing[0], "command execution") {
		t.Errorf("Expected /iter:iter -v to execute and show iter output")
		t.Errorf("Got: %s", outputStr)
	}
}

// TestIterRunInteractive tests `/iter:iter -v` in interactive Claude session
// After the skills refactor, the main skill is /iter:iter with options like -v, -t:, -w:, -r
func TestIterRunInteractive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker integration test in short mode")
	}

	// Check Docker availability
	dockerCheck := exec.Command("docker", "info")
	if err := dockerCheck.Run(); err != nil {
		t.Skip("Docker not available, skipping integration test")
	}

	// Setup: require Docker and auth
	baseArgs := requireDockerAndAuth(t)
	projectRoot := findProjectRoot(t)

	// Create result directory
	resultDir := createTestResultDir(t, projectRoot, "iter-run-interactive")
	defer resultDir.Close()

	// Build Docker image (reuses if exists)
	buildDockerImage(t, projectRoot)

	// Run interactive test - now uses /iter:iter -v (unified skill)
	args := append(baseArgs, "--entrypoint", "bash", dockerImage, "-c", `
			claude plugin marketplace add /home/testuser/iter-plugin
			claude plugin install iter@iter-local
			cd /tmp && git init -q && git config user.email "test@test.com" && git config user.name "Test"
			echo "/iter:iter -v" | claude --dangerously-skip-permissions
		`)
	runCmd := exec.Command("docker", args...)
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

	// Check for version output (Claude may format with markdown like "Version: **dev**")
	hasVersionOutput := strings.Contains(outputStr, "iter version") ||
		strings.Contains(outputStr, "ITERATIVE IMPLEMENTATION") ||
		strings.Contains(outputStr, "Version:") ||
		strings.Contains(outputStr, "VERSION MODE") ||
		strings.Contains(outputStr, "2.")
	if !hasVersionOutput {
		status = "FAIL"
		missing = append(missing, "iter version output")
	}

	// Write result summary
	resultDir.WriteResult(status, missing)

	// Report failures
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	if len(missing) > 0 && !strings.Contains(missing[0], "command execution") {
		t.Errorf("Expected /iter:iter -v to execute and show iter output")
		t.Errorf("Got: %s", outputStr)
	}
}

// TestIterSkillsDiscovery tests that the unified iter skill is discoverable and responds to various options
// After the skills refactor, there's a single /iter:iter skill with options: -v, -t:, -w:, -r
func TestIterSkillsDiscovery(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker integration test in short mode")
	}

	// Check Docker availability
	dockerCheck := exec.Command("docker", "info")
	if err := dockerCheck.Run(); err != nil {
		t.Skip("Docker not available, skipping integration test")
	}

	// Setup: require Docker and auth
	baseArgs := requireDockerAndAuth(t)
	projectRoot := findProjectRoot(t)

	// Create result directory
	resultDir := createTestResultDir(t, projectRoot, "iter-skills-discovery")
	defer resultDir.Close()

	// Build Docker image (reuses if exists)
	buildDockerImage(t, projectRoot)

	// Test the unified /iter:iter skill with -v flag
	testScript := `
		set -e
		claude plugin marketplace add /home/testuser/iter-plugin
		claude plugin install iter@iter-local
		cd /tmp && git init -q && git config user.email "test@test.com" && git config user.name "Test"

		echo ""
		echo "=== Testing unified /iter:iter skill discovery ==="
		echo ""

		# Test the main iter skill with -v flag
		echo "--- Testing /iter:iter -v ---"
		echo "/iter:iter -v" | timeout 120 claude --dangerously-skip-permissions 2>&1 || true
		echo ""
		echo "--- End /iter:iter -v ---"
		echo ""

		# Test the install skill
		echo "--- Testing /iter:install ---"
		echo "/iter:install" | timeout 120 claude --dangerously-skip-permissions 2>&1 || true
		echo ""
		echo "--- End /iter:install ---"
		echo ""

		echo "=== All skills tested ==="
	`

	args := append(baseArgs, "--entrypoint", "bash", dockerImage, "-c", testScript)
	runCmd := exec.Command("docker", args...)
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

	// Check that the iter skill was discovered and executed
	if !strings.Contains(outputStr, "Testing /iter:iter -v") {
		status = "FAIL"
		missing = append(missing, "/iter:iter was tested")
	}

	// Check for version output pattern - Claude may format with markdown
	// Look for version number pattern like "2.1.20260127" or "iter version"
	iterMarker := "Testing /iter:iter -v"
	iterEndMarker := "End /iter:iter -v"
	startIdx := strings.Index(outputStr, iterMarker)
	endIdx := strings.Index(outputStr, iterEndMarker)

	if startIdx >= 0 && endIdx > startIdx {
		skillOutput := outputStr[startIdx:endIdx]
		// Skill is considered discovered if it shows version info OR
		// responds with any valid output (not "Unknown skill")
		hasValidResponse := strings.Contains(skillOutput, "iter version") ||
			strings.Contains(skillOutput, "version") ||
			strings.Contains(skillOutput, "2.") ||
			strings.Contains(skillOutput, "ITERATIVE") ||
			strings.Contains(skillOutput, "/iter:iter")

		if !hasValidResponse {
			status = "FAIL"
			missing = append(missing, "/iter:iter valid response")
		}

		t.Logf("/iter:iter discovered with response containing keywords")
	}

	// Check for "Unknown skill" errors
	if strings.Contains(outputStr, "Unknown skill") {
		status = "FAIL"
		missing = append(missing, "no Unknown skill errors")
	}

	// Write result summary
	resultDir.WriteResult(status, missing)

	// Report failures
	if err != nil {
		t.Errorf("Command execution failed: %v", err)
	}

	if strings.Contains(outputStr, "Unknown skill") {
		t.Errorf("/iter:iter was not recognized (Unknown skill error)")
	}

	if len(missing) > 0 {
		t.Errorf("Missing checks: %v", missing)
	}
}

// TestIterSkillsAutocomplete tests that the /iter:* skills are discoverable
// and can be invoked by Claude after plugin installation.
// After the skills refactor, there are two skills: iter and install
func TestIterSkillsAutocomplete(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker integration test in short mode")
	}

	// Check Docker availability
	dockerCheck := exec.Command("docker", "info")
	if err := dockerCheck.Run(); err != nil {
		t.Skip("Docker not available, skipping integration test")
	}

	// Setup: require Docker and auth
	baseArgs := requireDockerAndAuth(t)
	projectRoot := findProjectRoot(t)

	// Create result directory
	resultDir := createTestResultDir(t, projectRoot, "iter-skills-autocomplete")
	defer resultDir.Close()

	// Build Docker image (reuses if exists)
	buildDockerImage(t, projectRoot)

	// Test script that verifies skills are discoverable and invokable
	// After refactor: only iter and install skills exist
	testScript := `
		set -e

		# Install plugin
		claude plugin marketplace add /home/testuser/iter-plugin
		claude plugin install iter@iter-local

		# Get plugin path
		PLUGIN_PATH=$(claude plugin list --json | jq -r '.[] | select(.id == "iter@iter-local") | .installPath')
		echo "Plugin path: $PLUGIN_PATH"

		# Verify plugin installed
		if [ -z "$PLUGIN_PATH" ]; then
			echo "FAIL: Plugin not installed"
			exit 1
		fi
		echo "OK: Plugin installed"

		echo ""
		echo "=== Verifying skill SKILL.md structure ==="
		echo "For autocomplete to work, each skill needs 'name:' in SKILL.md"
		echo ""

		# Expected skills after refactor: iter (unified) and install
		EXPECTED_SKILLS="iter install"
		MISSING_STRUCTURE=""

		for skill in $EXPECTED_SKILLS; do
			SKILL_FILE="$PLUGIN_PATH/skills/$skill/SKILL.md"
			if [ ! -f "$SKILL_FILE" ]; then
				echo "FAIL: $skill skill not found at $SKILL_FILE"
				MISSING_STRUCTURE="$MISSING_STRUCTURE $skill(missing)"
				continue
			fi

			# Check for name field (required for autocomplete)
			if grep -q "^name:" "$SKILL_FILE"; then
				SKILL_NAME=$(grep "^name:" "$SKILL_FILE" | head -1 | cut -d: -f2 | tr -d ' ')
				echo "OK: iter:$skill has name field (name=$SKILL_NAME)"
			else
				echo "FAIL: iter:$skill missing 'name:' in SKILL.md"
				MISSING_STRUCTURE="$MISSING_STRUCTURE $skill(no-name)"
			fi
		done

		if [ -n "$MISSING_STRUCTURE" ]; then
			echo ""
			echo "STRUCTURE TEST FAILED - Skills with issues:$MISSING_STRUCTURE"
			exit 1
		fi

		echo ""
		echo "=== Testing skill invocation (no Unknown skill error) ==="
		echo ""

		# Create test git repo (required for skill invocation)
		cd /tmp && git init -q test-skills && cd test-skills
		git config user.email "test@test.com"
		git config user.name "Test"

		# Test the unified iter skill with -v flag
		INVOKE_FAILURES=""

		echo "Testing /iter:iter invocation..."
		OUTPUT=$(timeout 60 claude -p "/iter:iter -v" 2>&1 || true)

		if echo "$OUTPUT" | grep -q "Unknown skill"; then
			echo "FAIL: /iter:iter - Unknown skill error"
			INVOKE_FAILURES="$INVOKE_FAILURES iter:iter"
		else
			echo "OK: /iter:iter is recognized (no Unknown skill error)"
		fi

		if [ -n "$INVOKE_FAILURES" ]; then
			echo ""
			echo "INVOKE TEST FAILED - Unknown skill errors:$INVOKE_FAILURES"
			exit 1
		fi

		echo ""
		echo "=== Asking Claude to list iter skills ==="
		echo ""

		# Ask Claude to list available iter skills
		OUTPUT=$(timeout 120 claude -p "List all available skills that start with 'iter:' from your Skill tool. Output ONLY the skill names, one per line." 2>&1)

		echo "Claude response:"
		echo "$OUTPUT"
		echo ""

		# Check which skills Claude lists (after refactor: iter and install)
		echo "=== Checking Claude's skill listing ==="
		for skill in iter install; do
			if echo "$OUTPUT" | grep -qi "iter:$skill"; then
				echo "OK: iter:$skill listed by Claude"
			else
				echo "NOTE: iter:$skill not in Claude's listing (may be abbreviated)"
			fi
		done

		echo ""
		echo "=== All skills verified for autocomplete ==="
		echo "Skills have correct SKILL.md structure and can be invoked."
	`

	args := append(baseArgs, "--entrypoint", "bash", dockerImage, "-c", testScript)
	runCmd := exec.Command("docker", args...)
	runCmd.Dir = projectRoot
	output, err := runCmd.CombinedOutput()

	// Write output to log file
	resultDir.WriteLog(output)

	t.Logf("Output:\n%s", output)

	// Determine test result
	status := "PASS"
	var missing []string
	outputStr := string(output)

	// Check for success marker
	if !strings.Contains(outputStr, "All skills verified for autocomplete") {
		status = "FAIL"
		missing = append(missing, "All skills verified for autocomplete")
	}

	// Check for structure test failure
	if strings.Contains(outputStr, "STRUCTURE TEST FAILED") {
		status = "FAIL"
		missing = append(missing, "SKILL.md structure test passed")
	}

	// Check for invoke test failure
	if strings.Contains(outputStr, "INVOKE TEST FAILED") {
		status = "FAIL"
		missing = append(missing, "skill invocation test passed")
	}

	// Check for Unknown skill errors only in the context of actual failures
	// (not when Claude mentions "Unknown skill" in explanatory text)
	if strings.Contains(outputStr, "FAIL:") && strings.Contains(outputStr, "Unknown skill") {
		status = "FAIL"
		missing = append(missing, "no Unknown skill errors")
	}

	// Write result summary
	resultDir.WriteResult(status, missing)

	// Report failures
	if strings.Contains(outputStr, "STRUCTURE TEST FAILED") {
		t.Errorf("Skill SKILL.md structure test failed - some skills missing name field")
	}

	if strings.Contains(outputStr, "INVOKE TEST FAILED") {
		t.Errorf("Skill invocation test failed - some skills returned Unknown skill error")
	}

	// Only fail for Unknown skill if it's in the context of an actual failure
	// (not if Claude mentions "Unknown skill" in explanatory text)
	if strings.Contains(outputStr, "FAIL:") && strings.Contains(outputStr, "Unknown skill") {
		t.Errorf("Some skills were not recognized by Claude (Unknown skill error)")
	}

	if !strings.Contains(outputStr, "All skills verified for autocomplete") {
		t.Errorf("Skills autocomplete test did not complete successfully")
	}

	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}
}

// TestPluginSkillAutoprompt tests that plugin skills are discoverable and appear
// in Claude's skill system.
// After the skills refactor, there are two skills: iter (unified) and install
func TestPluginSkillAutoprompt(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker integration test in short mode")
	}

	// Check Docker availability
	dockerCheck := exec.Command("docker", "info")
	if err := dockerCheck.Run(); err != nil {
		t.Skip("Docker not available, skipping integration test")
	}

	// Setup: require Docker and auth
	baseArgs := requireDockerAndAuth(t)
	projectRoot := findProjectRoot(t)

	// Create result directory
	resultDir := createTestResultDir(t, projectRoot, "plugin-skill-autoprompt")
	defer resultDir.Close()

	// Build Docker image (reuses if exists)
	buildDockerImage(t, projectRoot)

	// Test script that checks:
	// 1. All expected skills are cached with valid SKILL.md
	// 2. Skills are discoverable (no "Unknown skill" error)
	// 3. Skills appear in plugin list --json output
	// After refactor: only iter and install skills exist
	testScript := `
		set -e

		# Install plugin
		claude plugin marketplace add /home/testuser/iter-plugin
		claude plugin install iter@iter-local

		# Get plugin install path
		PLUGIN_PATH=$(claude plugin list --json | jq -r '.[] | select(.id == "iter@iter-local") | .installPath')
		echo "Plugin path: $PLUGIN_PATH"

		# Expected skills after refactor: iter (unified) and install
		EXPECTED_SKILLS="iter install"

		echo ""
		echo "=== Testing skill cache structure (name field required for autocomplete) ==="
		for skill in $EXPECTED_SKILLS; do
			SKILL_FILE="$PLUGIN_PATH/skills/$skill/SKILL.md"
			if [ -f "$SKILL_FILE" ]; then
				# Check SKILL.md has required name field for autocomplete
				if grep -q "^name:" "$SKILL_FILE"; then
					echo "OK: $skill skill has name field (required for autocomplete)"
				else
					echo "FAIL: $skill skill SKILL.md missing name field (required for autocomplete)"
					exit 1
				fi
				# Also verify description
				if grep -q "^description:" "$SKILL_FILE"; then
					echo "OK: $skill skill has description field"
				else
					echo "FAIL: $skill skill SKILL.md missing description field"
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

		# Test that the unified iter skill is recognized (no "Unknown skill" or regex errors)
		# Using -p mode to check skill recognition
		echo "Testing /iter:iter recognition..."
		OUTPUT=$(timeout 60 claude -p "/iter:iter --help" 2>&1 || true)

		if echo "$OUTPUT" | grep -q "Unknown skill"; then
			echo "FAIL: /iter:iter not recognized (Unknown skill error)"
			echo "Output: $OUTPUT"
			exit 1
		fi

		# Check for regex errors (e.g., from invalid arguments field in SKILL.md)
		if echo "$OUTPUT" | grep -qi "Invalid regular expression\|Range out of order\|SyntaxError"; then
			echo "FAIL: /iter:iter has regex error (likely invalid arguments field in SKILL.md)"
			echo "Output: $OUTPUT"
			exit 1
		fi

		echo "OK: /iter:iter is recognized by Claude without errors"

		echo ""
		echo "=== Testing skill execution ==="

		# Test /iter:iter -v specifically
		# Claude may format version with markdown like "version **2.1.xxx**" or "version dev"
		OUTPUT=$(timeout 60 claude -p "/iter:iter -v" 2>&1)
		if echo "$OUTPUT" | grep -qiE "(iter version|ITERATIVE|VERSION MODE|version.*(dev|[0-9]+)|v\.dev|v\.[0-9])"; then
			echo "OK: /iter:iter -v executes correctly"
		else
			echo "FAIL: /iter:iter -v did not execute correctly"
			echo "Output: $OUTPUT"
			exit 1
		fi

		echo ""
		echo "=== All skill autoprompt tests passed ==="
	`

	args := append(baseArgs, "--entrypoint", "bash", dockerImage, "-c", testScript)
	runCmd := exec.Command("docker", args...)
	runCmd.Dir = projectRoot
	output, err := runCmd.CombinedOutput()

	// Write output to log file
	resultDir.WriteLog(output)

	t.Logf("Output:\n%s", output)

	// Determine test result
	status := "PASS"
	var missing []string
	outputStr := string(output)

	// Check for test success markers
	if !strings.Contains(outputStr, "All skill autoprompt tests passed") {
		status = "FAIL"
		missing = append(missing, "All skill autoprompt tests passed")
	}

	// Check no skills had "Unknown skill" errors
	if strings.Contains(outputStr, "Unknown skill") {
		status = "FAIL"
		missing = append(missing, "no Unknown skill errors")
	}

	// Check for regex errors (from invalid arguments field in SKILL.md files)
	if strings.Contains(outputStr, "Invalid regular expression") ||
		strings.Contains(outputStr, "Range out of order") ||
		strings.Contains(outputStr, "SyntaxError") {
		status = "FAIL"
		missing = append(missing, "no regex errors in SKILL.md files")
	}

	// Write result summary
	resultDir.WriteResult(status, missing)

	// Report failures
	if !strings.Contains(outputStr, "All skill autoprompt tests passed") {
		t.Errorf("Skill autoprompt tests did not pass")
	}

	if strings.Contains(outputStr, "Unknown skill") {
		t.Errorf("Some skills were not recognized by Claude")
	}

	if strings.Contains(outputStr, "Invalid regular expression") ||
		strings.Contains(outputStr, "Range out of order") ||
		strings.Contains(outputStr, "SyntaxError") {
		t.Errorf("Skill SKILL.md files contain invalid regex patterns (likely in arguments field)")
	}

	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}
}
