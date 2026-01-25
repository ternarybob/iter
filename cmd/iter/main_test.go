package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// TestSKILLMDFormat validates that all SKILL.md files have required fields
func TestSKILLMDFormat(t *testing.T) {
	// Find project root (go up from cmd/iter)
	projectRoot := findTestProjectRoot(t)
	skillsDir := filepath.Join(projectRoot, "skills")

	// Required frontmatter fields
	requiredFields := []string{"name", "description"}

	// Walk all skills directories
	err := filepath.Walk(skillsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.Name() != "SKILL.md" {
			return nil
		}

		t.Run(path, func(t *testing.T) {
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("failed to read %s: %v", path, err)
			}

			// Parse frontmatter
			frontmatter := parseFrontmatter(string(content))
			if frontmatter == nil {
				t.Fatalf("SKILL.md missing frontmatter: %s", path)
			}

			// Check required fields
			for _, field := range requiredFields {
				if _, ok := frontmatter[field]; !ok {
					t.Errorf("SKILL.md missing required field '%s': %s", field, path)
				}
			}

			// Validate name field format (should be kebab-case or simple identifier)
			if name, ok := frontmatter["name"]; ok {
				if !isValidSkillName(name) {
					t.Errorf("SKILL.md has invalid name format '%s': %s", name, path)
				}
			}
		})

		return nil
	})

	if err != nil {
		t.Fatalf("failed to walk skills directory: %v", err)
	}
}

// TestBinSKILLMDFormat validates SKILL.md files in the bin directory (built output)
func TestBinSKILLMDFormat(t *testing.T) {
	projectRoot := findTestProjectRoot(t)
	binSkillsDir := filepath.Join(projectRoot, "bin", "skills")

	// Skip if bin directory doesn't exist (not built yet)
	if _, err := os.Stat(binSkillsDir); os.IsNotExist(err) {
		t.Skip("bin/skills directory not found - run build first")
	}

	requiredFields := []string{"name", "description"}

	err := filepath.Walk(binSkillsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.Name() != "SKILL.md" {
			return nil
		}

		t.Run(path, func(t *testing.T) {
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("failed to read %s: %v", path, err)
			}

			frontmatter := parseFrontmatter(string(content))
			if frontmatter == nil {
				t.Fatalf("SKILL.md missing frontmatter: %s", path)
			}

			for _, field := range requiredFields {
				if _, ok := frontmatter[field]; !ok {
					t.Errorf("SKILL.md missing required field '%s': %s", field, path)
				}
			}
		})

		return nil
	})

	if err != nil {
		t.Fatalf("failed to walk bin skills directory: %v", err)
	}
}

// TestMarketplaceJSON validates the marketplace.json structure
func TestMarketplaceJSON(t *testing.T) {
	projectRoot := findTestProjectRoot(t)

	// Test both config and bin marketplace.json files
	marketplaceFiles := []string{
		filepath.Join(projectRoot, "config", "marketplace.json"),
		filepath.Join(projectRoot, "bin", ".claude-plugin", "marketplace.json"),
	}

	for _, path := range marketplaceFiles {
		t.Run(path, func(t *testing.T) {
			if _, err := os.Stat(path); os.IsNotExist(err) {
				t.Skipf("marketplace.json not found at %s", path)
			}

			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("failed to read marketplace.json: %v", err)
			}

			var marketplace map[string]interface{}
			if err := json.Unmarshal(content, &marketplace); err != nil {
				t.Fatalf("invalid JSON in marketplace.json: %v", err)
			}

			// Check required top-level fields
			if _, ok := marketplace["name"]; !ok {
				t.Error("marketplace.json missing 'name' field")
			}

			// Check plugins array
			plugins, ok := marketplace["plugins"].([]interface{})
			if !ok {
				t.Fatal("marketplace.json missing or invalid 'plugins' array")
			}

			for i, p := range plugins {
				plugin, ok := p.(map[string]interface{})
				if !ok {
					t.Errorf("plugin %d is not an object", i)
					continue
				}

				// Check required plugin fields
				requiredPluginFields := []string{"name", "description", "source"}
				for _, field := range requiredPluginFields {
					if _, ok := plugin[field]; !ok {
						t.Errorf("plugin %d missing required field '%s'", i, field)
					}
				}

				// Check skills field exists and is an array
				skills, ok := plugin["skills"]
				if !ok {
					t.Errorf("plugin %d missing 'skills' field", i)
				} else {
					skillsArr, ok := skills.([]interface{})
					if !ok {
						t.Errorf("plugin %d 'skills' field is not an array", i)
					} else if len(skillsArr) == 0 {
						t.Errorf("plugin %d has empty 'skills' array", i)
					}
				}

				// Check strict field is false (required for skills in marketplace)
				strict, ok := plugin["strict"]
				if ok && strict.(bool) {
					t.Errorf("plugin %d has strict=true, but skills require strict=false", i)
				}
			}
		})
	}
}

// TestPluginJSON validates the plugin.json structure
func TestPluginJSON(t *testing.T) {
	projectRoot := findTestProjectRoot(t)

	pluginFiles := []string{
		filepath.Join(projectRoot, "config", "plugin.json"),
		filepath.Join(projectRoot, "bin", ".claude-plugin", "plugin.json"),
	}

	for _, path := range pluginFiles {
		t.Run(path, func(t *testing.T) {
			if _, err := os.Stat(path); os.IsNotExist(err) {
				t.Skipf("plugin.json not found at %s", path)
			}

			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("failed to read plugin.json: %v", err)
			}

			var plugin map[string]interface{}
			if err := json.Unmarshal(content, &plugin); err != nil {
				t.Fatalf("invalid JSON in plugin.json: %v", err)
			}

			// Check required fields
			requiredFields := []string{"name", "version", "description"}
			for _, field := range requiredFields {
				if _, ok := plugin[field]; !ok {
					t.Errorf("plugin.json missing required field '%s'", field)
				}
			}

			// plugin.json should NOT have skills field (that goes in marketplace.json)
			if _, ok := plugin["skills"]; ok {
				t.Error("plugin.json should not have 'skills' field - skills should be defined in marketplace.json")
			}
		})
	}
}

// TestSummarizeTask tests the task summarization function
func TestSummarizeTask(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Add health check endpoint", "add-health-check-endpoint"},
		{"Fix bug in login", "fix-bug-in-login"},
		{"", "task"},
		{"A very long task description that should be truncated to fit", "a-very-long-task-description"},
		{"Task with   multiple    spaces", "task-with-multiple-spaces"},
		{"Task\nwith\nnewlines", "task-with-newlines"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := summarizeTask(tt.input)
			if result != tt.expected {
				t.Errorf("summarizeTask(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestDetectOS tests OS detection
func TestDetectOS(t *testing.T) {
	info := detectOS()

	// Should always have a valid OS
	if info.OS == "" {
		t.Error("detectOS returned empty OS")
	}

	validOS := map[string]bool{"linux": true, "darwin": true, "windows": true}
	if !validOS[info.OS] {
		t.Errorf("detectOS returned unexpected OS: %s", info.OS)
	}
}

// TestHookResponseJSON tests that hook responses are valid JSON
func TestHookResponseJSON(t *testing.T) {
	responses := []HookResponse{
		{Continue: true},
		{Continue: false, SystemMessage: "Test message"},
		{Continue: true, SuppressOutput: true},
	}

	for i, resp := range responses {
		t.Run(string(rune('A'+i)), func(t *testing.T) {
			data, err := json.Marshal(resp)
			if err != nil {
				t.Fatalf("failed to marshal HookResponse: %v", err)
			}

			var decoded HookResponse
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("failed to unmarshal HookResponse: %v", err)
			}

			if decoded.Continue != resp.Continue {
				t.Errorf("Continue mismatch: got %v, want %v", decoded.Continue, resp.Continue)
			}
		})
	}
}

// TestStateJSON tests state serialization
func TestStateJSON(t *testing.T) {
	state := &State{
		Task:          "Test task",
		Mode:          "iter",
		MaxIterations: 50,
		Iteration:     1,
		Phase:         "architect",
		CurrentStep:   1,
	}

	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("failed to marshal State: %v", err)
	}

	var decoded State
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal State: %v", err)
	}

	if decoded.Task != state.Task {
		t.Errorf("Task mismatch: got %q, want %q", decoded.Task, state.Task)
	}

	if decoded.Mode != state.Mode {
		t.Errorf("Mode mismatch: got %q, want %q", decoded.Mode, state.Mode)
	}
}

// TestSkillsExist verifies all expected skills exist
func TestSkillsExist(t *testing.T) {
	projectRoot := findTestProjectRoot(t)
	skillsDir := filepath.Join(projectRoot, "skills")

	expectedSkills := []string{
		"iter",
		"iter-workflow",
		"iter-index",
		"iter-search",
		"run",
		"install",
	}

	for _, skill := range expectedSkills {
		skillPath := filepath.Join(skillsDir, skill, "SKILL.md")
		t.Run(skill, func(t *testing.T) {
			if _, err := os.Stat(skillPath); os.IsNotExist(err) {
				t.Errorf("expected skill not found: %s", skillPath)
			}
		})
	}
}

// TestBinaryCommands tests that the binary handles commands correctly
func TestBinaryCommands(t *testing.T) {
	// Test version command output
	t.Run("version", func(t *testing.T) {
		// Can't easily test main() directly, but we can test version is set
		if version == "" {
			t.Error("version should not be empty")
		}
	})
}

// TestDockerIntegration runs the Docker integration test
// This test requires Docker to be installed and running
func TestDockerIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker integration test in short mode")
	}

	// Check if Docker is available
	dockerCheck := exec.Command("docker", "info")
	if err := dockerCheck.Run(); err != nil {
		t.Skip("Docker not available, skipping integration test")
	}

	projectRoot := findTestProjectRoot(t)
	testDockerDir := filepath.Join(projectRoot, "test", "docker")

	// Check if test/docker directory exists
	if _, err := os.Stat(testDockerDir); os.IsNotExist(err) {
		t.Skip("test/docker directory not found")
	}

	// Create results directory with timestamp
	timestamp := time.Now().Format("20060102-150405")
	resultsDir := filepath.Join(projectRoot, "test", "results", timestamp+"-docker")
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		t.Fatalf("Failed to create results directory: %v", err)
	}

	// Open log file
	logPath := filepath.Join(resultsDir, "test-output.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		t.Fatalf("Failed to create log file: %v", err)
	}
	defer logFile.Close()

	// Build Docker image
	t.Log("Building Docker test image...")
	buildCmd := exec.Command("docker", "build", "--no-cache", "-t", "iter-plugin-test", "-f", "test/docker/Dockerfile", ".")
	buildCmd.Dir = projectRoot
	buildOutput, err := buildCmd.CombinedOutput()
	logFile.WriteString("=== Docker Build Output ===\n")
	logFile.Write(buildOutput)
	logFile.WriteString("\n")

	if err != nil {
		t.Fatalf("Failed to build Docker image: %v\nOutput: %s", err, buildOutput)
	}

	// Run Docker container
	t.Log("Running Docker test container...")

	// Pass API key if available for full integration test
	runArgs := []string{"run", "--rm"}
	if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
		t.Log("API key detected, running full Claude integration test")
		runArgs = append(runArgs, "-e", "ANTHROPIC_API_KEY="+apiKey)
	} else {
		t.Log("No API key, running offline simulation test")
	}
	runArgs = append(runArgs, "iter-plugin-test")

	runCmd := exec.Command("docker", runArgs...)
	runCmd.Dir = projectRoot
	output, err := runCmd.CombinedOutput()

	// Write output to log file
	logFile.WriteString("=== Docker Run Output ===\n")
	logFile.Write(output)
	logFile.WriteString("\n")

	// Log output for debugging
	t.Logf("Docker test output:\n%s", output)
	t.Logf("Results saved to: %s", resultsDir)

	// Write result file
	resultPath := filepath.Join(resultsDir, "result.txt")
	status := "PASS"
	if err != nil {
		status = "FAIL"
	}

	// Verify expected output
	outputStr := string(output)
	expectedStrings := []string{
		"Successfully added marketplace: iter-local",
		"Successfully installed plugin: iter@iter-local",
		"OK: iter@iter-local found in settings",
		"OK: SKILL.md has 'name' field",
		"OK: marketplace.json has 'skills' field",
		"OK: iter binary executes correctly",
		"OK: iter help works",
		"OK: iter run command executes correctly",
		"ALL TESTS PASSED",
	}

	var missing []string
	for _, expected := range expectedStrings {
		if !strings.Contains(outputStr, expected) {
			missing = append(missing, expected)
			status = "FAIL"
		}
	}

	// Write result file
	resultContent := fmt.Sprintf("Status: %s\nTimestamp: %s\nResultsDir: %s\n",
		status, time.Now().Format(time.RFC3339), resultsDir)
	if len(missing) > 0 {
		resultContent += fmt.Sprintf("Missing: %v\n", missing)
	}
	os.WriteFile(resultPath, []byte(resultContent), 0644)

	if err != nil {
		t.Fatalf("Docker integration test failed: %v", err)
	}

	for _, m := range missing {
		t.Errorf("Docker test output missing expected string: %q", m)
	}
}

// Helper functions

func findTestProjectRoot(t *testing.T) string {
	t.Helper()

	// Start from current directory and go up
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	// Go up from cmd/iter to project root
	for i := 0; i < 5; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}

	t.Fatal("could not find project root (go.mod)")
	return ""
}

func parseFrontmatter(content string) map[string]string {
	// YAML frontmatter is between --- markers
	if !strings.HasPrefix(content, "---") {
		return nil
	}

	parts := strings.SplitN(content[3:], "---", 2)
	if len(parts) < 2 {
		return nil
	}

	frontmatter := make(map[string]string)
	lines := strings.Split(strings.TrimSpace(parts[0]), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Simple key: value parsing
		colonIdx := strings.Index(line, ":")
		if colonIdx == -1 {
			continue
		}

		key := strings.TrimSpace(line[:colonIdx])
		value := strings.TrimSpace(line[colonIdx+1:])

		// Remove quotes if present
		value = strings.Trim(value, "\"'")

		frontmatter[key] = value
	}

	return frontmatter
}

func isValidSkillName(name string) bool {
	// Skill names should be kebab-case or simple identifiers
	match, _ := regexp.MatchString(`^[a-z][a-z0-9-]*$`, name)
	return match
}
