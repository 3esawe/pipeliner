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
	ack       chan struct{}
}

type ConfigurableTool struct {
	name      string
	tool_type string
	config    ToolConfig
	runner    CommandRunner
	progress  chan ProgressEvent
}

func NewConfigurableTool(name string, tool_type string, config ToolConfig, runner CommandRunner) Tool {
	return &ConfigurableTool{
		name:      name,
		tool_type: tool_type,
		config:    config,
		runner:    runner,
		progress:  make(chan ProgressEvent, 500),
	}
}

func (t *ConfigurableTool) Name() string {
	return t.name
}

func (t *ConfigurableTool) Type() string {
	return t.tool_type
}

func (t *ConfigurableTool) DependsOn() []string { return t.config.DependsOn }

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
		log.Infof("Executing command: %s %s", t.config.Command, strings.Join(args, " "))
		err = t.runner.Run(ctx, t.config.Command, args)
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
		log.Warnf("Progress channel full, dropping event for %s", t.name)
	}
}

func (t *ConfigurableTool) sendProgress(event ProgressEvent) {
	select {
	case t.progress <- event:
	default:
		log.Warnf("Progress channel full, dropping event for tool %s", t.name)
	}
}
