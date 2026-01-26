// Package docker provides Docker-based integration tests for the iter plugin.
//
// Tests:
//   - TestPluginInstallation - Full installation and /iter:run test
//   - TestIterRunCommandLine - Tests `claude -p '/iter:run -v'`
//   - TestIterRunInteractive - Tests `/iter:run -v` in interactive session
//   - TestPluginSkillAutoprompt - Tests skill discoverability
//   - TestIterDirectoryCreation - Tests .iter directory creation
//   - TestIterDirectoryRecreation - Tests .iter directory recreation
//
// Each test is independent and sets up its own Docker environment.
//
// Run all tests:
//
//	go test ./tests/docker/... -v
//
// Run specific test:
//
//	go test ./tests/docker/... -run TestPluginInstallation -v
//
// With API key for full Claude integration:
//
//	ANTHROPIC_API_KEY=sk-... go test ./tests/docker/... -v -timeout 15m
//
// Note: Tests can run in parallel or individually. Each test creates its own
// timestamped results directory in tests/results/.
package docker

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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
