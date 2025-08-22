package tools

import "context"

type Tool interface {
	Name() string
	Type() string
	Run(ctx context.Context, options *Options) error
	DependsOn() []string
}
