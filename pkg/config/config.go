// Package config provides configuration loading and management for Iter.
// It supports .claude directory conventions and SKILL.md files.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ternarybob/iter/pkg/sdk"
)

// Load loads configuration from the specified directory.
// It looks for .claude/settings.json and merges with defaults.
func Load(dir string) (*sdk.Config, error) {
	config := DefaultConfig()

	// Try to load .claude/settings.json
	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	if data, err := os.ReadFile(settingsPath); err == nil {
		var fileConfig FileConfig
		if err := json.Unmarshal(data, &fileConfig); err != nil {
			return nil, fmt.Errorf("parse settings.json: %w", err)
		}
		mergeConfig(config, &fileConfig)
	}

	// Set the root directory
	config.Project.RootDir = dir

	return config, nil
}

// LoadFile loads configuration from a specific file.
func LoadFile(path string) (*sdk.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	config := DefaultConfig()
	var fileConfig FileConfig
	if err := json.Unmarshal(data, &fileConfig); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	mergeConfig(config, &fileConfig)
	return config, nil
}

// FileConfig represents the JSON configuration file structure.
type FileConfig struct {
	Project  *ProjectFileConfig  `json:"project,omitempty"`
	Models   *ModelFileConfig    `json:"models,omitempty"`
	Loop     *LoopFileConfig     `json:"loop,omitempty"`
	Exit     *ExitFileConfig     `json:"exit,omitempty"`
	Circuit  *CircuitFileConfig  `json:"circuit,omitempty"`
	Monitor  *MonitorFileConfig  `json:"monitor,omitempty"`
}

// ProjectFileConfig is the project section of the config file.
type ProjectFileConfig struct {
	Name           string   `json:"name,omitempty"`
	RootDir        string   `json:"rootDir,omitempty"`
	IgnorePatterns []string `json:"ignorePatterns,omitempty"`
	IndexPatterns  []string `json:"indexPatterns,omitempty"`
}

// ModelFileConfig is the models section of the config file.
type ModelFileConfig struct {
	Planning   string `json:"planning,omitempty"`
	Execution  string `json:"execution,omitempty"`
	Validation string `json:"validation,omitempty"`
}

// LoopFileConfig is the loop section of the config file.
type LoopFileConfig struct {
	MaxIterations        int    `json:"maxIterations,omitempty"`
	RateLimitPerHour     int    `json:"rateLimitPerHour,omitempty"`
	IterationTimeout     string `json:"iterationTimeout,omitempty"`
	Cooldown             string `json:"cooldown,omitempty"`
	MaxValidationRetries int    `json:"maxValidationRetries,omitempty"`
	ParallelSteps        *bool  `json:"parallelSteps,omitempty"`
}

// ExitFileConfig is the exit section of the config file.
type ExitFileConfig struct {
	RequireExplicitSignal    *bool `json:"requireExplicitSignal,omitempty"`
	CompletionThreshold      int   `json:"completionThreshold,omitempty"`
	MaxConsecutiveNoProgress int   `json:"maxConsecutiveNoProgress,omitempty"`
	MaxConsecutiveErrors     int   `json:"maxConsecutiveErrors,omitempty"`
}

// CircuitFileConfig is the circuit section of the config file.
type CircuitFileConfig struct {
	NoProgressThreshold    int    `json:"noProgressThreshold,omitempty"`
	SameErrorThreshold     int    `json:"sameErrorThreshold,omitempty"`
	OutputDeclineThreshold int    `json:"outputDeclineThreshold,omitempty"`
	RecoveryTimeout        string `json:"recoveryTimeout,omitempty"`
}

// MonitorFileConfig is the monitor section of the config file.
type MonitorFileConfig struct {
	Enabled *bool `json:"enabled,omitempty"`
	Port    int   `json:"port,omitempty"`
}

// mergeConfig merges file config into sdk.Config.
func mergeConfig(config *sdk.Config, file *FileConfig) {
	if file.Project != nil {
		if file.Project.Name != "" {
			config.Project.Name = file.Project.Name
		}
		if file.Project.RootDir != "" {
			config.Project.RootDir = file.Project.RootDir
		}
		if len(file.Project.IgnorePatterns) > 0 {
			config.Project.IgnorePatterns = file.Project.IgnorePatterns
		}
		if len(file.Project.IndexPatterns) > 0 {
			config.Project.IndexPatterns = file.Project.IndexPatterns
		}
	}

	if file.Models != nil {
		if file.Models.Planning != "" {
			config.Models.Planning = file.Models.Planning
		}
		if file.Models.Execution != "" {
			config.Models.Execution = file.Models.Execution
		}
		if file.Models.Validation != "" {
			config.Models.Validation = file.Models.Validation
		}
	}

	if file.Loop != nil {
		if file.Loop.MaxIterations > 0 {
			config.Loop.MaxIterations = file.Loop.MaxIterations
		}
		if file.Loop.RateLimitPerHour > 0 {
			config.Loop.RateLimitPerHour = file.Loop.RateLimitPerHour
		}
		if file.Loop.IterationTimeout != "" {
			config.Loop.IterationTimeout = file.Loop.IterationTimeout
		}
		if file.Loop.Cooldown != "" {
			config.Loop.Cooldown = file.Loop.Cooldown
		}
		if file.Loop.MaxValidationRetries > 0 {
			config.Loop.MaxValidationRetries = file.Loop.MaxValidationRetries
		}
		if file.Loop.ParallelSteps != nil {
			config.Loop.ParallelSteps = *file.Loop.ParallelSteps
		}
	}

	if file.Exit != nil {
		if file.Exit.RequireExplicitSignal != nil {
			config.Exit.RequireExplicitSignal = *file.Exit.RequireExplicitSignal
		}
		if file.Exit.CompletionThreshold > 0 {
			config.Exit.CompletionThreshold = file.Exit.CompletionThreshold
		}
		if file.Exit.MaxConsecutiveNoProgress > 0 {
			config.Exit.MaxConsecutiveNoProgress = file.Exit.MaxConsecutiveNoProgress
		}
		if file.Exit.MaxConsecutiveErrors > 0 {
			config.Exit.MaxConsecutiveErrors = file.Exit.MaxConsecutiveErrors
		}
	}

	if file.Circuit != nil {
		if file.Circuit.NoProgressThreshold > 0 {
			config.Circuit.NoProgressThreshold = file.Circuit.NoProgressThreshold
		}
		if file.Circuit.SameErrorThreshold > 0 {
			config.Circuit.SameErrorThreshold = file.Circuit.SameErrorThreshold
		}
		if file.Circuit.OutputDeclineThreshold > 0 {
			config.Circuit.OutputDeclineThreshold = file.Circuit.OutputDeclineThreshold
		}
		if file.Circuit.RecoveryTimeout != "" {
			config.Circuit.RecoveryTimeout = file.Circuit.RecoveryTimeout
		}
	}

	if file.Monitor != nil {
		if file.Monitor.Enabled != nil {
			config.Monitor.Enabled = *file.Monitor.Enabled
		}
		if file.Monitor.Port > 0 {
			config.Monitor.Port = file.Monitor.Port
		}
	}
}

// Save writes configuration to a file.
func Save(config *sdk.Config, path string) error {
	fileConfig := toFileConfig(config)

	data, err := json.MarshalIndent(fileConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

// toFileConfig converts sdk.Config to FileConfig.
func toFileConfig(config *sdk.Config) *FileConfig {
	parallelSteps := config.Loop.ParallelSteps
	requireExplicit := config.Exit.RequireExplicitSignal
	monitorEnabled := config.Monitor.Enabled

	return &FileConfig{
		Project: &ProjectFileConfig{
			Name:           config.Project.Name,
			RootDir:        config.Project.RootDir,
			IgnorePatterns: config.Project.IgnorePatterns,
			IndexPatterns:  config.Project.IndexPatterns,
		},
		Models: &ModelFileConfig{
			Planning:   config.Models.Planning,
			Execution:  config.Models.Execution,
			Validation: config.Models.Validation,
		},
		Loop: &LoopFileConfig{
			MaxIterations:        config.Loop.MaxIterations,
			RateLimitPerHour:     config.Loop.RateLimitPerHour,
			IterationTimeout:     config.Loop.IterationTimeout,
			Cooldown:             config.Loop.Cooldown,
			MaxValidationRetries: config.Loop.MaxValidationRetries,
			ParallelSteps:        &parallelSteps,
		},
		Exit: &ExitFileConfig{
			RequireExplicitSignal:    &requireExplicit,
			CompletionThreshold:      config.Exit.CompletionThreshold,
			MaxConsecutiveNoProgress: config.Exit.MaxConsecutiveNoProgress,
			MaxConsecutiveErrors:     config.Exit.MaxConsecutiveErrors,
		},
		Circuit: &CircuitFileConfig{
			NoProgressThreshold:    config.Circuit.NoProgressThreshold,
			SameErrorThreshold:     config.Circuit.SameErrorThreshold,
			OutputDeclineThreshold: config.Circuit.OutputDeclineThreshold,
			RecoveryTimeout:        config.Circuit.RecoveryTimeout,
		},
		Monitor: &MonitorFileConfig{
			Enabled: &monitorEnabled,
			Port:    config.Monitor.Port,
		},
	}
}
