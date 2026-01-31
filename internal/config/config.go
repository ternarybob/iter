// Package config provides configuration management for iter-service.
package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config represents the service configuration.
type Config struct {
	Service  ServiceConfig  `toml:"service"`
	API      APIConfig      `toml:"api"`
	MCP      MCPConfig      `toml:"mcp"`
	LLM      LLMConfig      `toml:"llm"`
	Index    IndexConfig    `toml:"index"`
	Logging  LoggingConfig  `toml:"logging"`
	Security SecurityConfig `toml:"security"`
}

// ServiceConfig contains service-level settings.
type ServiceConfig struct {
	Host            string `toml:"host"`
	Port            int    `toml:"port"`
	DataDir         string `toml:"data_dir"`
	PIDFile         string `toml:"pid_file"`
	ShutdownTimeout int    `toml:"shutdown_timeout_seconds"`
	MaxRequestSize  int64  `toml:"max_request_size_bytes"`
}

// APIConfig contains API settings.
type APIConfig struct {
	Enabled        bool     `toml:"enabled"`
	APIKey         string   `toml:"api_key"`
	RateLimit      int      `toml:"rate_limit_per_minute"`
	AllowedOrigins []string `toml:"allowed_origins"`
	RequestTimeout int      `toml:"request_timeout_seconds"`
}

// MCPConfig contains MCP server settings.
type MCPConfig struct {
	Enabled        bool `toml:"enabled"`
	AutoBuildIndex bool `toml:"auto_build_index"`
}

// LLMConfig contains LLM integration settings.
type LLMConfig struct {
	Provider    string  `toml:"provider"`
	APIKey      string  `toml:"api_key"`
	Model       string  `toml:"model"`
	MaxTokens   int     `toml:"max_tokens"`
	Temperature float64 `toml:"temperature"`
	TimeoutSecs int     `toml:"timeout_seconds"`
}

// IndexConfig contains indexing settings.
type IndexConfig struct {
	ExcludeGlobs      []string `toml:"exclude_globs"`
	IncludeExts       []string `toml:"include_extensions"`
	MaxFileSize       int64    `toml:"max_file_size_bytes"`
	DebounceMs        int      `toml:"debounce_ms"`
	WatchEnabled      bool     `toml:"watch_enabled"`
	MaxSymbolsPerFile int      `toml:"max_symbols_per_file"`
	EmbeddingModel    string   `toml:"embedding_model"`
}

// LoggingConfig contains logging settings.
type LoggingConfig struct {
	Level      string       `toml:"level"`
	Format     string       `toml:"format"`
	Output     StringSlice  `toml:"output"`
	TimeFormat string       `toml:"time_format"`
	MaxSizeMB  int          `toml:"max_size_mb"`
	MaxBackups int          `toml:"max_backups"`
	MaxAgeDays int          `toml:"max_age_days"`
	Compress   bool         `toml:"compress"`
}

// StringSlice is a custom type that can unmarshal from either a string or []string.
type StringSlice []string

// UnmarshalTOML implements toml.Unmarshaler for flexible config parsing.
func (s *StringSlice) UnmarshalTOML(data interface{}) error {
	switch v := data.(type) {
	case string:
		*s = []string{v}
	case []interface{}:
		result := make([]string, len(v))
		for i, item := range v {
			str, ok := item.(string)
			if !ok {
				return fmt.Errorf("expected string in array, got %T", item)
			}
			result[i] = str
		}
		*s = result
	default:
		return fmt.Errorf("expected string or array, got %T", data)
	}
	return nil
}

// SecurityConfig contains security settings.
type SecurityConfig struct {
	TLSEnabled  bool   `toml:"tls_enabled"`
	TLSCertFile string `toml:"tls_cert_file"`
	TLSKeyFile  string `toml:"tls_key_file"`
	CORSEnabled bool   `toml:"cors_enabled"`
}

// DefaultConfig returns the default configuration with all values set.
// Environment variables ITER_HOST and ITER_PORT can override defaults.
func DefaultConfig() *Config {
	dataDir := DefaultDataDir()

	// Check for environment variable overrides
	host := "127.0.0.1"
	if envHost := os.Getenv("ITER_HOST"); envHost != "" {
		host = envHost
	}

	port := 8420
	if envPort := os.Getenv("ITER_PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			port = p
		}
	}

	return &Config{
		Service: ServiceConfig{
			Host:            host,
			Port:            port,
			DataDir:         dataDir,
			PIDFile:         filepath.Join(dataDir, "iter-service.pid"),
			ShutdownTimeout: 30,
			MaxRequestSize:  10 * 1024 * 1024, // 10MB
		},
		API: APIConfig{
			Enabled:        true,
			APIKey:         "", // Empty = no auth for localhost
			RateLimit:      100,
			AllowedOrigins: []string{"http://localhost:*", "http://127.0.0.1:*"},
			RequestTimeout: 60,
		},
		MCP: MCPConfig{
			Enabled:        true,
			AutoBuildIndex: true,
		},
		LLM: LLMConfig{
			Provider:    "gemini",
			APIKey:      os.Getenv("GEMINI_API_KEY"),
			Model:       "gemini-1.5-flash",
			MaxTokens:   1024,
			Temperature: 0.3,
			TimeoutSecs: 30,
		},
		Index: IndexConfig{
			ExcludeGlobs: []string{
				"vendor/**",
				"node_modules/**",
				".git/**",
				"*.min.js",
				"*.min.css",
				"dist/**",
				"build/**",
				"__pycache__/**",
				"*.pyc",
				".venv/**",
				"target/**",
			},
			IncludeExts: []string{
				".go", ".py", ".js", ".ts", ".tsx", ".jsx",
				".java", ".kt", ".scala", ".rs", ".c", ".cpp",
				".h", ".hpp", ".cs", ".rb", ".php", ".swift",
				".m", ".mm", ".sql", ".sh", ".bash", ".zsh",
			},
			MaxFileSize:       1024 * 1024, // 1MB
			DebounceMs:        500,
			WatchEnabled:      true,
			MaxSymbolsPerFile: 1000,
			EmbeddingModel:    "nomic-embed-text-v1.5",
		},
		Logging: LoggingConfig{
			Level:      "info",
			Format:     "text",
			Output:     StringSlice{"file"},
			TimeFormat: "15:04:05.000",
			MaxSizeMB:  100,
			MaxBackups: 5,
			MaxAgeDays: 30,
			Compress:   true,
		},
		Security: SecurityConfig{
			TLSEnabled:  false,
			TLSCertFile: "",
			TLSKeyFile:  "",
			CORSEnabled: true,
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
	return filepath.Join(DefaultDataDir(), "config.toml")
}

// Load loads configuration from a file, merging with defaults.
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

	if _, err := toml.Decode(expanded, cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	// Expand tilde in paths
	cfg.expandPaths()

	return cfg, nil
}

// LoadFromString loads configuration from a TOML string, merging with defaults.
func LoadFromString(tomlStr string) (*Config, error) {
	cfg := DefaultConfig()

	// Expand environment variables
	expanded := os.ExpandEnv(tomlStr)

	if _, err := toml.Decode(expanded, cfg); err != nil {
		return nil, fmt.Errorf("parse config string: %w", err)
	}

	cfg.expandPaths()
	return cfg, nil
}

// expandPaths expands tilde in path fields.
func (c *Config) expandPaths() {
	home, _ := os.UserHomeDir()

	expandTilde := func(path string) string {
		if strings.HasPrefix(path, "~/") {
			return filepath.Join(home, path[2:])
		}
		return path
	}

	c.Service.DataDir = expandTilde(c.Service.DataDir)
	c.Service.PIDFile = expandTilde(c.Service.PIDFile)
	c.Security.TLSCertFile = expandTilde(c.Security.TLSCertFile)
	c.Security.TLSKeyFile = expandTilde(c.Security.TLSKeyFile)
}

// Save saves the configuration to a file in TOML format.
func (c *Config) Save(path string) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create config file: %w", err)
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	if err := encoder.Encode(c); err != nil {
		return fmt.Errorf("encode config: %w", err)
	}

	return nil
}

// WriteExampleConfig writes an example config file with comments.
func WriteExampleConfig(path string) error {
	example := `# iter-service configuration file
# All values shown are defaults - uncomment and modify as needed

[service]
# Host to bind the HTTP server to
host = "127.0.0.1"
# Port to listen on
port = 8420
# Directory for service data (indexes, registry, logs)
# data_dir = "~/.iter-service"
# PID file location
# pid_file = "~/.iter-service/iter-service.pid"
# Graceful shutdown timeout in seconds
shutdown_timeout_seconds = 30
# Maximum request body size in bytes (10MB default)
max_request_size_bytes = 10485760

[api]
# Enable the REST API
enabled = true
# API key for authentication (empty = no auth for localhost)
api_key = ""
# Rate limit requests per minute (0 = unlimited)
rate_limit_per_minute = 100
# Allowed CORS origins
allowed_origins = ["http://localhost:*", "http://127.0.0.1:*"]
# Request timeout in seconds
request_timeout_seconds = 60

[mcp]
# Enable MCP server mode
enabled = true
# Automatically build index when starting MCP server
auto_build_index = true

[llm]
# LLM provider (gemini, openai, anthropic)
provider = "gemini"
# API key (can use environment variable: ${GEMINI_API_KEY})
api_key = "${GEMINI_API_KEY}"
# Model to use
model = "gemini-1.5-flash"
# Maximum tokens for responses
max_tokens = 1024
# Temperature for generation (0.0-1.0)
temperature = 0.3
# Timeout in seconds
timeout_seconds = 30

[index]
# Glob patterns to exclude from indexing
exclude_globs = [
    "vendor/**",
    "node_modules/**",
    ".git/**",
    "*.min.js",
    "*.min.css",
    "dist/**",
    "build/**",
    "__pycache__/**",
    "*.pyc",
    ".venv/**",
    "target/**",
]
# File extensions to include
include_extensions = [
    ".go", ".py", ".js", ".ts", ".tsx", ".jsx",
    ".java", ".kt", ".scala", ".rs", ".c", ".cpp",
    ".h", ".hpp", ".cs", ".rb", ".php", ".swift",
]
# Maximum file size to index in bytes (1MB default)
max_file_size_bytes = 1048576
# File change debounce time in milliseconds
debounce_ms = 500
# Enable file watching for automatic re-indexing
watch_enabled = true
# Maximum symbols to extract per file
max_symbols_per_file = 1000
# Embedding model for semantic search
embedding_model = "nomic-embed-text-v1.5"

[logging]
# Log level: debug, info, warn, error
level = "info"
# Log format: json, text
format = "text"
# Output destinations: "file", "stdout", or both
output = ["file"]
# Time format for log timestamps (Go time format)
time_format = "15:04:05.000"
# Maximum log file size in MB before rotation
max_size_mb = 100
# Number of backup log files to keep
max_backups = 5
# Maximum age of log files in days
max_age_days = 30
# Compress rotated log files
compress = true

[security]
# Enable TLS/HTTPS
tls_enabled = false
# Path to TLS certificate file
# tls_cert_file = "/path/to/cert.pem"
# Path to TLS key file
# tls_key_file = "/path/to/key.pem"
# Enable CORS
cors_enabled = true
`

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	return os.WriteFile(path, []byte(example), 0644)
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

// PIDPath returns the path to the PID file.
func (c *Config) PIDPath() string {
	if c.Service.PIDFile != "" {
		return c.Service.PIDFile
	}
	return filepath.Join(c.Service.DataDir, "iter-service.pid")
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

// Validate validates the configuration and returns any errors.
func (c *Config) Validate() error {
	if c.Service.Port < 1 || c.Service.Port > 65535 {
		return fmt.Errorf("invalid port: %d (must be 1-65535)", c.Service.Port)
	}

	if c.Service.ShutdownTimeout < 1 {
		return fmt.Errorf("shutdown_timeout_seconds must be at least 1")
	}

	if c.API.RateLimit < 0 {
		return fmt.Errorf("rate_limit_per_minute cannot be negative")
	}

	if c.LLM.Temperature < 0 || c.LLM.Temperature > 1 {
		return fmt.Errorf("temperature must be between 0.0 and 1.0")
	}

	if c.Security.TLSEnabled {
		if c.Security.TLSCertFile == "" || c.Security.TLSKeyFile == "" {
			return fmt.Errorf("TLS enabled but cert/key files not specified")
		}
	}

	return nil
}

// Clone creates a deep copy of the configuration.
func (c *Config) Clone() *Config {
	clone := *c

	// Deep copy slices
	clone.API.AllowedOrigins = make([]string, len(c.API.AllowedOrigins))
	copy(clone.API.AllowedOrigins, c.API.AllowedOrigins)

	clone.Index.ExcludeGlobs = make([]string, len(c.Index.ExcludeGlobs))
	copy(clone.Index.ExcludeGlobs, c.Index.ExcludeGlobs)

	clone.Index.IncludeExts = make([]string, len(c.Index.IncludeExts))
	copy(clone.Index.IncludeExts, c.Index.IncludeExts)

	clone.Logging.Output = make(StringSlice, len(c.Logging.Output))
	copy(clone.Logging.Output, c.Logging.Output)

	return &clone
}
