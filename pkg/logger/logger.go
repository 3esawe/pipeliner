// Package logger provides structured logging for the pipeliner application
package logger

import (
	"context"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

// Fields represents structured log fields
type Fields map[string]interface{}

// Logger wraps logrus.Logger with additional functionality
type Logger struct {
	*logrus.Logger
}

// NewLogger creates a new structured logger
func NewLogger(level logrus.Level) *Logger {
	logger := logrus.New()

	// Set log level
	logger.SetLevel(level)

	// Use JSON formatter for structured logging in production
	if os.Getenv("ENV") == "production" {
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: time.RFC3339Nano,
		})
	} else {
		// Use text formatter for development
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: time.RFC3339,
		})
	}

	return &Logger{Logger: logger}
}

// WithContext adds context-specific fields to the logger
func (l *Logger) WithContext(ctx context.Context) *logrus.Entry {
	entry := l.Logger.WithContext(ctx)

	// Add request ID if available
	if reqID := ctx.Value("request_id"); reqID != nil {
		entry = entry.WithField("request_id", reqID)
	}

	// Add correlation ID if available
	if corrID := ctx.Value("correlation_id"); corrID != nil {
		entry = entry.WithField("correlation_id", corrID)
	}

	return entry
}

// WithTool adds tool-specific fields to the logger
func (l *Logger) WithTool(toolName, toolType string) *logrus.Entry {
	return l.Logger.WithFields(logrus.Fields{
		"tool_name": toolName,
		"tool_type": toolType,
	})
}

// WithError adds error context to the logger
func (l *Logger) WithError(err error) *logrus.Entry {
	return l.Logger.WithError(err)
}

// WithFields adds multiple fields to the logger
func (l *Logger) WithFields(fields Fields) *logrus.Entry {
	return l.Logger.WithFields(logrus.Fields(fields))
}

// LogToolExecution logs the start and end of tool execution
func (l *Logger) LogToolExecution(toolName string, fn func() error) error {
	start := time.Now()

	l.WithFields(Fields{
		"tool_name": toolName,
		"action":    "start",
	}).Info("Tool execution started")

	err := fn()
	duration := time.Since(start)

	fields := Fields{
		"tool_name": toolName,
		"action":    "complete",
		"duration":  duration.String(),
	}

	if err != nil {
		fields["error"] = err.Error()
		l.WithFields(fields).Error("Tool execution failed")
	} else {
		l.WithFields(fields).Info("Tool execution completed successfully")
	}

	return err
}

// Default logger instance
var defaultLogger = NewLogger(logrus.InfoLevel)

// SetLevel sets the log level for the default logger
func SetLevel(level logrus.Level) {
	defaultLogger.SetLevel(level)
}

// Info logs an info message using the default logger
func Info(args ...interface{}) {
	defaultLogger.Info(args...)
}

// Infof logs a formatted info message using the default logger
func Infof(format string, args ...interface{}) {
	defaultLogger.Infof(format, args...)
}

// Error logs an error message using the default logger
func Error(args ...interface{}) {
	defaultLogger.Error(args...)
}

// Errorf logs a formatted error message using the default logger
func Errorf(format string, args ...interface{}) {
	defaultLogger.Errorf(format, args...)
}

// WithFields returns an entry with the specified fields using the default logger
func WithFields(fields Fields) *logrus.Entry {
	return defaultLogger.WithFields(fields)
}

// WithContext returns an entry with context using the default logger
func WithContext(ctx context.Context) *logrus.Entry {
	return defaultLogger.WithContext(ctx)
}
