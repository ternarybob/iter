package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_DefaultsWhenNoConfig(t *testing.T) {
	// Create a temp directory without config
	tmpDir := t.TempDir()

	cfg, err := Load(tmpDir)
	require.NoError(t, err, "should load without error")

	// Should have defaults
	assert.NotEmpty(t, cfg.Loop.MaxIterations, "should have default max iterations")
	assert.NotEmpty(t, cfg.Loop.RateLimitPerHour, "should have default rate limit")
}

func TestLoad_FromFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .claude directory and settings file
	claudeDir := filepath.Join(tmpDir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))

	settingsContent := `{
		"project": {
			"name": "test-project",
			"ignorePatterns": ["vendor/", "node_modules/"]
		},
		"models": {
			"planning": "claude-opus-4-20250514",
			"execution": "claude-sonnet-4-20250514",
			"validation": "claude-opus-4-20250514"
		},
		"loop": {
			"maxIterations": 50,
			"rateLimitPerHour": 200
		}
	}`

	settingsPath := filepath.Join(claudeDir, "settings.json")
	require.NoError(t, os.WriteFile(settingsPath, []byte(settingsContent), 0644))

	cfg, err := Load(tmpDir)
	require.NoError(t, err, "should load without error")

	assert.Equal(t, "test-project", cfg.Project.Name)
	assert.Equal(t, []string{"vendor/", "node_modules/"}, cfg.Project.IgnorePatterns)
	assert.Equal(t, "claude-opus-4-20250514", cfg.Models.Planning)
	assert.Equal(t, "claude-sonnet-4-20250514", cfg.Models.Execution)
	assert.Equal(t, 50, cfg.Loop.MaxIterations)
	assert.Equal(t, 200, cfg.Loop.RateLimitPerHour)
}

func TestLoad_MergesWithDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	claudeDir := filepath.Join(tmpDir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))

	// Partial config - only project name
	settingsContent := `{
		"project": {
			"name": "partial-config"
		}
	}`

	settingsPath := filepath.Join(claudeDir, "settings.json")
	require.NoError(t, os.WriteFile(settingsPath, []byte(settingsContent), 0644))

	cfg, err := Load(tmpDir)
	require.NoError(t, err, "should load without error")

	// Custom value should be set
	assert.Equal(t, "partial-config", cfg.Project.Name)

	// Defaults should fill in missing values
	assert.NotEmpty(t, cfg.Loop.MaxIterations, "should have default max iterations")
}

func TestSave(t *testing.T) {
	tmpDir := t.TempDir()

	// First load defaults
	cfg := DefaultConfig()
	cfg.Project.Name = "saved-project"
	cfg.Models.Planning = "model-a"
	cfg.Loop.MaxIterations = 25

	// Save to file
	settingsPath := filepath.Join(tmpDir, ".claude", "settings.json")
	err := Save(cfg, settingsPath)
	require.NoError(t, err, "should save without error")

	// Verify file was created
	assert.FileExists(t, settingsPath)

	// Load it back and verify
	loaded, err := LoadFile(settingsPath)
	require.NoError(t, err, "should load saved config")

	assert.Equal(t, cfg.Project.Name, loaded.Project.Name)
	assert.Equal(t, cfg.Models.Planning, loaded.Models.Planning)
	assert.Equal(t, cfg.Loop.MaxIterations, loaded.Loop.MaxIterations)
}

func TestLoad_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	claudeDir := filepath.Join(tmpDir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))

	// Invalid JSON
	settingsPath := filepath.Join(claudeDir, "settings.json")
	require.NoError(t, os.WriteFile(settingsPath, []byte("{ invalid json }"), 0644))

	_, err := Load(tmpDir)
	assert.Error(t, err, "should error on invalid JSON")
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.NotEmpty(t, cfg.Loop.MaxIterations)
	assert.NotEmpty(t, cfg.Loop.RateLimitPerHour)
	assert.NotEmpty(t, cfg.Project.IgnorePatterns)
}

func TestLoadFile(t *testing.T) {
	tmpDir := t.TempDir()
	
	content := `{
		"project": {"name": "file-loaded"},
		"loop": {"maxIterations": 99}
	}`
	
	path := filepath.Join(tmpDir, "config.json")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	cfg, err := LoadFile(path)
	require.NoError(t, err)
	
	assert.Equal(t, "file-loaded", cfg.Project.Name)
	assert.Equal(t, 99, cfg.Loop.MaxIterations)
}

func TestFileConfig_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		jsonContent string
		wantName    string
		wantErr     bool
	}{
		{
			name:        "valid config",
			jsonContent: `{"project": {"name": "test"}}`,
			wantName:    "test",
			wantErr:     false,
		},
		{
			name:        "empty object",
			jsonContent: `{}`,
			wantName:    "",
			wantErr:     false,
		},
		{
			name:        "invalid json",
			jsonContent: `{invalid}`,
			wantName:    "",
			wantErr:     true,
		},
		{
			name:        "nested models",
			jsonContent: `{"models": {"planning": "opus", "execution": "sonnet"}}`,
			wantName:    "",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			claudeDir := filepath.Join(tmpDir, ".claude")
			require.NoError(t, os.MkdirAll(claudeDir, 0755))

			settingsPath := filepath.Join(claudeDir, "settings.json")
			require.NoError(t, os.WriteFile(settingsPath, []byte(tt.jsonContent), 0644))

			cfg, err := Load(tmpDir)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantName, cfg.Project.Name)
		})
	}
}
