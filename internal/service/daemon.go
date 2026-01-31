// Package service provides the core service lifecycle management.
package service

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/iter/internal/config"
	"github.com/ternarybob/iter/internal/logger"
)

// Daemon manages the service lifecycle.
type Daemon struct {
	cfg       *config.Config
	server    *http.Server
	logger    arbor.ILogger
	stopCh    chan struct{}
	stoppedCh chan struct{}
	mu        sync.Mutex
	running   bool
}

// NewDaemon creates a new daemon instance.
func NewDaemon(cfg *config.Config) *Daemon {
	return &Daemon{
		cfg:       cfg,
		stopCh:    make(chan struct{}),
		stoppedCh: make(chan struct{}),
	}
}

// Start starts the daemon with the given HTTP handler.
func (d *Daemon) Start(handler http.Handler) error {
	d.mu.Lock()
	if d.running {
		d.mu.Unlock()
		return fmt.Errorf("daemon already running")
	}
	d.running = true
	d.mu.Unlock()

	// Ensure directories exist
	if err := d.cfg.EnsureDirectories(); err != nil {
		return fmt.Errorf("ensure directories: %w", err)
	}

	// Set up logging using arbor
	d.logger = logger.SetupLogger(d.cfg)

	// Write PID file
	if err := d.writePID(); err != nil {
		return fmt.Errorf("write PID: %w", err)
	}

	// Create HTTP server
	d.server = &http.Server{
		Addr:         d.cfg.Address(),
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in goroutine
	go func() {
		d.logger.Info().Str("address", d.cfg.Address()).Msg("Starting server")
		if err := d.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			d.logger.Error().Err(err).Msg("Server error")
		}
	}()

	return nil
}

// Wait waits for the daemon to stop, handling signals.
func (d *Daemon) Wait() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

	select {
	case sig := <-sigCh:
		d.logger.Info().Str("signal", sig.String()).Msg("Received signal, shutting down")
	case <-d.stopCh:
		d.logger.Info().Msg("Stop requested, shutting down")
	}

	d.shutdown()
}

// Stop signals the daemon to stop.
func (d *Daemon) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.running {
		return
	}

	close(d.stopCh)
	<-d.stoppedCh
}

// shutdown performs graceful shutdown.
func (d *Daemon) shutdown() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.running {
		return
	}

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if d.server != nil {
		if err := d.server.Shutdown(ctx); err != nil {
			d.logger.Error().Err(err).Msg("Server shutdown error")
		}
	}

	// Remove PID file
	d.removePID()

	// Stop logger (flush pending logs)
	logger.Stop()

	d.running = false
	close(d.stoppedCh)
}

// writePID writes the current process PID to a file.
func (d *Daemon) writePID() error {
	pidPath := d.cfg.PIDPath()
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(pidPath), 0755); err != nil {
		return fmt.Errorf("create PID directory: %w", err)
	}
	return os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0644)
}

// removePID removes the PID file.
func (d *Daemon) removePID() {
	_ = os.Remove(d.cfg.PIDPath())
}

// IsRunning checks if a daemon is already running.
func IsRunning(cfg *config.Config) (bool, int) {
	pidPath := cfg.PIDPath()

	data, err := os.ReadFile(pidPath)
	if err != nil {
		return false, 0
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return false, 0
	}

	// Check if process exists by sending signal 0
	process, err := os.FindProcess(pid)
	if err != nil {
		return false, 0
	}

	err = process.Signal(syscall.Signal(0))
	if err != nil {
		// Process doesn't exist, clean up stale PID file
		_ = os.Remove(pidPath)
		return false, 0
	}

	return true, pid
}

// StopRunning stops a running daemon.
func StopRunning(cfg *config.Config) error {
	running, pid := IsRunning(cfg)
	if !running {
		return fmt.Errorf("daemon not running")
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process: %w", err)
	}

	// Send SIGTERM
	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("send signal: %w", err)
	}

	// Wait for process to exit
	for i := 0; i < 30; i++ {
		time.Sleep(100 * time.Millisecond)
		if running, _ := IsRunning(cfg); !running {
			return nil
		}
	}

	// Force kill if still running
	if err := process.Kill(); err != nil {
		return fmt.Errorf("kill process: %w", err)
	}

	// Clean up PID file
	_ = os.Remove(cfg.PIDPath())

	return nil
}

// Logger returns the daemon's logger.
func (d *Daemon) Logger() arbor.ILogger {
	return d.logger
}
