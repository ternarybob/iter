package docker

import (
	"strings"
	"testing"
)

// TestPluginInstallation tests that the iter plugin installs correctly
// in a fresh Docker container with Claude Code.
// This test covers all 12 verification steps:
//  1. Check Claude version
//  2. Add marketplace
//  3. List marketplaces
//  4. Install plugin
//  5. Check settings
//  6. Check plugin cache structure
//  7. Test iter binary version
//  8. Test iter binary help
//  9. Test claude -p '/iter:iter -v'
//  10. Test interactive /iter:iter -v
//  11. Check /iter wrapper installation
//  12. Test /iter -v shortcut
func TestPluginInstallation(t *testing.T) {
	// Setup test (handles Docker, auth, image build, result dir)
	setup := setupDockerTest(t, "plugin-installation")
	defer setup.Close()

	// Comprehensive test script that covers all 12 steps from the original shell script
	testScript := `
set -e

echo "=========================================="
echo "ITER PLUGIN INSTALLATION TEST"
echo "=========================================="
echo ""

# [1/12] Check Claude version
echo "[1/12] Checking Claude Code version..."
claude --version || { echo "FAIL: Claude not installed"; exit 1; }
echo ""

# [2/12] Add marketplace
echo "[2/12] Adding local marketplace..."
claude plugin marketplace add /home/testuser/iter-plugin
if [ $? -ne 0 ]; then
    echo "FAIL: Could not add marketplace"
    exit 1
fi
echo ""

# [3/12] List marketplaces
echo "[3/12] Listing marketplaces..."
claude plugin marketplace list
echo ""

# [4/12] Install plugin
echo "[4/12] Installing iter plugin..."
claude plugin install iter@iter-local
if [ $? -ne 0 ]; then
    echo "FAIL: Could not install plugin"
    exit 1
fi
echo ""

# [5/12] Check settings
echo "[5/12] Checking settings for installed plugin..."
if [ -f ~/.claude/settings.json ]; then
    echo "Settings file contents:"
    cat ~/.claude/settings.json

    if grep -q "iter@iter-local" ~/.claude/settings.json; then
        echo ""
        echo "OK: iter@iter-local found in settings"
    else
        echo ""
        echo "FAIL: iter@iter-local NOT found in settings"
        exit 1
    fi
else
    echo "FAIL: Settings file not found"
    exit 1
fi
echo ""

# [6/12] Check plugin cache structure
echo "[6/12] Checking plugin cache structure..."
CACHE_DIR=~/.claude/plugins/cache/iter-local/iter
if [ -d "$CACHE_DIR" ]; then
    echo "Plugin cache directory exists"
    echo ""
    echo "Cached files:"
    find "$CACHE_DIR" -type f | head -30
    echo ""

    LATEST_VERSION=$(ls -t "$CACHE_DIR" | grep -v orphaned | head -1)
    echo "Latest version: $LATEST_VERSION"

    if [ -n "$LATEST_VERSION" ]; then
        SKILL_FILE="$CACHE_DIR/$LATEST_VERSION/skills/iter/SKILL.md"
        if [ -f "$SKILL_FILE" ]; then
            echo ""
            echo "Checking SKILL.md for 'name' field..."
            echo "--- SKILL.md contents ---"
            cat "$SKILL_FILE"
            echo "--- end ---"
            echo ""

            if grep -q "^name:" "$SKILL_FILE"; then
                echo "OK: SKILL.md has 'name' field"
            else
                echo "FAIL: SKILL.md missing 'name' field"
                exit 1
            fi
        else
            echo "FAIL: SKILL.md not found at $SKILL_FILE"
            exit 1
        fi

        MKT_FILE="$CACHE_DIR/$LATEST_VERSION/.claude-plugin/marketplace.json"
        if [ -f "$MKT_FILE" ]; then
            echo ""
            echo "Checking marketplace.json..."
            echo "--- marketplace.json ---"
            cat "$MKT_FILE"
            echo "--- end ---"
            echo ""
            echo "OK: marketplace.json present"

            # Skills are auto-discovered from skills/ directory
            SKILLS_DIR="$CACHE_DIR/$LATEST_VERSION/skills"
            if [ -d "$SKILLS_DIR" ]; then
                SKILL_COUNT=$(find "$SKILLS_DIR" -name "SKILL.md" | wc -l)
                echo "OK: Found $SKILL_COUNT skills in skills/ directory"
            else
                echo "FAIL: skills/ directory not found"
                exit 1
            fi
        fi

        # Store iter binary path for later tests
        ITER_BIN="$CACHE_DIR/$LATEST_VERSION/iter"
        export CLAUDE_PLUGIN_ROOT="$CACHE_DIR/$LATEST_VERSION"
    fi
else
    echo "FAIL: Plugin cache directory not found"
    exit 1
fi
echo ""

# [7/12] Test iter binary version
echo "[7/12] Testing iter binary version..."
if [ -x "$ITER_BIN" ]; then
    echo "--- iter version ---"
    "$ITER_BIN" version
    echo "--- end ---"
    echo "OK: iter binary executes correctly"
else
    echo "FAIL: iter binary not found or not executable"
    exit 1
fi
echo ""

# [8/12] Test iter binary help
echo "[8/12] Testing iter binary help..."
echo "--- iter help ---"
"$ITER_BIN" help 2>&1 | head -20
echo "--- end ---"
echo "OK: iter help works"
echo ""

# Create a test directory with git for Claude tests
mkdir -p /tmp/iter-test
cd /tmp/iter-test
git init -q
git config user.email "test@test.com"
git config user.name "Test"

# Get the expected version from the binary
EXPECTED_VERSION=$("$ITER_BIN" version 2>&1 | grep -oE '[0-9]+\.[0-9]+\.[0-9]+-[0-9]+' | head -1)
if [ -z "$EXPECTED_VERSION" ]; then
    EXPECTED_VERSION="dev"
fi
echo "Expected iter version: $EXPECTED_VERSION"
echo ""

# [9/12] Test claude -p '/iter:iter -v' (command line invocation)
echo "[9/12] Testing: claude -p '/iter:iter -v' (command line)..."
echo ""

CLAUDE_CMD_OUTPUT=$(timeout 120 claude -p '/iter:iter -v' --dangerously-skip-permissions 2>&1) || CMD_EXIT=$?
CMD_EXIT=${CMD_EXIT:-0}

echo "--- claude -p '/iter:iter -v' output ---"
echo "$CLAUDE_CMD_OUTPUT"
echo "--- end (exit code: $CMD_EXIT) ---"
echo ""

# Check if /iter:iter was recognized and executed with correct version
if echo "$CLAUDE_CMD_OUTPUT" | grep -qE "$EXPECTED_VERSION"; then
    echo "OK: /iter:iter -v executed via command line (version matches)"
    CMD_TEST_PASS=1
elif echo "$CLAUDE_CMD_OUTPUT" | grep -qiE "(iter version|ITERATIVE IMPLEMENTATION|VERSION MODE)"; then
    echo "OK: /iter:iter -v executed via command line"
    CMD_TEST_PASS=1
else
    echo "FAIL: /iter:iter -v did NOT execute properly via command line"
    echo "Expected output to contain version '$EXPECTED_VERSION' or 'iter version'"
    CMD_TEST_PASS=0
fi
echo ""

# [10/12] Test interactive mode
echo "[10/12] Testing: /iter:iter -v in interactive Claude session..."
echo "(This also triggers SessionStart hook to install /iter wrapper)"
echo ""

INTERACTIVE_OUTPUT=$(timeout 120 bash -c '
echo "/iter:iter -v" | claude --dangerously-skip-permissions 2>&1
' 2>&1) || INT_EXIT=$?
INT_EXIT=${INT_EXIT:-0}

echo "--- Interactive /iter:iter -v output ---"
echo "$INTERACTIVE_OUTPUT"
echo "--- end (exit code: $INT_EXIT) ---"
echo ""

# Check if /iter:iter was recognized and executed
if echo "$INTERACTIVE_OUTPUT" | grep -qE "$EXPECTED_VERSION"; then
    echo "OK: /iter:iter -v executed in interactive mode (version matches)"
    INT_TEST_PASS=1
elif echo "$INTERACTIVE_OUTPUT" | grep -qiE "(iter version|ITERATIVE IMPLEMENTATION|VERSION MODE)"; then
    echo "OK: /iter:iter -v executed in interactive mode"
    INT_TEST_PASS=1
else
    echo "FAIL: /iter:iter -v did NOT execute properly in interactive mode"
    echo "Expected output to contain version '$EXPECTED_VERSION' or 'iter version'"
    INT_TEST_PASS=0
fi
echo ""

# [11/12] Check /iter wrapper installation
echo "[11/12] Checking /iter wrapper installation..."
ITER_WRAPPER="$HOME/.claude/skills/iter/SKILL.md"
if [ -f "$ITER_WRAPPER" ]; then
    echo "--- Wrapper skill installed at $ITER_WRAPPER ---"
    cat "$ITER_WRAPPER"
    echo "--- end ---"
    echo "OK: /iter wrapper skill installed by SessionStart hook"
    WRAPPER_INSTALLED=1
else
    echo "FAIL: /iter wrapper skill NOT installed"
    echo "Expected file at: $ITER_WRAPPER"
    WRAPPER_INSTALLED=0
fi
echo ""

# [12/12] Test /iter -v shortcut
echo "[12/12] Testing: /iter -v (shortcut command)..."
echo ""

ITER_SHORTCUT_OUTPUT=$(timeout 120 bash -c '
echo "/iter -v" | claude --dangerously-skip-permissions 2>&1
' 2>&1) || SHORTCUT_EXIT=$?
SHORTCUT_EXIT=${SHORTCUT_EXIT:-0}

echo "--- /iter -v output ---"
echo "$ITER_SHORTCUT_OUTPUT"
echo "--- end (exit code: $SHORTCUT_EXIT) ---"
echo ""

# Check if /iter -v executed and shows the correct version
if echo "$ITER_SHORTCUT_OUTPUT" | grep -qE "$EXPECTED_VERSION"; then
    echo "OK: /iter -v executed and version matches ($EXPECTED_VERSION)"
    SHORTCUT_TEST_PASS=1
elif echo "$ITER_SHORTCUT_OUTPUT" | grep -qiE "(iter version|VERSION MODE)"; then
    echo "OK: /iter -v executed (version output detected)"
    SHORTCUT_TEST_PASS=1
else
    echo "FAIL: /iter -v did NOT execute properly"
    echo "Expected output to contain version '$EXPECTED_VERSION'"
    SHORTCUT_TEST_PASS=0
fi
echo ""

# Final result
echo "=========================================="
echo "TEST RESULTS"
echo "=========================================="
echo "Command line test (claude -p '/iter:iter -v'): $([ $CMD_TEST_PASS -eq 1 ] && echo 'PASS' || echo 'FAIL')"
echo "Interactive test (/iter:iter -v in session):  $([ $INT_TEST_PASS -eq 1 ] && echo 'PASS' || echo 'FAIL')"
echo "Wrapper installation (SessionStart hook):   $([ $WRAPPER_INSTALLED -eq 1 ] && echo 'PASS' || echo 'FAIL')"
echo "Shortcut test (/iter -v):                   $([ $SHORTCUT_TEST_PASS -eq 1 ] && echo 'PASS' || echo 'FAIL')"
echo ""

if [ $CMD_TEST_PASS -eq 1 ] && [ $INT_TEST_PASS -eq 1 ] && [ $WRAPPER_INSTALLED -eq 1 ] && [ $SHORTCUT_TEST_PASS -eq 1 ]; then
    echo "=========================================="
    echo "ALL TESTS PASSED"
    echo "=========================================="
    exit 0
else
    echo "=========================================="
    echo "TESTS FAILED"
    echo "=========================================="
    echo ""
    echo "Check that:"
    echo "  1. The plugin is properly installed"
    echo "  2. Skills are correctly defined in marketplace.json"
    echo "  3. SKILL.md files have the 'name' field"
    echo "  4. SessionStart hook creates /iter wrapper skill"
    echo ""
    exit 1
fi
`

	output, err := setup.RunScript(testScript)

	// Determine test result
	status := "PASS"
	var missing []string

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

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			missing = append(missing, expected)
			status = "FAIL"
		}
	}

	// Check Claude integration tests
	if !strings.Contains(output, "OK: /iter:iter -v executed via command line") {
		missing = append(missing, "OK: /iter:iter -v executed via command line")
		status = "FAIL"
	}
	if !strings.Contains(output, "OK: /iter:iter -v executed in interactive mode") {
		missing = append(missing, "OK: /iter:iter -v executed in interactive mode")
		status = "FAIL"
	}

	// Check SessionStart hook installed the /iter wrapper
	if !strings.Contains(output, "OK: /iter wrapper skill installed") {
		missing = append(missing, "OK: /iter wrapper skill installed")
		status = "FAIL"
	}

	// Check /iter -v shortcut works
	if !strings.Contains(output, "OK: /iter -v executed") {
		missing = append(missing, "OK: /iter -v executed")
		status = "FAIL"
	}

	// Final check
	if !strings.Contains(output, "ALL TESTS PASSED") {
		status = "FAIL"
	}

	// Write result summary
	setup.ResultDir.WriteResult(status, missing)

	// Report failures
	if err != nil {
		t.Fatalf("Docker integration test failed: %v", err)
	}

	for _, m := range missing {
		t.Errorf("Docker test output missing expected string: %q", m)
	}

	if status == "FAIL" {
		t.Errorf("Test failed - /iter:iter command not executing properly in Claude")
	}
}
