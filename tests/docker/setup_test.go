// Package docker provides Docker-based integration tests for the iter plugin.
//
// Tests:
//   - TestPluginInstallation - Full installation and /iter:run test
//   - TestIterRunCommandLine - Tests `claude -p '/iter:iter -v'`
//   - TestIterRunInteractive - Tests `/iter:iter -v` in interactive session
//   - TestPluginSkillAutoprompt - Tests skill discoverability
//   - TestIterDirectoryCreation - Tests .iter directory creation
//   - TestIterDirectoryRecreation - Tests .iter directory recreation
//   - TestIterInstallSkill - Tests /iter:install creates /iter wrapper
//
// Each test is independent and sets up its own Docker environment.
//
// # API Key Configuration
//
// Tests automatically load ANTHROPIC_API_KEY from:
//  1. Environment variable ANTHROPIC_API_KEY
//  2. tests/docker/.env file (format: ANTHROPIC_API_KEY=sk-...)
//
// # Running Tests
//
// Run all tests (API key loaded from .env):
//
//	go test ./tests/docker/... -v -timeout 15m
//
// Run specific test:
//
//	go test ./tests/docker/... -run TestIterInstallSkill -v -timeout 15m
//
// Override API key via environment:
//
//	ANTHROPIC_API_KEY=sk-... go test ./tests/docker/... -v -timeout 15m
//
// Note: Tests can run in parallel or individually. Each test creates its own
// timestamped results directory in tests/results/.
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

// TestResultDir manages test result directory and logging
type TestResultDir struct {
	Dir     string
	LogFile *os.File
	T       *testing.T
}

// createTestResultDir creates a timestamped result directory for a test
func createTestResultDir(t *testing.T, projectRoot, testName string) *TestResultDir {
	t.Helper()
	timestamp := time.Now().Format("20060102-150405")
	resultsDir := filepath.Join(projectRoot, "tests", "results", timestamp+"-"+testName)
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		t.Fatalf("Failed to create results directory: %v", err)
	}

	logPath := filepath.Join(resultsDir, "test-output.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		t.Fatalf("Failed to create log file: %v", err)
	}

	return &TestResultDir{Dir: resultsDir, LogFile: logFile, T: t}
}

// Close closes the log file
func (r *TestResultDir) Close() {
	if r.LogFile != nil {
		r.LogFile.Close()
	}
}

// WriteLog writes content to the log file
func (r *TestResultDir) WriteLog(content []byte) {
	r.LogFile.Write(content)
}

// WriteResult writes the result summary file
func (r *TestResultDir) WriteResult(status string, missing []string) {
	resultPath := filepath.Join(r.Dir, "result.txt")
	content := fmt.Sprintf("Status: %s\nTimestamp: %s\nResultsDir: %s\n",
		status, time.Now().Format(time.RFC3339), r.Dir)
	if len(missing) > 0 {
		content += "Missing:\n"
		for _, m := range missing {
			content += fmt.Sprintf("  - %s\n", m)
		}
	}
	os.WriteFile(resultPath, []byte(content), 0644)
}

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
		"-f", "tests/docker/Dockerfile", ".")
	buildCmd.Dir = projectRoot
	buildOutput, err := buildCmd.CombinedOutput()

	if err != nil {
		t.Fatalf("Failed to build Docker image: %v\nOutput: %s", err, buildOutput)
	}
	t.Log("Docker image built successfully")
}

// findProjectRoot walks up directory tree looking for go.mod
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

// loadAPIKey loads ANTHROPIC_API_KEY from environment or tests/docker/.env
func loadAPIKey(t *testing.T, projectRoot string) string {
	t.Helper()

	// First check environment
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		return key
	}

	// Try .env file
	envFile := filepath.Join(projectRoot, "tests", "docker", ".env")
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

// verifyIterDirectories checks that required .iter subdirectories exist in container output.
// Returns a slice of missing directory names, or empty slice if all present.
func verifyIterDirectories(t *testing.T, output string) []string {
	t.Helper()

	required := []string{
		".iter",
		".iter/index",
		".iter/worktrees",
		".iter/workdir",
	}

	var missing []string
	for _, dir := range required {
		if !strings.Contains(output, dir) {
			missing = append(missing, dir)
		}
	}

	return missing
}

// DockerTestConfig holds configuration for running Docker tests
type DockerTestConfig struct {
	ProjectRoot string
	APIKey      string
	Script      string
	Timeout     time.Duration // optional, defaults to 5 minutes
}

// runDockerTest runs a test script in Docker with the API key automatically configured.
// It handles common setup: Docker availability check, API key loading, image building.
// Returns the combined output and any error.
func runDockerTest(t *testing.T, script string) (string, error) {
	t.Helper()

	// Check Docker availability
	dockerCheck := exec.Command("docker", "info")
	if err := dockerCheck.Run(); err != nil {
		t.Skip("Docker not available, skipping integration test")
	}

	// Get project root and API key
	projectRoot := findProjectRoot(t)
	apiKey := loadAPIKey(t, projectRoot)

	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY required (set env var or tests/docker/.env)")
	}

	// Build Docker image (reuses if exists)
	buildDockerImage(t, projectRoot)

	// Run Docker container with script
	runCmd := exec.Command("docker", "run", "--rm",
		"-e", "ANTHROPIC_API_KEY="+apiKey,
		"--entrypoint", "bash",
		dockerImage,
		"-c", script)
	runCmd.Dir = projectRoot

	output, err := runCmd.CombinedOutput()
	return string(output), err
}

// DockerTestRunner provides a structured way to run Docker tests with result logging
type DockerTestRunner struct {
	T           *testing.T
	ProjectRoot string
	APIKey      string
	ResultDir   *TestResultDir
}

// NewDockerTestRunner creates a new test runner with all setup done
func NewDockerTestRunner(t *testing.T, testName string) *DockerTestRunner {
	t.Helper()

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
		t.Skip("ANTHROPIC_API_KEY required (set env var or tests/docker/.env)")
	}

	// Create result directory
	resultDir := createTestResultDir(t, projectRoot, testName)

	// Build Docker image (reuses if exists)
	buildDockerImage(t, projectRoot)

	return &DockerTestRunner{
		T:           t,
		ProjectRoot: projectRoot,
		APIKey:      apiKey,
		ResultDir:   resultDir,
	}
}

// Close cleans up resources
func (r *DockerTestRunner) Close() {
	if r.ResultDir != nil {
		r.ResultDir.Close()
	}
}

// Run executes a script in Docker and returns the output
func (r *DockerTestRunner) Run(script string) (string, error) {
	r.T.Helper()

	runCmd := exec.Command("docker", "run", "--rm",
		"-e", "ANTHROPIC_API_KEY="+r.APIKey,
		"--entrypoint", "bash",
		dockerImage,
		"-c", script)
	runCmd.Dir = r.ProjectRoot

	output, err := runCmd.CombinedOutput()

	// Log output
	r.ResultDir.WriteLog(output)
	r.T.Logf("Output:\n%s", output)

	return string(output), err
}
