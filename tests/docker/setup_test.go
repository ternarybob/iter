// Package docker provides Docker-based integration tests for the iter plugin.
//
// Tests:
//   - TestPluginInstallation - Full installation and /iter:iter test
//   - TestIterRunCommandLine - Tests `claude -p '/iter:iter -v'`
//   - TestIterRunInteractive - Tests `/iter:iter -v` in interactive session
//   - TestPluginSkillAutoprompt - Tests skill discoverability
//   - TestIterDirectoryCreation - Tests .iter directory creation
//   - TestIterDirectoryRecreation - Tests .iter directory recreation
//   - TestIterInstallSkill - Tests /iter:install creates /iter wrapper
//
// Each test is independent and sets up its own Docker environment.
//
// # Authentication Configuration
//
// Tests support two authentication methods (in priority order):
//
// 1. API Key: Set ANTHROPIC_API_KEY environment variable or in tests/docker/.env
// 2. Claude Max: Uses ~/.claude/.credentials.json OAuth token automatically
//
// # Running Tests
//
// Run all tests:
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
// Use Claude Max credentials (auto-detected from ~/.claude/.credentials.json):
//
//	go test ./tests/docker/... -v -timeout 15m
//
// Note: Tests can run in parallel or individually. Each test creates its own
// timestamped results directory in tests/results/.
package docker

import (
	"encoding/json"
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

// Package-level auth configuration (initialized by setupAuth)
var (
	globalAuthMode  AuthMode
	globalAuthValue string
	globalAuthInit  bool
)

// setupAuth initializes the global auth configuration once
// This is called automatically by requireAuth
func setupAuth(t *testing.T, projectRoot string) {
	if globalAuthInit {
		return
	}
	globalAuthMode, globalAuthValue = getAuthMode(t, projectRoot)
	globalAuthInit = true
}

// requireAuth ensures auth is available and returns the docker args
// This is the main function tests should use for auth setup
func requireAuth(t *testing.T) []string {
	t.Helper()

	projectRoot := findProjectRoot(t)
	setupAuth(t, projectRoot)

	if globalAuthMode == AuthModeNone {
		t.Skip("No authentication available (set ANTHROPIC_API_KEY, tests/docker/.env, or login with 'claude login' for Claude Max)")
	}

	return buildDockerRunArgs(t, globalAuthMode, globalAuthValue)
}

// requireDockerAndAuth is a convenience function that checks Docker availability
// and returns the base docker run args with auth configured
func requireDockerAndAuth(t *testing.T) []string {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping Docker integration test in short mode")
	}

	// Check Docker availability
	dockerCheck := exec.Command("docker", "info")
	if err := dockerCheck.Run(); err != nil {
		t.Skip("Docker not available, skipping integration test")
	}

	return requireAuth(t)
}

// TestSetup contains all the common setup for Docker tests
type TestSetup struct {
	T           *testing.T
	ProjectRoot string
	BaseArgs    []string // Docker run base args with auth
	ResultDir   *TestResultDir
}

// setupDockerTest performs all common test setup and returns a TestSetup
// This is the main entry point for Docker tests
func setupDockerTest(t *testing.T, testName string) *TestSetup {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping Docker integration test in short mode")
	}

	// Check Docker availability
	dockerCheck := exec.Command("docker", "info")
	if err := dockerCheck.Run(); err != nil {
		t.Skip("Docker not available, skipping integration test")
	}

	projectRoot := findProjectRoot(t)

	// Setup auth
	setupAuth(t, projectRoot)
	if globalAuthMode == AuthModeNone {
		t.Skip("No authentication available (set ANTHROPIC_API_KEY, tests/docker/.env, or login with 'claude login' for Claude Max)")
	}

	// Create result directory
	resultDir := createTestResultDir(t, projectRoot, testName)

	// Build Docker image (reuses if exists)
	buildDockerImage(t, projectRoot)

	return &TestSetup{
		T:           t,
		ProjectRoot: projectRoot,
		BaseArgs:    buildDockerRunArgs(t, globalAuthMode, globalAuthValue),
		ResultDir:   resultDir,
	}
}

// Close cleans up test resources
func (s *TestSetup) Close() {
	if s.ResultDir != nil {
		s.ResultDir.Close()
	}
}

// RunScript executes a shell script in Docker and returns the output
func (s *TestSetup) RunScript(script string) (string, error) {
	s.T.Helper()

	args := append(s.BaseArgs, "--entrypoint", "bash", dockerImage, "-c", script)
	runCmd := exec.Command("docker", args...)
	runCmd.Dir = s.ProjectRoot

	output, err := runCmd.CombinedOutput()

	// Log output
	s.ResultDir.WriteLog(output)
	s.T.Logf("Output:\n%s", output)

	return string(output), err
}

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

// getClaudeCredentialsPath returns the path to Claude credentials if they exist
// This supports Claude Max subscription authentication
func getClaudeCredentialsPath(t *testing.T) string {
	t.Helper()

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	credPath := filepath.Join(homeDir, ".claude", ".credentials.json")
	if _, err := os.Stat(credPath); err == nil {
		return filepath.Join(homeDir, ".claude")
	}

	return ""
}

// getOAuthToken extracts the OAuth token from Claude credentials file
// Returns empty string if not found
func getOAuthToken(t *testing.T, claudeDir string) string {
	t.Helper()

	credFile := filepath.Join(claudeDir, ".credentials.json")
	data, err := os.ReadFile(credFile)
	if err != nil {
		return ""
	}

	// Parse JSON to extract claudeAiOauth.accessToken
	var creds struct {
		ClaudeAiOauth struct {
			AccessToken string `json:"accessToken"`
		} `json:"claudeAiOauth"`
	}
	if err := json.Unmarshal(data, &creds); err != nil {
		return ""
	}

	return creds.ClaudeAiOauth.AccessToken
}

// AuthMode indicates how Docker tests should authenticate with Claude
type AuthMode int

const (
	AuthModeNone AuthMode = iota
	AuthModeAPIKey
	AuthModeCredentials // Claude Max subscription
)

// getAuthMode determines which authentication method is available
// Supports both API key and Claude Max subscription authentication
// Set USE_CLAUDE_MAX=1 to prefer Claude Max subscription over API key
func getAuthMode(t *testing.T, projectRoot string) (AuthMode, string) {
	t.Helper()

	// Check for Claude credentials first if USE_CLAUDE_MAX is set
	if os.Getenv("USE_CLAUDE_MAX") != "" {
		credPath := getClaudeCredentialsPath(t)
		if credPath != "" {
			t.Log("Using Claude Max credentials (USE_CLAUDE_MAX=1)")
			return AuthModeCredentials, credPath
		}
	}

	// Check for API key
	apiKey := loadAPIKey(t, projectRoot)
	if apiKey != "" {
		return AuthModeAPIKey, apiKey
	}

	// Fall back to Claude credentials if available
	credPath := getClaudeCredentialsPath(t)
	if credPath != "" {
		t.Log("Using Claude Max credentials")
		return AuthModeCredentials, credPath
	}

	return AuthModeNone, ""
}

// buildDockerRunArgs builds the docker run arguments based on auth mode
func buildDockerRunArgs(t *testing.T, authMode AuthMode, authValue string) []string {
	t.Helper()

	args := []string{"run", "--rm"}

	switch authMode {
	case AuthModeAPIKey:
		t.Log("Using API key authentication")
		args = append(args, "-e", "ANTHROPIC_API_KEY="+authValue)
	case AuthModeCredentials:
		t.Log("Using Claude Max credentials (CLAUDE_CODE_OAUTH_TOKEN)")
		// Use CLAUDE_CODE_OAUTH_TOKEN environment variable for headless auth
		// This is the recommended method for Docker/CI environments
		token := getOAuthToken(t, authValue)
		if token != "" {
			args = append(args, "-e", "CLAUDE_CODE_OAUTH_TOKEN="+token)
		} else {
			t.Log("Warning: Could not extract OAuth token from credentials")
		}
	}

	return args
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

// runDockerTest runs a test script in Docker with authentication automatically configured.
// It handles common setup: Docker availability check, auth loading, image building.
// Supports both API key and Claude Max subscription authentication.
// Returns the combined output and any error.
func runDockerTest(t *testing.T, script string) (string, error) {
	t.Helper()

	// Check Docker availability
	dockerCheck := exec.Command("docker", "info")
	if err := dockerCheck.Run(); err != nil {
		t.Skip("Docker not available, skipping integration test")
	}

	// Get project root
	projectRoot := findProjectRoot(t)

	// Determine authentication mode
	authMode, authValue := getAuthMode(t, projectRoot)
	if authMode == AuthModeNone {
		t.Skip("No authentication available (set ANTHROPIC_API_KEY, tests/docker/.env, or login with 'claude login' for Claude Max)")
	}

	// Build Docker image (reuses if exists)
	buildDockerImage(t, projectRoot)

	// Build docker run arguments based on auth mode
	args := buildDockerRunArgs(t, authMode, authValue)
	args = append(args, "--entrypoint", "bash", dockerImage, "-c", script)

	runCmd := exec.Command("docker", args...)
	runCmd.Dir = projectRoot

	output, err := runCmd.CombinedOutput()
	return string(output), err
}

// DockerTestRunner provides a structured way to run Docker tests with result logging
type DockerTestRunner struct {
	T           *testing.T
	ProjectRoot string
	AuthMode    AuthMode
	AuthValue   string // API key or credentials path
	ResultDir   *TestResultDir
}

// NewDockerTestRunner creates a new test runner with all setup done
// Supports both API key and Claude Max subscription authentication
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

	// Get project root
	projectRoot := findProjectRoot(t)

	// Determine authentication mode
	authMode, authValue := getAuthMode(t, projectRoot)
	if authMode == AuthModeNone {
		t.Skip("No authentication available (set ANTHROPIC_API_KEY, tests/docker/.env, or login with 'claude login' for Claude Max)")
	}

	// Create result directory
	resultDir := createTestResultDir(t, projectRoot, testName)

	// Build Docker image (reuses if exists)
	buildDockerImage(t, projectRoot)

	return &DockerTestRunner{
		T:           t,
		ProjectRoot: projectRoot,
		AuthMode:    authMode,
		AuthValue:   authValue,
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

	// Build docker run arguments based on auth mode
	args := buildDockerRunArgs(r.T, r.AuthMode, r.AuthValue)
	args = append(args, "--entrypoint", "bash", dockerImage, "-c", script)

	runCmd := exec.Command("docker", args...)
	runCmd.Dir = r.ProjectRoot

	output, err := runCmd.CombinedOutput()

	// Log output
	r.ResultDir.WriteLog(output)
	r.T.Logf("Output:\n%s", output)

	return string(output), err
}
