package runner

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"pipeliner/pkg/logger"
	"pipeliner/pkg/tools"
	"regexp"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
)

var safeFilename = regexp.MustCompile(`^[a-zA-Z0-9_\-./]+$`)

type SimpleRunner struct {
	logger *logger.Logger
}

func NewSimpleRunner() *SimpleRunner {
	return &SimpleRunner{
		logger: logger.NewLogger(logrus.InfoLevel),
	}
}

func (r *SimpleRunner) Run(ctx context.Context, command string, args []string) error {
	if err := r.validateCommand(command); err != nil {
		return fmt.Errorf("invalid command: %w", err)
	}

	for i, arg := range args {
		if err := r.validateArgument(arg); err != nil {
			return fmt.Errorf("invalid argument at index %d (%s): %w", i, arg, err)
		}
	}

	finalCommand, finalArgs := r.resolveInterpreter(command, args)

	if err := r.validateCommand(finalCommand); err != nil {
		return fmt.Errorf("invalid resolved command: %w", err)
	}

	r.logger.WithFields(logger.Fields{
		"command": finalCommand,
		"args":    finalArgs,
	}).Info("Executing command")

	cmd := exec.CommandContext(ctx, finalCommand, finalArgs...)

	if workDir := tools.GetWorkingDirFromContext(ctx); workDir != "" {
		cmd.Dir = workDir
		r.logger.WithFields(logger.Fields{
			"working_dir": workDir,
		}).Debug("Setting command working directory")
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	stdoutStr := stdout.String()
	stderrStr := stderr.String()

	if err != nil {
		if stderr.Len() > 0 {
			r.logger.WithFields(logger.Fields{
				"stderr": stderrStr,
			}).Error("Command stderr output")
		}
		if stdout.Len() > 0 {
			r.logger.WithFields(logger.Fields{
				"stdout": stdoutStr,
			}).Info("Command stdout output")
		}

		errorMsg := fmt.Sprintf("execution failed: %v", err)
		if stderr.Len() > 0 {
			errorMsg = fmt.Sprintf("%s\nstderr: %s", errorMsg, stderrStr)
		}

		r.logger.WithError(err).Error("Command execution failed")
		return fmt.Errorf("%s", errorMsg)
	}

	if stdout.Len() > 0 {
		r.logger.WithFields(logger.Fields{
			"stdout": stdoutStr,
		}).Debug("Command stdout output")
	}

	return nil
}

func (r *SimpleRunner) validateCommand(command string) error {
	if command == "" {
		return fmt.Errorf("command is empty")
	}

	if strings.Contains(command, ".") {
		if !safeFilename.MatchString(command) {
			return fmt.Errorf("unsafe characters in command: %s", command)
		}

		if _, err := os.Stat(command); err != nil {
			return fmt.Errorf("command file does not exist: %w", err)
		}

		fi, err := os.Lstat(command)
		if err != nil {
			return fmt.Errorf("cannot stat command: %w", err)
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("command is a symlink: %s", command)
		}

		return nil
	}

	return nil
}

func (r *SimpleRunner) validateArgument(arg string) error {
	if arg == "" {
		return nil
	}

	dangerous := []string{";", "&", "|", "`", "$", "(", ")", "\n", "\r", "<", ">"}
	for _, char := range dangerous {
		if strings.Contains(arg, char) {
			return fmt.Errorf("argument contains dangerous character: %s", char)
		}
	}

	if strings.Contains(arg, "..") {
		if !strings.Contains(arg, "://") {
			return fmt.Errorf("path traversal detected in argument")
		}
	}

	return nil
}

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
			return "sh", append([]string{command}, args...)
		case ".ps1":
			return "powershell", append([]string{"-File", command}, args...)
		}
	}

	return command, args
}
