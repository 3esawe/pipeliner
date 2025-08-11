package runner

import (
	"context"
	"os/exec"

	log "github.com/sirupsen/logrus"
)

type CommandRunner interface {
	Run(ctx context.Context, cmd *exec.Cmd) ([]byte, error)
}

type DefaultCommandRunner struct{}

func (r *DefaultCommandRunner) Run(ctx context.Context, cmd *exec.Cmd) ([]byte, error) {
	log.Debugf("Running command: %s", cmd.String())

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Errorf("Command failed: %v", err)
		return output, err
	}

	return output, nil
}
