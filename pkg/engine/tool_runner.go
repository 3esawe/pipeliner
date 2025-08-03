package engine

import (
	"context"
	"fmt"
	"os/exec"

	log "github.com/sirupsen/logrus"
)

type CommandRunner struct{}

func (r *CommandRunner) Run(
	ctx context.Context,
	command string,
	args []string,
) error {
	log.Infof("Executing command: %s with args: %v", command, args)
	cmd := exec.CommandContext(ctx, command, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s failed: %w\nOutput: %s", command, err, output)
	}
	return nil
}
