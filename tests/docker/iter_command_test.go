package docker

import (
	"os/exec"
	"strings"
	"testing"
)

// TestIterRunCommandLine tests `claude -p '/iter:run -v'` command line invocation
func TestIterRunCommandLine(t *testing.T) {
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

// TestIterRunInteractive tests `/iter:run -v` in interactive Claude session
func TestIterRunInteractive(t *testing.T) {
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

// TestPluginSkillAutoprompt tests that plugin skills are discoverable and appear
// in Claude's skill system.
func TestPluginSkillAutoprompt(t *testing.T) {
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
