// Package config provides configuration management for iter-service.
package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the service configuration.
type Config struct {
	Service ServiceConfig `yaml:"service"`
	API     APIConfig     `yaml:"api"`
	MCP     MCPConfig     `yaml:"mcp"`
	LLM     LLMConfig     `yaml:"llm"`
}

// ServiceConfig contains service-level settings.
type ServiceConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	DataDir  string `yaml:"data_dir"`
	LogLevel string `yaml:"log_level"`
}

// APIConfig contains API settings.
type APIConfig struct {
	Enabled bool   `yaml:"enabled"`
	APIKey  string `yaml:"api_key"`
}

// MCPConfig contains MCP server settings.
type MCPConfig struct {
	Enabled bool `yaml:"enabled"`
}

// LLMConfig contains LLM integration settings.
type LLMConfig struct {
	APIKey string `yaml:"api_key"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		Service: ServiceConfig{
			Host:     "127.0.0.1",
			Port:     8420,
			DataDir:  DefaultDataDir(),
			LogLevel: "info",
		},
		API: APIConfig{
			Enabled: true,
			APIKey:  "", // Empty = no auth for localhost
		},
		MCP: MCPConfig{
			Enabled: true,
		},
		LLM: LLMConfig{
			APIKey: os.Getenv("GEMINI_API_KEY"),
		},
	}
}

// DefaultDataDir returns the default data directory based on OS.
func DefaultDataDir() string {
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData != "" {
			return filepath.Join(appData, "iter-service")
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "AppData", "Roaming", "iter-service")
	case "darwin":
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", "iter-service")
	default: // linux and others
		// Check XDG_DATA_HOME first
		xdgData := os.Getenv("XDG_DATA_HOME")
		if xdgData != "" {
			return filepath.Join(xdgData, "iter-service")
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".iter-service")
	}
}

// DefaultConfigPath returns the default config file path.
func DefaultConfigPath() string {
	return filepath.Join(DefaultDataDir(), "config.yaml")
}

// Load loads configuration from a file.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return defaults if no config file exists
			return cfg, nil
		}
		return nil, fmt.Errorf("read config file: %w", err)
	}

	// Expand environment variables in the config
	expanded := os.ExpandEnv(string(data))

	if err := yaml.Unmarshal([]byte(expanded), cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	// Expand tilde in data_dir
	if strings.HasPrefix(cfg.Service.DataDir, "~/") {
		home, _ := os.UserHomeDir()
		cfg.Service.DataDir = filepath.Join(home, cfg.Service.DataDir[2:])
	}

	return cfg, nil
}

// Save saves the configuration to a file.
func (c *Config) Save(path string) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	return nil
}

// Address returns the full address string for the HTTP server.
func (c *Config) Address() string {
	return fmt.Sprintf("%s:%d", c.Service.Host, c.Service.Port)
}

// ProjectsDir returns the path to the projects data directory.
func (c *Config) ProjectsDir() string {
	return filepath.Join(c.Service.DataDir, "data", "projects")
}

// RegistryPath returns the path to the project registry file.
func (c *Config) RegistryPath() string {
	return filepath.Join(c.Service.DataDir, "registry.json")
}

// LogPath returns the path to the service log file.
func (c *Config) LogPath() string {
	return filepath.Join(c.Service.DataDir, "logs", "service.log")
}

// EnsureDirectories creates all necessary directories.
func (c *Config) EnsureDirectories() error {
	dirs := []string{
		c.Service.DataDir,
		c.ProjectsDir(),
		filepath.Dir(c.LogPath()),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
	}

	return nil
}

// ProjectHash generates a unique hash for a project path.
// Returns the first 16 characters of the SHA256 hash.
func ProjectHash(path string) string {
	// Normalize the path
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}
	absPath = filepath.Clean(absPath)

	h := sha256.Sum256([]byte(absPath))
	return hex.EncodeToString(h[:])[:16]
}

// ProjectDataDir returns the data directory for a specific project.
func (c *Config) ProjectDataDir(projectPath string) string {
	hash := ProjectHash(projectPath)
	return filepath.Join(c.ProjectsDir(), hash)
}

// ProjectIndexDir returns the index directory for a specific project.
func (c *Config) ProjectIndexDir(projectPath string) string {
	return filepath.Join(c.ProjectDataDir(projectPath), "index")
}
