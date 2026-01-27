package docker

import (
	"strings"
	"testing"
)

// TestIterDirectoryCreation tests that executing /iter:iter creates the
// required .iter directory structure (index, worktrees, workdir).
func TestIterDirectoryCreation(t *testing.T) {
	// Setup test (handles Docker, auth, image build, result dir)
	setup := setupDockerTest(t, "iter-directory-creation")
	defer setup.Close()

	testScript := `
		set -e

		# Install plugin
		claude plugin marketplace add /home/testuser/iter-plugin
		claude plugin install iter@iter-local

		# Create test directory with git
		TEST_DIR=/tmp/iter-dir-test-$(date +%s)
		mkdir -p "$TEST_DIR"
		cd "$TEST_DIR"
		git init -q
		git config user.email "test@test.com"
		git config user.name "Test"

		echo ""
		echo "=== Testing .iter directory creation ==="
		echo "Working directory: $TEST_DIR"
		echo ""

		# Execute /iter:iter command (use -r for reindex/quick execution)
		echo "Executing: /iter:iter -r"
		timeout 60 claude -p '/iter:iter -r' --dangerously-skip-permissions 2>&1 || true

		echo ""
		echo "=== Checking .iter directory structure ==="

		# Check .iter directory exists
		if [ -d ".iter" ]; then
			echo "OK: .iter directory exists"
		else
			echo "FAIL: .iter directory NOT found"
			exit 1
		fi

		# List .iter contents
		echo ""
		echo ".iter directory contents:"
		ls -la .iter/
		echo ""

		# Check for required subdirectories (reindex mode creates index only)
		MISSING=""

		if [ -d ".iter/index" ]; then
			echo "OK: .iter/index directory exists"
		else
			echo "FAIL: .iter/index directory NOT found"
			MISSING="$MISSING index"
		fi

		# Note: worktrees and workdir are only created in run/test/workflow modes
		# Reindex mode only creates the index directory
		if [ -d ".iter/worktrees" ]; then
			echo "OK: .iter/worktrees directory exists (optional)"
		else
			echo "NOTE: .iter/worktrees not created (expected for reindex mode)"
		fi

		if [ -d ".iter/workdir" ]; then
			echo "OK: .iter/workdir directory exists (optional)"
		else
			echo "NOTE: .iter/workdir not created (expected for reindex mode)"
		fi

		if [ -n "$MISSING" ]; then
			echo ""
			echo "FAIL: Missing directories:$MISSING"
			exit 1
		fi

		echo ""
		echo "=== .iter directory creation test PASSED ==="
	`

	output, err := setup.RunScript(testScript)

	// Determine test result
	status := "PASS"
	var missing []string

	// Check for success markers
	if !strings.Contains(output, ".iter directory creation test PASSED") {
		status = "FAIL"
		missing = append(missing, ".iter directory creation test PASSED")
	}

	// Check no failures
	if strings.Contains(output, "FAIL:") {
		status = "FAIL"
		missing = append(missing, "no FAIL: markers in output")
	}

	// Write result summary
	setup.ResultDir.WriteResult(status, missing)

	// Report failures
	if !strings.Contains(output, ".iter directory creation test PASSED") {
		t.Errorf(".iter directory creation test did not pass")
	}

	if strings.Contains(output, "FAIL:") {
		t.Errorf("Test reported failures in output")
	}

	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}
}

// TestIterDirectoryRecreation tests that deleting .iter directory and
// re-running /iter:iter properly recreates the directory structure.
func TestIterDirectoryRecreation(t *testing.T) {
	// Setup test (handles Docker, auth, image build, result dir)
	setup := setupDockerTest(t, "iter-directory-recreation")
	defer setup.Close()

	testScript := `
		set -e

		# Install plugin
		claude plugin marketplace add /home/testuser/iter-plugin
		claude plugin install iter@iter-local

		# Create test directory with git
		TEST_DIR=/tmp/iter-dir-recreate-$(date +%s)
		mkdir -p "$TEST_DIR"
		cd "$TEST_DIR"
		git init -q
		git config user.email "test@test.com"
		git config user.name "Test"

		echo ""
		echo "=== Testing .iter directory recreation ==="
		echo "Working directory: $TEST_DIR"
		echo ""

		# First execution - create .iter directory
		echo "=== FIRST EXECUTION ==="
		echo "Executing: /iter:iter -r"
		timeout 60 claude -p '/iter:iter -r' --dangerously-skip-permissions 2>&1 || true

		echo ""
		echo "Checking .iter directory exists after first execution..."
		if [ -d ".iter" ]; then
			echo "OK: .iter directory exists after first execution"
		else
			echo "FAIL: .iter directory NOT created on first execution"
			exit 1
		fi

		# Delete .iter directory
		echo ""
		echo "=== DELETING .iter DIRECTORY ==="
		rm -rf .iter
		echo "Deleted .iter directory"

		# Verify deletion
		if [ -d ".iter" ]; then
			echo "FAIL: .iter directory still exists after deletion"
			exit 1
		else
			echo "OK: .iter directory deleted successfully"
		fi

		# Second execution - recreate .iter directory
		echo ""
		echo "=== SECOND EXECUTION ==="
		echo "Executing: /iter:iter -r (again)"
		timeout 60 claude -p '/iter:iter -r' --dangerously-skip-permissions 2>&1 || true

		echo ""
		echo "=== Checking .iter directory recreation ==="

		# Check .iter directory recreated
		if [ -d ".iter" ]; then
			echo "OK: .iter directory recreated"
		else
			echo "FAIL: .iter directory NOT recreated"
			exit 1
		fi

		# List .iter contents
		echo ""
		echo ".iter directory contents after recreation:"
		ls -la .iter/
		echo ""

		# Check for required subdirectories (reindex mode creates index only)
		MISSING=""

		if [ -d ".iter/index" ]; then
			echo "OK: .iter/index directory recreated"
		else
			echo "FAIL: .iter/index directory NOT recreated"
			MISSING="$MISSING index"
		fi

		# Note: worktrees and workdir are only created in run/test/workflow modes
		# Reindex mode only creates the index directory
		if [ -d ".iter/worktrees" ]; then
			echo "OK: .iter/worktrees directory recreated (optional)"
		else
			echo "NOTE: .iter/worktrees not created (expected for reindex mode)"
		fi

		if [ -d ".iter/workdir" ]; then
			echo "OK: .iter/workdir directory recreated (optional)"
		else
			echo "NOTE: .iter/workdir not created (expected for reindex mode)"
		fi

		if [ -n "$MISSING" ]; then
			echo ""
			echo "FAIL: Missing directories after recreation:$MISSING"
			exit 1
		fi

		echo ""
		echo "=== .iter directory recreation test PASSED ==="
	`

	output, err := setup.RunScript(testScript)

	// Determine test result
	status := "PASS"
	var missing []string

	// Check for success markers
	if !strings.Contains(output, ".iter directory recreation test PASSED") {
		status = "FAIL"
		missing = append(missing, ".iter directory recreation test PASSED")
	}

	// Check intermediate steps
	if !strings.Contains(output, "OK: .iter directory exists after first execution") {
		status = "FAIL"
		missing = append(missing, "OK: .iter directory exists after first execution")
	}

	if !strings.Contains(output, "OK: .iter directory deleted successfully") {
		status = "FAIL"
		missing = append(missing, "OK: .iter directory deleted successfully")
	}

	if !strings.Contains(output, "OK: .iter directory recreated") {
		status = "FAIL"
		missing = append(missing, "OK: .iter directory recreated")
	}

	// Check no failures
	if strings.Contains(output, "FAIL:") {
		status = "FAIL"
		missing = append(missing, "no FAIL: markers in output")
	}

	// Write result summary
	setup.ResultDir.WriteResult(status, missing)

	// Report failures
	if !strings.Contains(output, ".iter directory recreation test PASSED") {
		t.Errorf(".iter directory recreation test did not pass")
	}

	if !strings.Contains(output, "OK: .iter directory exists after first execution") {
		t.Errorf("First execution did not create .iter directory")
	}

	if !strings.Contains(output, "OK: .iter directory deleted successfully") {
		t.Errorf("Directory deletion failed")
	}

	if !strings.Contains(output, "OK: .iter directory recreated") {
		t.Errorf("Second execution did not recreate .iter directory")
	}

	if strings.Contains(output, "FAIL:") {
		t.Errorf("Test reported failures in output")
	}

	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}
}
