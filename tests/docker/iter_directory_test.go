package docker

import (
	"os/exec"
	"strings"
	"testing"
)

// TestIterDirectoryCreation tests that executing /iter:run creates the
// required .iter directory structure (index, worktrees, workdir).
func TestIterDirectoryCreation(t *testing.T) {
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

		# Execute /iter:run command (use -v for quick execution)
		echo "Executing: /iter:run -v"
		timeout 60 claude -p '/iter:run -v' 2>&1 || true

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

		# Check for required subdirectories
		MISSING=""

		if [ -d ".iter/index" ]; then
			echo "OK: .iter/index directory exists"
		else
			echo "FAIL: .iter/index directory NOT found"
			MISSING="$MISSING index"
		fi

		if [ -d ".iter/worktrees" ]; then
			echo "OK: .iter/worktrees directory exists"
		else
			echo "FAIL: .iter/worktrees directory NOT found"
			MISSING="$MISSING worktrees"
		fi

		if [ -d ".iter/workdir" ]; then
			echo "OK: .iter/workdir directory exists"
		else
			echo "FAIL: .iter/workdir directory NOT found"
			MISSING="$MISSING workdir"
		fi

		if [ -n "$MISSING" ]; then
			echo ""
			echo "FAIL: Missing directories:$MISSING"
			exit 1
		fi

		echo ""
		echo "=== .iter directory creation test PASSED ==="
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

	// Check for success markers
	if !strings.Contains(outputStr, ".iter directory creation test PASSED") {
		t.Errorf(".iter directory creation test did not pass")
	}

	// Check no failures
	if strings.Contains(outputStr, "FAIL:") {
		t.Errorf("Test reported failures in output")
	}

	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}
}

// TestIterDirectoryRecreation tests that deleting .iter directory and
// re-running /iter:run properly recreates the directory structure.
func TestIterDirectoryRecreation(t *testing.T) {
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
		echo "Executing: /iter:run -v"
		timeout 60 claude -p '/iter:run -v' 2>&1 || true

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
		echo "Executing: /iter:run -v (again)"
		timeout 60 claude -p '/iter:run -v' 2>&1 || true

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

		# Check for required subdirectories
		MISSING=""

		if [ -d ".iter/index" ]; then
			echo "OK: .iter/index directory recreated"
		else
			echo "FAIL: .iter/index directory NOT recreated"
			MISSING="$MISSING index"
		fi

		if [ -d ".iter/worktrees" ]; then
			echo "OK: .iter/worktrees directory recreated"
		else
			echo "FAIL: .iter/worktrees directory NOT recreated"
			MISSING="$MISSING worktrees"
		fi

		if [ -d ".iter/workdir" ]; then
			echo "OK: .iter/workdir directory recreated"
		else
			echo "FAIL: .iter/workdir directory NOT recreated"
			MISSING="$MISSING workdir"
		fi

		if [ -n "$MISSING" ]; then
			echo ""
			echo "FAIL: Missing directories after recreation:$MISSING"
			exit 1
		fi

		echo ""
		echo "=== .iter directory recreation test PASSED ==="
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

	// Check for success markers
	if !strings.Contains(outputStr, ".iter directory recreation test PASSED") {
		t.Errorf(".iter directory recreation test did not pass")
	}

	// Check intermediate steps
	if !strings.Contains(outputStr, "OK: .iter directory exists after first execution") {
		t.Errorf("First execution did not create .iter directory")
	}

	if !strings.Contains(outputStr, "OK: .iter directory deleted successfully") {
		t.Errorf("Directory deletion failed")
	}

	if !strings.Contains(outputStr, "OK: .iter directory recreated") {
		t.Errorf("Second execution did not recreate .iter directory")
	}

	// Check no failures
	if strings.Contains(outputStr, "FAIL:") {
		t.Errorf("Test reported failures in output")
	}

	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}
}
