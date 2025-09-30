package errors

import (
	"errors"
	"fmt"
)

var (
	ErrToolNotFound         = errors.New("tool not found")
	ErrInvalidConfig        = errors.New("invalid configuration")
	ErrDependencyCycle      = errors.New("dependency cycle detected")
	ErrToolExecutionFailed  = errors.New("tool execution failed")
	ErrDiscordNotConfigured = errors.New("discord client not configured")
)

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

func NewToolError(toolName string, err error) *ToolError {
	return &ToolError{
		ToolName: toolName,
		Err:      err,
	}
}

type ConfigError struct {
	Field   string
	Value   interface{}
	Message string
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("config error for field %s (value: %v): %s", e.Field, e.Value, e.Message)
}

func NewConfigError(field string, value interface{}, message string) *ConfigError {
	return &ConfigError{
		Field:   field,
		Value:   value,
		Message: message,
	}
}
