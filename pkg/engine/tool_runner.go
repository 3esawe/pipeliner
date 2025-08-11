package engine

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	log "github.com/sirupsen/logrus"
)

type CommandRunner struct{}

func (r *CommandRunner) Run(
	ctx context.Context,
	command string,
	args []string,
) error {
	finalCommand, finalArgs := r.resolveInterpreter(command, args)

	log.Infof("Executing: %s %v", finalCommand, finalArgs)
	cmd := exec.CommandContext(ctx, finalCommand, finalArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("execution failed: %w\nOutput: %s", err, output)
	}
	return nil
}

func (r *CommandRunner) resolveInterpreter(command string, args []string) (string, []string) {
	if strings.Contains(command, ".") {
		ext := filepath.Ext(command)

		switch ext {
		case ".py":
			return "python3", append([]string{command}, args...)
		case ".js":
			return "node", append([]string{command}, args...)
		case ".rb":
			return "ruby", append([]string{command}, args...)
		case ".sh":
			if runtime.GOOS == "windows" {
				return "bash", append([]string{command}, args...)
			}
			return "sh", append([]string{command}, args...)
		}
	}

	return command, args
}
