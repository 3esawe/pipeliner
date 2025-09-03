package tools

import (
	"context"
	"fmt"
	"pipeliner/pkg/logger"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

type ProgressEvent struct {
	Tool      string
	Status    string // "started", "running", "completed", "failed"
	Message   string
	Timestamp time.Time
	ack       chan struct{}
}

type ConfigurableTool struct {
	name         string
	tool_type    string
	config       ToolConfig
	runner       CommandRunner
	progress     chan ProgressEvent
	toolRegistry ToolRegistry
	logger       *logger.Logger
}

func NewConfigurableTool(name string, tool_type string, config ToolConfig, runner CommandRunner) Tool {
	return &ConfigurableTool{
		name:         name,
		tool_type:    tool_type,
		config:       config,
		runner:       runner,
		progress:     make(chan ProgressEvent, 500),
		toolRegistry: nil, // Will be set later if needed
		logger:       logger.NewLogger(logrus.InfoLevel),
	}
}

// NewConfigurableToolWithLogger creates a tool with a specific logger
func NewConfigurableToolWithLogger(name string, tool_type string, config ToolConfig, runner CommandRunner, lgr *logger.Logger) Tool {
	return &ConfigurableTool{
		name:         name,
		tool_type:    tool_type,
		config:       config,
		runner:       runner,
		progress:     make(chan ProgressEvent, 500),
		toolRegistry: nil,
		logger:       lgr,
	}
}

// NewConfigurableToolWithRegistry creates a tool with access to the tool registry
func NewConfigurableToolWithRegistry(name string, tool_type string, config ToolConfig, runner CommandRunner, registry ToolRegistry) Tool {
	return &ConfigurableTool{
		name:         name,
		tool_type:    tool_type,
		config:       config,
		runner:       runner,
		progress:     make(chan ProgressEvent, 500),
		toolRegistry: registry,
		logger:       logger.NewLogger(logrus.InfoLevel),
	}
}

// NewConfigurableToolWithRegistryAndLogger creates a tool with registry and logger
func NewConfigurableToolWithRegistryAndLogger(name string, tool_type string, config ToolConfig, runner CommandRunner, registry ToolRegistry, lgr *logger.Logger) Tool {
	return &ConfigurableTool{
		name:         name,
		tool_type:    tool_type,
		config:       config,
		runner:       runner,
		progress:     make(chan ProgressEvent, 500),
		toolRegistry: registry,
		logger:       lgr,
	}
}

// SetToolRegistry sets the tool registry for dynamic configuration resolution
func (t *ConfigurableTool) SetToolRegistry(registry ToolRegistry) {
	t.toolRegistry = registry
}

func (t *ConfigurableTool) Name() string {
	return t.name
}

func (t *ConfigurableTool) Type() string {
	return t.tool_type
}

func (t *ConfigurableTool) DependsOn() []string { return t.config.DependsOn }

func (t *ConfigurableTool) PostHooks() []string { return t.config.PostHooks }

func (t *ConfigurableTool) Run(ctx context.Context, options *Options) error {
	done := make(chan bool, 1)
	eventAck := make(chan struct{})
	go t.monitorProgress(ctx, done)

	t.sendProgress(ProgressEvent{
		Tool:      t.name,
		Status:    "Started",
		Message:   "Running command",
		Timestamp: time.Now(),
	})

	// Build args and run tool
	args, buildErr := t.config.BuildArgs(options)
	var err error
	if buildErr != nil {
		err = fmt.Errorf("failed to build arguments: %w", buildErr)
	} else {
		// Check if this tool requires replacement logic
		if t.config.Replace != "" {
			err = t.runWithReplacement(ctx, args, options)
		} else {
			t.logger.WithTool(t.name, t.tool_type).Infof("Executing command: %s %s", t.config.Command, strings.Join(args, " "))
			err = t.runner.Run(ctx, t.config.Command, args)
		}
	}

	status := "Completed"
	if err != nil {
		status = "Failed"
	}
	t.sendProgressWithAck(ProgressEvent{
		Status:    status,
		Message:   fmt.Sprintf("%s completed", t.name),
		Timestamp: time.Now(),
		Tool:      t.name,
	}, eventAck)
	done <- true
	return err
}

// runWithReplacement executes the tool with replacement logic
func (t *ConfigurableTool) runWithReplacement(ctx context.Context, args []string, options *Options) error {
	// Determine the replacement file
	replaceFromFile := t.config.ReplaceFrom
	if replaceFromFile == "" && len(t.config.DependsOn) > 0 {
		// Use the output of the first dependency as default
		replaceFromFile = t.inferReplacementFile(t.config.DependsOn[0])
	}

	if replaceFromFile == "" {
		return fmt.Errorf("no replacement file specified for tool %s with replace token %s", t.name, t.config.Replace)
	}

	// Check if runner supports replacement
	if replacementRunner, ok := t.runner.(ReplacementCommandRunner); ok {
		t.logger.WithTool(t.name, t.tool_type).Infof("Executing replacement command: %s with token %s from file %s", t.config.Command, t.config.Replace, replaceFromFile)
		return replacementRunner.RunWithReplacement(ctx, t.config.Command, args, t.config.Replace, replaceFromFile)
	}

	return fmt.Errorf("runner does not support replacement for tool %s", t.name)
}

// inferReplacementFile dynamically determines the output file of a dependency tool
func (t *ConfigurableTool) inferReplacementFile(dependencyName string) string {
	// If we have access to the tool registry, use the actual configuration
	if t.toolRegistry != nil {
		if depConfig, exists := t.toolRegistry.GetToolConfig(dependencyName); exists {
			return t.extractOutputFileFromConfig(depConfig)
		}
	}

	// Fallback to default pattern if no registry or tool not found
	t.logger.WithTool(t.name, t.tool_type).Warnf("Could not find dependency tool configuration for %s, using default pattern", dependencyName)
	return fmt.Sprintf("%s_output.txt", dependencyName)
}

// extractOutputFileFromConfig extracts the output filename from a tool's configuration
func (t *ConfigurableTool) extractOutputFileFromConfig(config *ToolConfig) string {
	// Look for output flags in the tool configuration
	for _, flag := range config.Flags {
		if t.isOutputFlag(flag) {
			return flag.Default
		}
	}

	// If no explicit output flag found, use default pattern
	return fmt.Sprintf("%s_output.txt", config.Name)
}

// ExtractOutputFileFromConfig is a public wrapper for testing
func (t *ConfigurableTool) ExtractOutputFileFromConfig(config *ToolConfig) string {
	return t.extractOutputFileFromConfig(config)
}

// isOutputFlag determines if a flag represents an output file specification
func (t *ConfigurableTool) isOutputFlag(flag FlagConfig) bool {
	// Common output flag patterns
	outputFlags := []string{"-o", "--output", "-output", "--out", "-out"}
	outputOptions := []string{"Output", "OutputFile", "Out", "output", "outputfile", "out"}

	// Check if flag matches common output flag patterns
	for _, outputFlag := range outputFlags {
		if flag.Flag == outputFlag {
			return true
		}
	}

	// Check if option name suggests it's an output parameter
	for _, outputOption := range outputOptions {
		if flag.Option == outputOption {
			return true
		}
	}

	return false
}

func (t *ConfigurableTool) monitorProgress(ctx context.Context, done chan bool) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			t.logger.WithTool(t.name, t.tool_type).Infof("Stopping progress monitoring for tool %s", t.name)
			return
		case <-done:
			return
		case event := <-t.progress:
			t.logger.WithTool(t.name, t.tool_type).Infof("Tool: %s, Progress Event: %v, Message: %s, Timestamp: %s", event.Tool, event.Status, event.Message, event.Timestamp)
			if event.ack != nil {
				close(event.ack)
			}
		case <-ticker.C:
			select {
			case t.progress <- ProgressEvent{
				Status:    "Running",
				Message:   fmt.Sprintf("Tool %s is running", t.name),
				Timestamp: time.Now(),
				Tool:      t.name,
			}:
			default:
				// Skip if channel is full to avoid blocking
			}
		}
	}
}

func (t *ConfigurableTool) sendProgressWithAck(event ProgressEvent, ack chan struct{}) {
	event.ack = ack

	select {
	case t.progress <- event:
		// Wait for acknowledgment with timeout
		select {
		case <-ack:
		case <-time.After(500 * time.Millisecond):
		}
	default:
		t.logger.WithTool(t.name, t.tool_type).Warnf("Progress channel full, dropping event for %s", t.name)
	}
}

func (t *ConfigurableTool) sendProgress(event ProgressEvent) {
	select {
	case t.progress <- event:
	default:
		t.logger.WithTool(t.name, t.tool_type).Warnf("Progress channel full, dropping event for tool %s", t.name)
	}
}
