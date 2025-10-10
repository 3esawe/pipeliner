package runner

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"pipeliner/pkg/logger"
	"regexp"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
)

var (
	safeFilename = regexp.MustCompile(`^[a-zA-Z0-9_\-./]+$`)
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
	// Validate command
	if err := r.validateCommand(command); err != nil {
		return fmt.Errorf("invalid command: %w", err)
	}

	// Validate all arguments
	for i, arg := range args {
		if err := r.validateArgument(arg); err != nil {
			return fmt.Errorf("invalid argument at index %d (%s): %w", i, arg, err)
		}
	}

	finalCommand, finalArgs := r.resolveInterpreter(command, args)

	// Final validation after interpreter resolution
	if err := r.validateCommand(finalCommand); err != nil {
		return fmt.Errorf("invalid resolved command: %w", err)
	}

	r.logger.WithFields(logger.Fields{
		"command": finalCommand,
		"args":    finalArgs,
	}).Info("Executing command")

	cmd := exec.CommandContext(ctx, finalCommand, finalArgs...)

	// Capture both stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// Log the captured output
		if stderr.Len() > 0 {
			r.logger.WithFields(logger.Fields{
				"stderr": stderr.String(),
			}).Error("Command stderr output")
		}
		if stdout.Len() > 0 {
			r.logger.WithFields(logger.Fields{
				"stdout": stdout.String(),
			}).Info("Command stdout output")
		}

		// Include stderr in the error message
		errorMsg := fmt.Sprintf("execution failed: %v", err)
		if stderr.Len() > 0 {
			errorMsg = fmt.Sprintf("%s\nstderr: %s", errorMsg, stderr.String())
		}

		r.logger.WithError(err).Error("Command execution failed")
		return fmt.Errorf("%s", errorMsg)
	}

	// Log successful output if there's any
	if stdout.Len() > 0 {
		r.logger.WithFields(logger.Fields{
			"stdout": stdout.String(),
		}).Debug("Command stdout output")
	}

	return nil
}

// validateCommand validates that a command is safe to execute
func (r *SimpleRunner) validateCommand(command string) error {
	if command == "" {
		return fmt.Errorf("command is empty")
	}

	// For script files, validate path
	if strings.Contains(command, ".") {
		// Must be a safe filename
		if !safeFilename.MatchString(command) {
			return fmt.Errorf("unsafe characters in command: %s", command)
		}

		// Must exist
		if _, err := os.Stat(command); err != nil {
			return fmt.Errorf("command file does not exist: %w", err)
		}

		// Must not be a symlink (prevent symlink attacks)
		fi, err := os.Lstat(command)
		if err != nil {
			return fmt.Errorf("cannot stat command: %w", err)
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("command is a symlink: %s", command)
		}

		return nil
	}

	return fmt.Errorf("command not in whitelist: %s (add to allowedCommands if this is a valid tool)", command)
}

// validateArgument validates that a command argument is safe
func (r *SimpleRunner) validateArgument(arg string) error {
	if arg == "" {
		return nil // Empty arguments are allowed
	}

	// Check for shell metacharacters that could enable command injection
	dangerous := []string{";", "&", "|", "`", "$", "(", ")", "\n", "\r", "<", ">"}
	for _, char := range dangerous {
		if strings.Contains(arg, char) {
			return fmt.Errorf("argument contains dangerous character: %s", char)
		}
	}

	// Check for path traversal
	if strings.Contains(arg, "..") {
		// Allow .. in URLs but not in file paths
		if !strings.Contains(arg, "://") {
			return fmt.Errorf("path traversal detected in argument")
		}
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
