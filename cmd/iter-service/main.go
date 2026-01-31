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
	"fmt"
	"os"

	"github.com/ternarybob/iter/internal/api"
	"github.com/ternarybob/iter/internal/config"
	"github.com/ternarybob/iter/internal/project"
	"github.com/ternarybob/iter/internal/service"
	"github.com/ternarybob/iter/pkg/index"
)

// version is set via -ldflags at build time
var version = "dev"

func main() {
	// Set version in API package
	api.SetVersion(version)

	if len(os.Args) < 2 {
		// Default: start service
		if err := cmdServe(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	var err error
	switch os.Args[1] {
	case "serve", "start":
		err = cmdServe()
	case "version", "-v", "--version":
		cmdVersion()
	case "status":
		err = cmdStatus()
	case "stop":
		err = cmdStop()
	case "mcp", "mcp-server":
		err = cmdMCP()
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
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
  iter-service [command]

Commands:
  serve         Start the service (default)
  version       Show version information
  status        Show service status
  stop          Stop the running service
  mcp           Start MCP server (stdio mode for Claude integration)
  help          Show this help

Environment:
  GEMINI_API_KEY    API key for LLM features (optional)

Configuration:
  Config file: ~/.iter-service/config.yaml (or $APPDATA/iter-service on Windows)

Examples:
  iter-service                  Start the service
  iter-service mcp              Start MCP server for Claude
  curl localhost:8420/health    Check service health
  curl localhost:8420/projects  List registered projects`)
}

func cmdVersion() {
	fmt.Printf("iter-service version %s\n", version)
}

func cmdServe() error {
	// Load configuration
	cfg, err := config.Load(config.DefaultConfigPath())
	if err != nil {
		return fmt.Errorf("load config: %w", err)
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
	cfg, err := config.Load(config.DefaultConfigPath())
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	running, pid := service.IsRunning(cfg)
	if running {
		fmt.Printf("iter-service: running (PID %d)\n", pid)
		fmt.Printf("Address: %s\n", cfg.Address())
	} else {
		fmt.Println("iter-service: stopped")
	}

	return nil
}

func cmdStop() error {
	cfg, err := config.Load(config.DefaultConfigPath())
	if err != nil {
		return fmt.Errorf("load config: %w", err)
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

func cmdMCP() error {
	// Check for project path argument
	projectPath := "."
	if len(os.Args) > 2 {
		projectPath = os.Args[2]
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
	cfg, err := config.Load(config.DefaultConfigPath())
	if err != nil {
		cfg = config.DefaultConfig()
	}

	// Create index config
	indexCfg := index.Config{
		ProjectID:    config.ProjectHash(absPath),
		ProjectPath:  absPath,
		RepoRoot:     absPath,
		IndexPath:    cfg.ProjectIndexDir(absPath),
		ExcludeGlobs: []string{"vendor/**", "*_test.go", ".git/**", "node_modules/**"},
		DebounceMs:   500,
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

	// Auto-build if index is empty
	if idx.Stats().DocumentCount == 0 {
		fmt.Fprintf(os.Stderr, "[iter-service] Building index for %s...\n", absPath)
		if err := idx.IndexAll(); err != nil {
			return fmt.Errorf("build index: %w", err)
		}
		stats := idx.Stats()
		fmt.Fprintf(os.Stderr, "[iter-service] Indexed %d symbols from %d files\n",
			stats.DocumentCount, stats.FileCount)
	}

	// Start watcher in background
	watcher, err := index.NewWatcher(idx)
	if err == nil {
		if err := watcher.Start(); err == nil {
			defer watcher.Stop()
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
