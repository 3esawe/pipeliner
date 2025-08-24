package tools

import "context"

// Tool represents a runnable tool in the pipeline
type Tool interface {
	Name() string
	Type() string
	Run(ctx context.Context, options *Options) error
	DependsOn() []string
	PostHooks() []string
}

// ProgressReporter allows tools to report their execution progress
type ProgressReporter interface {
	ReportProgress(event ProgressEvent)
}

// ConfigValidator validates tool configurations
type ConfigValidator interface {
	ValidateConfig() error
}

// ResourceManager handles resource allocation and cleanup for tools
type ResourceManager interface {
	AllocateResources(toolName string) error
	ReleaseResources(toolName string) error
}

// CommandExecutor defines how commands are executed
type CommandExecutor interface {
	Execute(ctx context.Context, command string, args []string) error
}

// NotificationSender sends notifications
type NotificationSender interface {
	SendMessage(message string) error
}

// FileWatcher watches for file system events
type FileWatcher interface {
	Watch(ctx context.Context, path string) error
}
