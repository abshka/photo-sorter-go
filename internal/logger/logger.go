package logger

import (
	"io"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

// LoggerConfig holds logger configuration
type LoggerConfig struct {
	Level      string
	FilePath   string
	MaxSize    int // MB
	MaxBackups int
	MaxAge     int // days
	Compress   bool
	Console    bool // Also log to console
}

// NewLogger creates a new structured logger with rotation
func NewLogger(config LoggerConfig) (*logrus.Logger, error) {
	logger := logrus.New()

	// Set log level
	level, err := logrus.ParseLevel(config.Level)
	if err != nil {
		return nil, err
	}
	logger.SetLevel(level)

	// Set JSON formatter for structured logging
	logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02 15:04:05",
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "timestamp",
			logrus.FieldKeyLevel: "level",
			logrus.FieldKeyMsg:   "message",
			logrus.FieldKeyFunc:  "function",
		},
	})

	// Configure output
	var writers []io.Writer

	// File output with rotation
	if config.FilePath != "" {
		// Ensure directory exists
		dir := filepath.Dir(config.FilePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}

		fileWriter := &lumberjack.Logger{
			Filename:   config.FilePath,
			MaxSize:    config.MaxSize,
			MaxBackups: config.MaxBackups,
			MaxAge:     config.MaxAge,
			Compress:   config.Compress,
		}
		writers = append(writers, fileWriter)
	}

	// Console output
	if config.Console || config.FilePath == "" {
		writers = append(writers, os.Stdout)
	}

	// Set output to multi-writer
	if len(writers) > 1 {
		logger.SetOutput(io.MultiWriter(writers...))
	} else if len(writers) == 1 {
		logger.SetOutput(writers[0])
	}

	return logger, nil
}

// WithFields creates a logger entry with additional fields
func WithFields(logger *logrus.Logger, fields logrus.Fields) *logrus.Entry {
	return logger.WithFields(fields)
}

// WithFile creates a logger entry with file context
func WithFile(logger *logrus.Logger, filePath string) *logrus.Entry {
	return logger.WithField("file", filePath)
}

// WithOperation creates a logger entry with operation context
func WithOperation(logger *logrus.Logger, operation string) *logrus.Entry {
	return logger.WithField("operation", operation)
}

// WithFileOperation creates a logger entry with file and operation context
func WithFileOperation(logger *logrus.Logger, filePath, operation string) *logrus.Entry {
	return logger.WithFields(logrus.Fields{
		"file":      filePath,
		"operation": operation,
	})
}

// DefaultConfig returns default logger configuration
func DefaultConfig() LoggerConfig {
	return LoggerConfig{
		Level:      "info",
		FilePath:   "photo-sorter.log",
		MaxSize:    10,
		MaxBackups: 3,
		MaxAge:     30,
		Compress:   true,
		Console:    true,
	}
}
