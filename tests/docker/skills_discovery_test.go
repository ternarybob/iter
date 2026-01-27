package docker

import (
	"strings"
	"testing"
)

// TestSkillsDiscovery tests that iter skills are discoverable in Claude after plugin installation.
// This verifies that the unified /iter:iter skill (for run/test/workflow modes) and /iter:install
// are properly registered and appear as available skills in Claude Code.
func TestSkillsDiscovery(t *testing.T) {
	// Setup test (handles Docker, auth, image build, result dir)
	setup := setupDockerTest(t, "skills-discovery")
	defer setup.Close()

	// Test script that verifies skills are discoverable with new unified syntax
	testScript := `
		set -e

		echo "=== Verifying clean Claude environment ==="
		# Ensure no existing plugins or user skills
		echo "Checking for clean plugin state..."
		EXISTING_PLUGINS=$(claude plugin list --json 2>/dev/null || echo "[]")
		if [ "$EXISTING_PLUGINS" != "[]" ]; then
			echo "WARNING: Found existing plugins (expected clean env): $EXISTING_PLUGINS"
		else
			echo "OK: No existing plugins (clean environment)"
		fi

		# Check no user skills exist
		if [ -d "$HOME/.claude/skills" ] && [ "$(ls -A $HOME/.claude/skills 2>/dev/null)" ]; then
			echo "WARNING: Found existing user skills (expected clean env)"
			ls -la "$HOME/.claude/skills/"
		else
			echo "OK: No existing user skills (clean environment)"
		fi

		echo ""
		echo "=== Installing plugin from clean state ==="
		# Install plugin
		claude plugin marketplace add /home/testuser/iter-plugin
		claude plugin install iter@iter-local

		# Get plugin install path
		PLUGIN_PATH=$(claude plugin list --json | jq -r '.[] | select(.id == "iter@iter-local") | .installPath')
		echo "Plugin path: $PLUGIN_PATH"

		# Expected skills from the plugin (new unified structure)
		EXPECTED_SKILLS="iter install"

		echo ""
		echo "=== Testing skill presence in plugin directory ==="
		for skill in $EXPECTED_SKILLS; do
			SKILL_FILE="$PLUGIN_PATH/skills/$skill/SKILL.md"
			if [ -f "$SKILL_FILE" ]; then
				# Check SKILL.md has required fields for autocomplete
				# The name field is REQUIRED for Claude Code to register skills in autocomplete
				if grep -q "^name:" "$SKILL_FILE"; then
					echo "OK: $skill skill has name field (required for autocomplete)"
				else
					echo "FAIL: $skill skill SKILL.md missing name field (required for autocomplete)"
					exit 1
				fi
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
		echo "=== Running /iter:install to create wrapper skill ==="

		# Create a test git repo (required for Claude)
		cd /tmp && git init -q test-skills-discovery && cd test-skills-discovery
		git config user.email "test@test.com"
		git config user.name "Test"

		# Run /iter:install to create the wrapper skill
		echo "Running /iter:install..."
		INSTALL_OUTPUT=$(timeout 120 claude -p "/iter:install" --dangerously-skip-permissions 2>&1 || true)
		echo "Install output:"
		echo "$INSTALL_OUTPUT"

		# Verify wrapper skill was created
		if [ -f "$HOME/.claude/skills/iter/SKILL.md" ]; then
			echo "OK: Wrapper skill created at $HOME/.claude/skills/iter/SKILL.md"
			echo "Wrapper skill contents:"
			cat "$HOME/.claude/skills/iter/SKILL.md"
		else
			echo "WARNING: Wrapper skill not created at $HOME/.claude/skills/iter/SKILL.md"
			echo "Checking if skill directories exist..."
			ls -la "$HOME/.claude/" 2>/dev/null || true
			ls -la "$HOME/.claude/skills/" 2>/dev/null || true
		fi

		echo ""
		echo "=== Testing unified /iter:iter skill discoverability ==="

		# Test the unified iter skill with -v flag
		echo "Testing /iter:iter -v (version)..."
		OUTPUT=$(timeout 90 claude -p "/iter:iter -v" --dangerously-skip-permissions --output-format json 2>&1)
		echo "JSON output: $OUTPUT"

		# Extract result from JSON
		RESULT=$(echo "$OUTPUT" | jq -r '.result // ""' 2>/dev/null || echo "$OUTPUT")
		echo "Parsed result: $RESULT"

		if echo "$RESULT" | grep -qiE "(iter version|version.*[0-9]+\.[0-9]+|VERSION MODE)"; then
			echo "OK: /iter:iter -v shows version"
		else
			if echo "$OUTPUT" | grep -qiE "(iter version|version.*[0-9]+\.[0-9]+|VERSION MODE)"; then
				echo "OK: /iter:iter -v shows version (in raw output)"
			else
				echo "FAIL: /iter:iter -v did not show version"
				echo "Full output: $OUTPUT"
				exit 1
			fi
		fi

		# Test unified syntax with different flags
		echo ""
		echo "=== Testing unified syntax modes ==="

		# Test -r (reindex) mode
		echo "Testing /iter:iter -r (reindex)..."
		OUTPUT=$(timeout 90 claude -p "/iter:iter -r" --dangerously-skip-permissions --output-format json 2>&1)
		RESULT=$(echo "$OUTPUT" | jq -r '.result // ""' 2>/dev/null || echo "$OUTPUT")
		if echo "$RESULT" | grep -qiE "(index|indexed|building)"; then
			echo "OK: /iter:iter -r triggers reindex"
		else
			if echo "$OUTPUT" | grep -qiE "(index|indexed|building)"; then
				echo "OK: /iter:iter -r triggers reindex (in raw output)"
			else
				echo "WARNING: /iter:iter -r may not have triggered reindex (check output)"
				echo "Output: $OUTPUT"
			fi
		fi

		# Test skill autocomplete listing
		echo ""
		echo "=== Testing skill autocomplete listing ==="
		echo "Asking Claude to list available /iter skills..."

		OUTPUT=$(timeout 90 claude -p "List all available skills that start with /iter (include both /iter and /iter:* skills). Just list the skill names, nothing else." --dangerously-skip-permissions 2>&1)
		echo "Claude skill listing output:"
		echo "$OUTPUT"

		# Verify the unified iter skill appears
		if echo "$OUTPUT" | grep -qi "iter:iter\|/iter:iter"; then
			echo "OK: /iter:iter appears in skill listing"
		else
			# It might appear as just "iter" in some contexts
			if echo "$OUTPUT" | grep -qi "iter"; then
				echo "OK: iter skill appears in skill listing"
			else
				echo "WARNING: iter skill may not appear in listing"
			fi
		fi

		# Verify install skill appears
		if echo "$OUTPUT" | grep -qi "iter:install\|/iter:install"; then
			echo "OK: /iter:install appears in skill listing"
		else
			echo "WARNING: /iter:install may not appear in listing"
		fi

		echo ""
		echo "=== All skills discovery tests passed ==="
	`

	output, err := setup.RunScript(testScript)

	// Determine test result
	status := "PASS"
	var missing []string

	// Check for test success markers
	if !strings.Contains(output, "All skills discovery tests passed") {
		status = "FAIL"
		missing = append(missing, "All skills discovery tests passed")
	}

	// Check no skills had "Unknown skill" errors
	if strings.Contains(output, "Unknown skill") {
		status = "FAIL"
		missing = append(missing, "no Unknown skill errors")
	}

	// Check wrapper skill was created by /iter:install
	if !strings.Contains(output, "Wrapper skill created at") {
		status = "FAIL"
		missing = append(missing, "wrapper skill creation via /iter:install")
	}

	// Verify all expected skills were found with required name field (for autocomplete)
	// New unified structure: only "iter" and "install"
	expectedSkills := []string{"iter", "install"}
	for _, skill := range expectedSkills {
		if !strings.Contains(output, "OK: "+skill+" skill has name field") {
			status = "FAIL"
			missing = append(missing, "OK: "+skill+" skill has name field")
		}
	}

	// Write result summary
	setup.ResultDir.WriteResult(status, missing)

	// Report failures
	if !strings.Contains(output, "All skills discovery tests passed") {
		t.Errorf("Skills discovery tests did not pass")
	}

	if strings.Contains(output, "Unknown skill") {
		t.Errorf("Some skills were not recognized by Claude")
	}

	if !strings.Contains(output, "Wrapper skill created at") {
		t.Errorf("/iter:install failed to create wrapper skill at ~/.claude/skills/iter/SKILL.md")
	}

	for _, skill := range expectedSkills {
		if !strings.Contains(output, "OK: "+skill+" skill has name field") {
			t.Errorf("Skill %s missing required name field for autocomplete", skill)
		}
	}

	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}
}
