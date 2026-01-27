package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
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

				// Note: skills array is NOT required in marketplace.json
				// Skills are auto-discovered from the skills/ directory per Claude Code docs
				// See: https://code.claude.com/docs/en/plugins-reference

				// Check strict field is false if present (required for custom component paths)
				strict, ok := plugin["strict"]
				if ok && strict.(bool) {
					t.Logf("plugin %d has strict=true - custom paths will be ignored", i)
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

	// New unified structure: only "iter" (main skill) and "install" (wrapper installer)
	expectedSkills := []string{
		"iter",
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

// TestParseUnifiedArgs tests the new unified argument parsing
func TestParseUnifiedArgs(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantMode    string
		wantModeArg string
		wantDesc    string
	}{
		{
			name:     "version flag -v",
			args:     []string{"-v"},
			wantMode: "version",
		},
		{
			name:     "version flag --version",
			args:     []string{"--version"},
			wantMode: "version",
		},
		{
			name:     "help flag",
			args:     []string{"-h"},
			wantMode: "help",
		},
		{
			name:     "reindex flag",
			args:     []string{"-r"},
			wantMode: "reindex",
		},
		{
			name:        "test flag",
			args:        []string{"-t:tests/foo_test.go", "check", "installation"},
			wantMode:    "test",
			wantModeArg: "tests/foo_test.go",
			wantDesc:    "check installation",
		},
		{
			name:        "workflow flag",
			args:        []string{"-w:workflow.md", "include", "logs"},
			wantMode:    "workflow",
			wantModeArg: "workflow.md",
			wantDesc:    "include logs",
		},
		{
			name:     "run mode (default)",
			args:     []string{"add health check endpoint"},
			wantMode: "run",
			wantDesc: "add health check endpoint",
		},
		{
			name:     "internal command status",
			args:     []string{"status"},
			wantMode: "status",
		},
		{
			name:     "internal command complete",
			args:     []string{"complete"},
			wantMode: "complete",
		},
		{
			name:     "empty args",
			args:     []string{},
			wantMode: "help",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode, modeArg, desc, _ := parseUnifiedArgs(tt.args)
			if mode != tt.wantMode {
				t.Errorf("mode = %q, want %q", mode, tt.wantMode)
			}
			if modeArg != tt.wantModeArg {
				t.Errorf("modeArg = %q, want %q", modeArg, tt.wantModeArg)
			}
			if desc != tt.wantDesc {
				t.Errorf("desc = %q, want %q", desc, tt.wantDesc)
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
