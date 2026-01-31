// Package main provides the entry point for iter-service.
//
// iter-service is a standalone service providing:
// - REST API for programmatic access
// - Web UI for project management
// - MCP server for Claude Code integration
// - Centralized service with per-project indexes
//
// Usage:
//
//	iter-service                    Start the service (default)
//	iter-service serve              Start the service
//	iter-service version            Show version
//	iter-service status             Show service status
//	iter-service stop               Stop the running service
//	iter-service mcp                Start MCP server (stdio mode)
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/ternarybob/iter/internal/api"
	"github.com/ternarybob/iter/internal/config"
	"github.com/ternarybob/iter/internal/project"
	"github.com/ternarybob/iter/internal/service"
	"github.com/ternarybob/iter/pkg/index"
)

// version is set via -ldflags at build time
var version = "dev"

// Command-line flags
var (
	configPath string
)

func main() {
	// Set version in API package
	api.SetVersion(version)

	// Parse global flags that appear before the command
	args := os.Args[1:]
	command := ""
	cmdArgs := []string{}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--config=") {
			configPath = strings.TrimPrefix(arg, "--config=")
		} else if arg == "--config" && i+1 < len(args) {
			configPath = args[i+1]
			i++
		} else if strings.HasPrefix(arg, "-") {
			// Skip unknown flags for now
		} else if command == "" {
			command = arg
		} else {
			cmdArgs = append(cmdArgs, arg)
		}
	}

	// Default command is serve
	if command == "" {
		command = "serve"
	}

	var err error
	switch command {
	case "serve", "start":
		err = cmdServe(cmdArgs)
	case "version", "-v", "--version":
		cmdVersion()
	case "status":
		err = cmdStatus()
	case "stop":
		err = cmdStop()
	case "mcp", "mcp-server":
		err = cmdMCP(cmdArgs)
	case "init-config":
		err = cmdInitConfig()
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`iter-service - Code indexing and discovery service

Usage:
  iter-service [flags] [command] [args]

Commands:
  serve         Start the service (default)
  version       Show version information
  status        Show service status
  stop          Stop the running service
  mcp           Start MCP server (stdio mode for Claude integration)
  init-config   Create example configuration file
  help          Show this help

Flags:
  --config PATH   Path to configuration file (default: ~/.iter-service/config.toml)

Environment:
  GEMINI_API_KEY    API key for LLM features (optional)
  ITER_CONFIG       Path to configuration file (alternative to --config)
  ITER_DATA_DIR     Override data directory

Configuration:
  Config file: ~/.iter-service/config.toml (TOML format)

Examples:
  iter-service                         Start the service with defaults
  iter-service --config /path/to.toml  Start with custom config
  iter-service mcp                     Start MCP server for Claude
  iter-service init-config             Create example config file
  curl localhost:8420/health           Check service health
  curl localhost:8420/projects         List registered projects`)
}

func cmdVersion() {
	fmt.Printf("iter-service version %s\n", version)
}

func getConfigPath() string {
	// Priority: --config flag > ITER_CONFIG env > default
	if configPath != "" {
		return configPath
	}
	if envPath := os.Getenv("ITER_CONFIG"); envPath != "" {
		return envPath
	}
	return config.DefaultConfigPath()
}

func cmdServe(args []string) error {
	// Parse serve-specific flags
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.Parse(args)

	// Load configuration
	cfg, err := config.Load(getConfigPath())
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Override data dir from environment if set
	if envDataDir := os.Getenv("ITER_DATA_DIR"); envDataDir != "" {
		cfg.Service.DataDir = envDataDir
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// Check if already running
	if running, pid := service.IsRunning(cfg); running {
		return fmt.Errorf("service already running (PID %d)", pid)
	}

	// Create registry
	registry := project.NewRegistry(cfg)
	if err := registry.Load(); err != nil {
		return fmt.Errorf("load registry: %w", err)
	}

	// Create manager
	manager := project.NewManager(cfg, registry)
	if err := manager.Initialize(); err != nil {
		return fmt.Errorf("initialize manager: %w", err)
	}
	defer manager.Shutdown()

	// Create API server
	apiServer := api.NewServer(cfg, registry, manager)

	// Create daemon
	daemon := service.NewDaemon(cfg)

	// Start service
	if err := daemon.Start(apiServer.Handler()); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}

	fmt.Printf("iter-service v%s started on %s\n", version, cfg.Address())
	fmt.Printf("Web UI: http://%s/\n", cfg.Address())
	fmt.Printf("API: http://%s/projects\n", cfg.Address())

	// Wait for shutdown signal
	daemon.Wait()

	return nil
}

func cmdStatus() error {
	cfg, err := config.Load(getConfigPath())
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Override data dir from environment if set
	if envDataDir := os.Getenv("ITER_DATA_DIR"); envDataDir != "" {
		cfg.Service.DataDir = envDataDir
	}

	running, pid := service.IsRunning(cfg)
	if running {
		fmt.Printf("iter-service: running (PID %d)\n", pid)
		fmt.Printf("Address: %s\n", cfg.Address())
		fmt.Printf("Config: %s\n", getConfigPath())
		fmt.Printf("Data: %s\n", cfg.Service.DataDir)
	} else {
		fmt.Println("iter-service: stopped")
	}

	return nil
}

func cmdStop() error {
	cfg, err := config.Load(getConfigPath())
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Override data dir from environment if set
	if envDataDir := os.Getenv("ITER_DATA_DIR"); envDataDir != "" {
		cfg.Service.DataDir = envDataDir
	}

	running, pid := service.IsRunning(cfg)
	if !running {
		fmt.Println("iter-service is not running")
		return nil
	}

	fmt.Printf("Stopping iter-service (PID %d)...\n", pid)
	if err := service.StopRunning(cfg); err != nil {
		return err
	}

	fmt.Println("iter-service stopped")
	return nil
}

func cmdMCP(args []string) error {
	// Check for project path argument
	projectPath := "."
	if len(args) > 0 {
		projectPath = args[0]
	}

	// Get absolute path
	absPath, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	if projectPath != "." {
		absPath = projectPath
	}

	// Check for GEMINI_API_KEY
	if os.Getenv("GEMINI_API_KEY") == "" {
		fmt.Fprintf(os.Stderr, "[iter-service] Warning: GEMINI_API_KEY not set.\n")
		fmt.Fprintf(os.Stderr, "[iter-service] LLM features (commit summaries) disabled.\n")
	}

	// Load config
	cfg, err := config.Load(getConfigPath())
	if err != nil {
		cfg = config.DefaultConfig()
	}

	// Create index config
	indexCfg := index.Config{
		ProjectID:    config.ProjectHash(absPath),
		ProjectPath:  absPath,
		RepoRoot:     absPath,
		IndexPath:    cfg.ProjectIndexDir(absPath),
		ExcludeGlobs: cfg.Index.ExcludeGlobs,
		DebounceMs:   cfg.Index.DebounceMs,
	}

	// Ensure index directory exists
	if err := os.MkdirAll(indexCfg.IndexPath, 0755); err != nil {
		return fmt.Errorf("create index directory: %w", err)
	}

	// Create indexer
	idx, err := index.NewIndexer(indexCfg)
	if err != nil {
		return fmt.Errorf("create indexer: %w", err)
	}

	// Auto-build if index is empty and auto_build_index is enabled
	if cfg.MCP.AutoBuildIndex && idx.Stats().DocumentCount == 0 {
		fmt.Fprintf(os.Stderr, "[iter-service] Building index for %s...\n", absPath)
		if err := idx.IndexAll(); err != nil {
			return fmt.Errorf("build index: %w", err)
		}
		stats := idx.Stats()
		fmt.Fprintf(os.Stderr, "[iter-service] Indexed %d symbols from %d files\n",
			stats.DocumentCount, stats.FileCount)
	}

	// Start watcher in background if enabled
	if cfg.Index.WatchEnabled {
		watcher, err := index.NewWatcher(idx)
		if err == nil {
			if err := watcher.Start(); err == nil {
				defer watcher.Stop()
			}
		}
	}

	// Create and start MCP server
	mcpServer := index.NewMCPServer(idx)

	// Handle context cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		<-ctx.Done()
		// Cleanup on context cancellation
	}()

	return mcpServer.ServeStdio()
}

func cmdInitConfig() error {
	path := getConfigPath()

	// Check if file already exists
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("config file already exists: %s", path)
	}

	if err := config.WriteExampleConfig(path); err != nil {
		return err
	}

	fmt.Printf("Created example configuration: %s\n", path)
	return nil
}
