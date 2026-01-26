// Package docker provides Docker-based integration tests for the iter plugin.
//
// Test Execution Order:
// These tests use t.Run() subtests to guarantee sequential execution.
// The TestDockerIntegration parent test builds the Docker image once,
// then runs all subtests in order:
//
//  1. PluginInstallation - Full installation and /iter:run test
//  2. IterRunCommandLine - Tests `claude -p '/iter:run -v'`
//  3. IterRunInteractive - Tests `/iter:run -v` in interactive session
//  4. PluginSkillAutoprompt - Tests skill discoverability
//  5. IterDirectoryCreation - Tests .iter directory creation
//  6. IterDirectoryRecreation - Tests .iter directory recreation
//
// Run tests:
//
//	go test ./tests/docker/... -v
//
// With API key for full Claude integration:
//
//	ANTHROPIC_API_KEY=sk-... go test ./tests/docker/... -v -timeout 15m
//
// Note: Tests run sequentially (not in parallel) to avoid Docker resource contention.
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

// TestDockerIntegration is the parent test that runs all Docker integration tests
// in a guaranteed sequential order. It builds the Docker image once and shares it
// across all subtests.
func TestDockerIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker integration test in short mode")
	}

	// Check if Docker is available
	dockerCheck := exec.Command("docker", "info")
	if err := dockerCheck.Run(); err != nil {
		t.Skip("Docker not available, skipping integration test")
	}

	// Clean up Docker before all tests
	pruneDocker(t)

	projectRoot := findProjectRoot(t)
	apiKey := loadAPIKey(t, projectRoot)

	if apiKey == "" {
		t.Log("WARNING: No API key found - tests requiring Claude will fail")
		t.Log("Set ANTHROPIC_API_KEY in environment or tests/docker/.env")
	}

	// Build Docker image once for all subtests
	buildDockerImage(t, projectRoot)

	// Create results directory for this test run
	timestamp := time.Now().Format("20060102-150405")
	resultsDir := filepath.Join(projectRoot, "tests", "results", timestamp+"-docker")
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		t.Fatalf("Failed to create results directory: %v", err)
	}
	t.Logf("Results will be saved to: %s", resultsDir)

	// Run subtests in order - Go guarantees sequential execution within t.Run
	t.Run("1_PluginInstallation", func(t *testing.T) {
		runPluginInstallationTest(t, projectRoot, apiKey, resultsDir)
	})

	t.Run("2_IterRunCommandLine", func(t *testing.T) {
		if apiKey == "" {
			t.Skip("ANTHROPIC_API_KEY required")
		}
		runIterRunCommandLineTest(t, projectRoot, apiKey)
	})

	t.Run("3_IterRunInteractive", func(t *testing.T) {
		if apiKey == "" {
			t.Skip("ANTHROPIC_API_KEY required")
		}
		runIterRunInteractiveTest(t, projectRoot, apiKey)
	})

	t.Run("4_PluginSkillAutoprompt", func(t *testing.T) {
		if apiKey == "" {
			t.Skip("ANTHROPIC_API_KEY required")
		}
		runPluginSkillAutopromptTest(t, projectRoot, apiKey)
	})

	t.Run("5_IterDirectoryCreation", func(t *testing.T) {
		if apiKey == "" {
			t.Skip("ANTHROPIC_API_KEY required")
		}
		runIterDirectoryCreationTest(t, projectRoot, apiKey)
	})

	t.Run("6_IterDirectoryRecreation", func(t *testing.T) {
		if apiKey == "" {
			t.Skip("ANTHROPIC_API_KEY required")
		}
		runIterDirectoryRecreationTest(t, projectRoot, apiKey)
	})
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
