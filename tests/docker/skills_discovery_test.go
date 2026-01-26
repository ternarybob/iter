package docker

import (
	"os/exec"
	"strings"
	"testing"
)

// TestSkillsDiscovery tests that all iter skills are discoverable in Claude after plugin installation.
// This verifies that skills like /iter:workflow, /iter:run, /iter:index, /iter:search, and /iter:test
// are properly registered and appear as available skills in Claude Code.
func TestSkillsDiscovery(t *testing.T) {
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

	// Test script that verifies all skills are discoverable
	testScript := `
		set -e

		# Install plugin
		claude plugin marketplace add /home/testuser/iter-plugin
		claude plugin install iter@iter-local

		# Get plugin install path
		PLUGIN_PATH=$(claude plugin list --json | jq -r '.[] | select(.id == "iter@iter-local") | .installPath')
		echo "Plugin path: $PLUGIN_PATH"

		# Expected skills from the plugin
		EXPECTED_SKILLS="run iter-workflow iter-index iter-search iter-test install"

		echo ""
		echo "=== Testing skill presence in plugin directory ==="
		for skill in $EXPECTED_SKILLS; do
			SKILL_FILE="$PLUGIN_PATH/skills/$skill/SKILL.md"
			if [ -f "$SKILL_FILE" ]; then
				# Check SKILL.md has required fields
				if grep -q "^name:" "$SKILL_FILE" && grep -q "^description:" "$SKILL_FILE"; then
					echo "OK: $skill skill has valid SKILL.md"
				else
					echo "FAIL: $skill skill SKILL.md missing required fields"
					exit 1
				fi
			else
				echo "FAIL: $skill skill not found at $SKILL_FILE"
				exit 1
			fi
		done

		echo ""
		echo "=== Testing skill discoverability in Claude ==="

		# Create a test git repo (required for Claude)
		cd /tmp && git init -q test-skills-discovery && cd test-skills-discovery
		git config user.email "test@test.com"
		git config user.name "Test"

		# Test that each plugin skill is recognized by Claude
		# We test by invoking with --help which should work if skill is discovered
		for skill in run iter-workflow iter-index iter-search iter-test; do
			echo "Testing /iter:$skill discovery..."

			# Try to invoke the skill with --help or -v (should work if discovered)
			OUTPUT=$(timeout 60 claude -p "/iter:$skill --help" 2>&1 || true)

			# Check if skill was recognized (not "Unknown skill" error)
			if echo "$OUTPUT" | grep -q "Unknown skill"; then
				echo "FAIL: /iter:$skill not recognized (Unknown skill error)"
				echo "Output: $OUTPUT"
				exit 1
			else
				echo "OK: /iter:$skill is recognized by Claude"
			fi
		done

		# Special test for iter-index (test with 'status' command)
		echo ""
		echo "Testing /iter:iter-index status..."
		OUTPUT=$(timeout 60 claude -p "/iter:iter-index status" 2>&1 || true)
		if echo "$OUTPUT" | grep -q "Unknown skill"; then
			echo "FAIL: /iter:iter-index status not working"
			echo "Output: $OUTPUT"
			exit 1
		else
			echo "OK: /iter:iter-index status works"
		fi

		# Test for iter-search (should work even with no index)
		echo ""
		echo "Testing /iter:iter-search..."
		OUTPUT=$(timeout 60 claude -p "/iter:iter-search 'test'" 2>&1 || true)
		if echo "$OUTPUT" | grep -q "Unknown skill"; then
			echo "FAIL: /iter:iter-search not working"
			echo "Output: $OUTPUT"
			exit 1
		else
			echo "OK: /iter:iter-search works"
		fi

		# Test iter:run with -v flag (version check)
		echo ""
		echo "Testing /iter:run -v (version)..."
		OUTPUT=$(timeout 60 claude -p "/iter:run -v" 2>&1)
		if echo "$OUTPUT" | grep -qE "(iter version|version.*[0-9]+\.[0-9]+)"; then
			echo "OK: /iter:run -v shows version"
		else
			echo "FAIL: /iter:run -v did not show version"
			echo "Output: $OUTPUT"
			exit 1
		fi

		echo ""
		echo "=== All skills discovery tests passed ==="
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
	if !strings.Contains(outputStr, "All skills discovery tests passed") {
		t.Errorf("Skills discovery tests did not pass")
	}

	// Check no skills had "Unknown skill" errors
	if strings.Contains(outputStr, "Unknown skill") {
		t.Errorf("Some skills were not recognized by Claude")
	}

	// Verify all expected skills were found
	expectedSkills := []string{"run", "iter-workflow", "iter-index", "iter-search", "iter-test", "install"}
	for _, skill := range expectedSkills {
		if !strings.Contains(outputStr, "OK: "+skill+" skill has valid SKILL.md") {
			t.Errorf("Skill %s was not found or invalid", skill)
		}
	}

	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}
}
