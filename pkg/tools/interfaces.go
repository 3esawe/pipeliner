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

// CommandRunner is the unified interface for command execution
type CommandRunner interface {
	Run(ctx context.Context, command string, args []string) error
}

// ReplacementCommandRunner extends CommandRunner with replacement capabilities
type ReplacementCommandRunner interface {
	CommandRunner
	RunWithReplacement(ctx context.Context, command string, args []string, replaceToken, replaceFromFile string) error
}

// ToolRegistry provides access to tool configurations for dynamic resolution
type ToolRegistry interface {
	GetToolConfig(name string) (*ToolConfig, bool)
	GetAllToolConfigs() map[string]*ToolConfig
}
