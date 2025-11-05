package tools

import "context"

type Tool interface {
	Name() string
	Type() string
	Run(ctx context.Context, options *Options) error
	DependsOn() []string
	PostHooks() []string
}

type CommandRunner interface {
	Run(ctx context.Context, command string, args []string) error
}

type ReplacementCommandRunner interface {
	CommandRunner
	RunWithReplacement(ctx context.Context, command string, args []string, replaceToken, replaceFromFile string) error
}

type ToolRegistry interface {
	GetToolConfig(name string) (*ToolConfig, bool)
	GetAllToolConfigs() map[string]*ToolConfig
}
