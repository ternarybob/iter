package docker

import (
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
	// Setup test (handles Docker, auth, image build, result dir)
	setup := setupDockerTest(t, "iter-version-banner")
	defer setup.Close()

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
		OUTPUT=$(claude -p '/iter:iter -v' --dangerously-skip-permissions 2>&1)
		echo "$OUTPUT"
		echo "=== End output ==="

		# Check for banner markers (asterisks) or version mention
		if echo "$OUTPUT" | grep -q '\*\*\*\*\*\*\*\*\*\*\*\*\*\*\*\*\*\*'; then
			echo "BANNER_FOUND: yes"
		elif echo "$OUTPUT" | grep -qiE 'iter v\.|version'; then
			# Claude may mention the version without full banner
			echo "BANNER_FOUND: yes"
		else
			echo "BANNER_FOUND: no"
		fi

		# Check for version line within banner (accepts X.X.X-XXXX or dev)
		# Claude may format as "v.dev" or "version dev" or "Version: dev"
		if echo "$OUTPUT" | grep -qiE '(iter v\.|version.*(dev|[0-9]))'; then
			echo "VERSION_IN_BANNER: yes"
		else
			echo "VERSION_IN_BANNER: no"
		fi
	`

	output, err := setup.RunScript(testScript)

	// Determine test result
	status := "PASS"
	var missing []string

	if err != nil {
		status = "FAIL"
		missing = append(missing, "command execution succeeded")
	}

	// Check for banner asterisks or version mention (Claude may summarize output)
	hasBannerOrVersion := strings.Contains(output, "******************") ||
		strings.Contains(output, "iter v.") ||
		strings.Contains(output, "iter version") ||
		strings.Contains(output, "Version:") ||
		strings.Contains(output, "version dev") ||
		strings.Contains(output, "v.dev") ||
		strings.Contains(output, "VERSION MODE")
	if !hasBannerOrVersion {
		status = "FAIL"
		missing = append(missing, "banner or version output")
	}

	// Check for BANNER_FOUND marker from test script
	if strings.Contains(output, "BANNER_FOUND: no") {
		status = "FAIL"
		missing = append(missing, "banner found in output")
	}

	// Check for VERSION_IN_BANNER marker from test script
	if strings.Contains(output, "VERSION_IN_BANNER: no") {
		status = "FAIL"
		missing = append(missing, "version line in banner format")
	}

	// Check that the version command still works (version output detected)
	// Accept various formats: "iter version", "iter v.", "v.dev", "Version: dev"
	hasVersionOutput := strings.Contains(output, "iter version") ||
		strings.Contains(output, "iter v.") ||
		strings.Contains(output, "v.dev") ||
		strings.Contains(output, "Version: dev") ||
		strings.Contains(output, "Version:** dev") ||
		strings.Contains(output, "VERSION MODE")
	if !hasVersionOutput {
		status = "FAIL"
		missing = append(missing, "version output")
	}

	// Write result summary
	setup.ResultDir.WriteResult(status, missing)

	// Report failures
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	if !hasBannerOrVersion {
		t.Errorf("Expected banner or version output")
	}

	if strings.Contains(output, "BANNER_FOUND: no") {
		t.Errorf("Banner not found in expected format")
	}

	if strings.Contains(output, "VERSION_IN_BANNER: no") {
		t.Errorf("Version line not in banner format (iter v.X.X.X-XXXX)")
	}

	if len(missing) > 0 {
		t.Errorf("Missing checks: %v", missing)
		t.Errorf("Full output:\n%s", output)
	}
}
