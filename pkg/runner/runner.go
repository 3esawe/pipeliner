package runner

import "context"

type CommandRunner interface {
	Run(ctx context.Context, command string, args []string) error
}
