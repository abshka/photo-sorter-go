package logger

import (
	"io"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

// LoggerConfig defines the configuration for the logger.
type LoggerConfig struct {
	Level      string // Log level (e.g., "info", "debug", "error")
	FilePath   string // Path to the log file
	MaxSize    int    // Maximum size in megabytes before log rotation
	MaxBackups int    // Maximum number of old log files to retain
	MaxAge     int    // Maximum number of days to retain old log files
	Compress   bool   // Whether to compress rotated log files
	Console    bool   // Whether to also log to the console
}

// NewLogger returns a new logrus.Logger configured according to the provided LoggerConfig.
// The logger supports log rotation and structured JSON output.
func NewLogger(config LoggerConfig) (*logrus.Logger, error) {
	logger := logrus.New()

	level, err := logrus.ParseLevel(config.Level)
	if err != nil {
		return nil, err
	}
	logger.SetLevel(level)

	logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02 15:04:05",
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "timestamp",
			logrus.FieldKeyLevel: "level",
			logrus.FieldKeyMsg:   "message",
			logrus.FieldKeyFunc:  "function",
		},
	})

	var writers []io.Writer

	if config.FilePath != "" {
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

	if config.Console || config.FilePath == "" {
		writers = append(writers, os.Stdout)
	}

	if len(writers) > 1 {
		logger.SetOutput(io.MultiWriter(writers...))
	} else if len(writers) == 1 {
		logger.SetOutput(writers[0])
	}

	return logger, nil
}

// WithFields returns a logger entry with the specified fields.
func WithFields(logger *logrus.Logger, fields logrus.Fields) *logrus.Entry {
	return logger.WithFields(fields)
}

// WithFile returns a logger entry with the specified file context.
func WithFile(logger *logrus.Logger, filePath string) *logrus.Entry {
	return logger.WithField("file", filePath)
}

// WithOperation returns a logger entry with the specified operation context.
func WithOperation(logger *logrus.Logger, operation string) *logrus.Entry {
	return logger.WithField("operation", operation)
}

// WithFileOperation returns a logger entry with both file and operation context.
func WithFileOperation(logger *logrus.Logger, filePath, operation string) *logrus.Entry {
	return logger.WithFields(logrus.Fields{
		"file":      filePath,
		"operation": operation,
	})
}

// DefaultConfig returns the default LoggerConfig.
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
