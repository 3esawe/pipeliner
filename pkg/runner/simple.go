package runner

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"pipeliner/pkg/logger"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
)

// SimpleRunner is a basic command runner that executes system commands
// It implements BaseCommandRunner interface and provides interpreter resolution
type SimpleRunner struct {
	logger *logger.Logger
}

// NewSimpleRunner creates a new SimpleRunner instance
func NewSimpleRunner() *SimpleRunner {
	return &SimpleRunner{
		logger: logger.NewLogger(logrus.InfoLevel),
	}
}

// Run executes a command with automatic interpreter resolution for script files
func (r *SimpleRunner) Run(
	ctx context.Context,
	command string,
	args []string,
) error {
	finalCommand, finalArgs := r.resolveInterpreter(command, args)

	r.logger.WithFields(logger.Fields{
		"command": finalCommand,
		"args":    finalArgs,
	}).Info("Executing command")

	cmd := exec.CommandContext(ctx, finalCommand, finalArgs...)
	err := cmd.Run()
	if err != nil {
		r.logger.WithError(err).Error("Command execution failed")
		return fmt.Errorf("execution failed: %w", err)
	}
	return nil
}

// resolveInterpreter determines the appropriate interpreter for script files
// based on file extension and returns the command and arguments to execute
func (r *SimpleRunner) resolveInterpreter(command string, args []string) (string, []string) {
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
		case ".bat":
			if runtime.GOOS == "windows" {
				return "cmd", append([]string{"/c", command}, args...)
			}
			// On non-Windows, treat .bat files as shell scripts
			return "sh", append([]string{command}, args...)
		case ".ps1":
			return "powershell", append([]string{"-File", command}, args...)
		}
	}

	return command, args
}
