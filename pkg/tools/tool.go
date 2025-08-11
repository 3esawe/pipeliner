package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

type ProgressEvent struct {
	Tool      string
	Status    string // "started", "running", "completed", "failed"
	Message   string
	Timestamp time.Time
}

type ConfigurableTool struct {
	name     string
	config   ToolConfig
	runner   CommandRunner
	progress chan ProgressEvent
}

func NewConfigurableTool(name string, config ToolConfig, runner CommandRunner) Tool {
	return &ConfigurableTool{
		name:     name,
		config:   config,
		runner:   runner,
		progress: make(chan ProgressEvent, 100),
	}
}

func (t *ConfigurableTool) Name() string {
	return t.name
}

func (t *ConfigurableTool) Run(ctx context.Context, options *Options) error {
	done := make(chan bool, 1)

	args, err := t.config.BuildArgs(options)
	if err != nil {
		return fmt.Errorf("failed to build arguments: %w", err)
	}

	log.Infof("Executing command: %s %s", t.config.Command, strings.Join(args, " "))

	t.sendProgress(ProgressEvent{
		Tool:      t.name,
		Status:    "Started",
		Message:   "Running command",
		Timestamp: time.Now(),
	})

	go t.monitorProgress(ctx, done)
	err = t.runner.Run(ctx, t.config.Command, args)
	if err != nil {
		t.sendProgress(ProgressEvent{
			Status:    "Failed",
			Message:   fmt.Sprintf("Command failed with error: %s", err.Error()),
			Timestamp: time.Now(),
			Tool:      t.name,
		})
		done <- true // stop monitoring progress for failed tool
		return fmt.Errorf("tool %s failed: %w", t.name, err)
	} else {
		t.sendProgress(ProgressEvent{
			Status:    "Completed",
			Message:   fmt.Sprintf("%s completed successfully", t.name),
			Timestamp: time.Now(),
			Tool:      t.name,
		})

		log.Infof("Tool %s completed successfully", t.name)
		done <- true
		return nil
	}
}

func (t *ConfigurableTool) monitorProgress(ctx context.Context, done chan bool) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Infof("Stopping progress monitoring for tool %s", t.name)
			return
		case <-done:
			return
		case event := <-t.progress:
			log.Infof("Tool: %s, Progress Event: %v, Message: %s, Timestamp: %s", event.Tool, event.Status, event.Message, event.Timestamp)
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

func (t *ConfigurableTool) sendProgress(event ProgressEvent) {
	select {
	case t.progress <- event:
	default:
		log.Warnf("Progress channel full, dropping event for tool %s", t.name)
	}
}
