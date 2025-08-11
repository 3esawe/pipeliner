package tools

import "context"

type Tool interface {
	Name() string
	Run(ctx context.Context, options *Options) error
}
