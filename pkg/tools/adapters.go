package tools

import (
	"context"
	"pipeliner/pkg/runner"
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
	runner   runner.CommandRunner
	progress chan ProgressEvent
}

func NewConfigurableTool(name string, config ToolConfig, runner runner.CommandRunner) Tool {
	return &ConfigurableTool{
		name:     name,
		config:   config,
		runner:   runner,
		progress: make(chan ProgressEvent, 100), // Buffered channel for progress events
	}
}

func (t *ConfigurableTool) Name() string {
	return t.name
}

func (t *ConfigurableTool) Run(ctx context.Context, opts interface{}) error {
	args, err := t.config.BuildArgs(opts)
	if err != nil {
		return err
	}

	// Send initial progress event
	t.progress <- ProgressEvent{
		Tool:      t.name,
		Status:    "started",
		Message:   "tool is starting",
		Timestamp: time.Now(),
	}

	done := make(chan bool)
	go t.monitorProgress(ctx, done)

	// Run the command
	err = t.runner.Run(ctx, t.config.Command, args)

	// Send completion event
	if err != nil {
		t.progress <- ProgressEvent{
			Tool:      t.name,
			Status:    "failed",
			Message:   err.Error(),
			Timestamp: time.Now(),
		}
	} else {
		t.progress <- ProgressEvent{
			Tool:      t.name,
			Status:    "completed",
			Message:   "tool completed successfully",
			Timestamp: time.Now(),
		}
	}

	// Signal monitor to stop
	done <- true
	return err
}

func (t *ConfigurableTool) monitorProgress(ctx context.Context, done chan bool) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case event := <-t.progress:
			log.Infof("Tool: %s, Status: %s, Message: %s, Timestamp: %s",
				event.Tool, event.Status, event.Message, event.Timestamp.Format(time.RFC3339))
		case <-ticker.C:
			t.progress <- ProgressEvent{
				Tool:      t.name,
				Status:    "running",
				Message:   "tool is running",
				Timestamp: time.Now(),
			}

		}
	}

}
