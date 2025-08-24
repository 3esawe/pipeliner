// Package errors defines common error types used throughout the pipeliner application
package errors

import (
	"errors"
	"fmt"
)

// Sentinel errors for common failure scenarios
var (
	ErrToolNotFound         = errors.New("tool not found")
	ErrInvalidConfig        = errors.New("invalid configuration")
	ErrDependencyCycle      = errors.New("dependency cycle detected")
	ErrToolExecutionFailed  = errors.New("tool execution failed")
	ErrDiscordNotConfigured = errors.New("discord client not configured")
)

// ToolError represents an error that occurred during tool execution
type ToolError struct {
	ToolName string
	Err      error
}

func (e *ToolError) Error() string {
	return fmt.Sprintf("tool %s failed: %v", e.ToolName, e.Err)
}

func (e *ToolError) Unwrap() error {
	return e.Err
}

// NewToolError creates a new tool error
func NewToolError(toolName string, err error) *ToolError {
	return &ToolError{
		ToolName: toolName,
		Err:      err,
	}
}

// ConfigError represents a configuration-related error
type ConfigError struct {
	Field   string
	Value   interface{}
	Message string
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("config error for field %s (value: %v): %s", e.Field, e.Value, e.Message)
}

// NewConfigError creates a new configuration error
func NewConfigError(field string, value interface{}, message string) *ConfigError {
	return &ConfigError{
		Field:   field,
		Value:   value,
		Message: message,
	}
}
