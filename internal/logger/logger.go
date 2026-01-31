// Package logger provides centralized logging using arbor.
package logger

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/ternarybob/arbor"
	arborcommon "github.com/ternarybob/arbor/common"
	"github.com/ternarybob/arbor/models"
	"github.com/ternarybob/iter/internal/config"
)

var (
	globalLogger arbor.ILogger
	loggerMutex  sync.RWMutex
)

// GetLogger returns the global logger instance.
// If InitLogger() hasn't been called yet, returns a fallback console logger.
func GetLogger() arbor.ILogger {
	loggerMutex.RLock()
	if globalLogger != nil {
		loggerMutex.RUnlock()
		return globalLogger
	}
	loggerMutex.RUnlock()

	loggerMutex.Lock()
	defer loggerMutex.Unlock()

	// Double-check after acquiring write lock
	if globalLogger == nil {
		// WARNING: Using fallback logger - InitLogger() should be called during startup
		globalLogger = arbor.NewLogger().WithConsoleWriter(createWriterConfig(nil, models.LogWriterTypeConsole, ""))
		// Log warning about initialization order issue
		globalLogger.Warn().Msg("Using fallback logger - InitLogger() should be called during startup")
	}
	return globalLogger
}

// InitLogger stores the provided logger as the global singleton instance.
func InitLogger(logger arbor.ILogger) {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	globalLogger = logger
}

// SetupLogger configures and initializes the global logger based on configuration.
func SetupLogger(cfg *config.Config) arbor.ILogger {
	logger := arbor.NewLogger()

	// Get data directory for log files
	logsDir := filepath.Join(cfg.Service.DataDir, "logs")

	// Check if file output is enabled
	hasFileOutput := false
	hasStdoutOutput := false
	for _, output := range cfg.Logging.Output {
		if output == "file" {
			hasFileOutput = true
		}
		if output == "stdout" || output == "console" {
			hasStdoutOutput = true
		}
	}

	// Handle "both" as a single value for backwards compatibility
	if len(cfg.Logging.Output) == 1 && cfg.Logging.Output[0] == "both" {
		hasFileOutput = true
		hasStdoutOutput = true
	}

	// Configure file logging if enabled
	if hasFileOutput {
		if err := os.MkdirAll(logsDir, 0755); err != nil {
			// Use console writer temporarily for this warning
			tempLogger := logger.WithConsoleWriter(createWriterConfig(cfg, models.LogWriterTypeConsole, ""))
			tempLogger.Warn().Err(err).Str("logs_dir", logsDir).Msg("Failed to create logs directory")
		} else {
			logFile := filepath.Join(logsDir, "iter-service.log")
			logger = logger.WithFileWriter(createWriterConfig(cfg, models.LogWriterTypeFile, logFile))
		}
	}

	// Configure console logging if enabled
	if hasStdoutOutput {
		logger = logger.WithConsoleWriter(createWriterConfig(cfg, models.LogWriterTypeConsole, ""))
	}

	// Ensure at least one visible log writer is configured
	if !hasFileOutput && !hasStdoutOutput {
		logger = logger.WithConsoleWriter(createWriterConfig(cfg, models.LogWriterTypeConsole, ""))
		logger.Warn().
			Strs("configured_outputs", cfg.Logging.Output).
			Msg("No visible log outputs configured - falling back to console")
	}

	// Always add memory writer for potential log streaming
	logger = logger.WithMemoryWriter(createWriterConfig(cfg, models.LogWriterTypeMemory, ""))

	// Set log level
	logger = logger.WithLevelFromString(cfg.Logging.Level)

	// Store logger in singleton for global access
	InitLogger(logger)

	return logger
}

// createWriterConfig creates a standard writer configuration with user preferences.
func createWriterConfig(cfg *config.Config, writerType models.LogWriterType, filename string) models.WriterConfiguration {
	// Default time format if not specified (HH:MM:SS.mmm for alignment)
	timeFormat := "15:04:05.000"
	if cfg != nil && cfg.Logging.TimeFormat != "" {
		timeFormat = cfg.Logging.TimeFormat
	}

	// Determine output format (text/logfmt vs JSON)
	outputType := models.OutputFormatJSON
	if cfg != nil && cfg.Logging.Format == "text" {
		outputType = models.OutputFormatLogfmt
	}

	// Calculate max size in bytes
	var maxSize int64 = 100 * 1024 * 1024 // 100 MB default
	if cfg != nil && cfg.Logging.MaxSizeMB > 0 {
		maxSize = int64(cfg.Logging.MaxSizeMB) * 1024 * 1024
	}

	maxBackups := 5
	if cfg != nil && cfg.Logging.MaxBackups > 0 {
		maxBackups = cfg.Logging.MaxBackups
	}

	return models.WriterConfiguration{
		Type:             writerType,
		FileName:         filename,
		TimeFormat:       timeFormat,
		OutputType:       outputType,
		DisableTimestamp: false,
		MaxSize:          maxSize,
		MaxBackups:       maxBackups,
	}
}

// Stop flushes any remaining context logs before application shutdown.
// Safe to call multiple times (Arbor's Stop is idempotent).
func Stop() {
	arborcommon.Stop()
}
