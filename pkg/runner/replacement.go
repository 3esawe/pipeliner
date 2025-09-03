package runner

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"pipeliner/pkg/logger"
	"pipeliner/pkg/tools"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"
)

// ReplacementCommandRunner handles command execution with URL/value replacement
// It implements tools.ReplacementCommandRunner interface
type ReplacementCommandRunner struct {
	baseRunner tools.CommandRunner
	logger     *logger.Logger
}

// NewReplacementCommandRunner creates a new replacement command runner
func NewReplacementCommandRunner(baseRunner tools.CommandRunner) *ReplacementCommandRunner {
	return &ReplacementCommandRunner{
		baseRunner: baseRunner,
		logger:     logger.NewLogger(logrus.InfoLevel),
	}
}

// Run executes a command using the base runner (implements tools.CommandRunner)
func (r *ReplacementCommandRunner) Run(ctx context.Context, command string, args []string) error {
	r.logger.WithFields(logger.Fields{
		"command": command,
		"args":    args,
	}).Info("Running command")
	return r.baseRunner.Run(ctx, command, args)
}

// RunWithReplacement executes a command for each line in the replacement file
func (r *ReplacementCommandRunner) RunWithReplacement(ctx context.Context, command string, args []string, replaceToken, replaceFromFile string) error {
	r.logger.WithFields(logger.Fields{
		"command":          command,
		"args":             args,
		"token":            replaceToken,
		"replacement_file": replaceFromFile,
	}).Info("Running replacement command")

	// Read the replacement values from file
	replacementValues, err := r.readReplacementValues(replaceFromFile)
	if err != nil {
		return fmt.Errorf("failed to read replacement values from %s: %w", replaceFromFile, err)
	}

	if len(replacementValues) == 0 {
		r.logger.WithFields(logger.Fields{
			"file": replaceFromFile,
		}).Warn("No replacement values found in file")
		return nil
	}

	r.logger.WithFields(logger.Fields{
		"count": len(replacementValues),
		"file":  replaceFromFile,
	}).Info("Found replacement values")

	// Execute command for each replacement value
	for i, value := range replacementValues {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		r.logger.WithFields(logger.Fields{
			"current": i + 1,
			"total":   len(replacementValues),
			"value":   value,
		}).Info("Processing replacement")

		// Replace the token in args with the current value
		replacedArgs := r.replaceInArgs(args, replaceToken, value)

		r.logger.WithFields(logger.Fields{
			"command": command,
			"args":    strings.Join(replacedArgs, " "),
		}).Info("Executing replacement command")

		err := r.baseRunner.Run(ctx, command, replacedArgs)
		if err != nil {
			r.logger.WithFields(logger.Fields{
				"value": value,
				"error": err,
			}).Error("Command failed for replacement value")
			// Continue with other values even if one fails
		}
	}

	return nil
} // readReplacementValues reads lines from a file and returns them as replacement values
func (r *ReplacementCommandRunner) readReplacementValues(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filename, err)
	}
	defer file.Close()

	var values []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") { // Skip empty lines and comments
			values = append(values, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file %s: %w", filename, err)
	}

	return values, nil
}

// replaceInArgs replaces all instances of token with value in the args slice
// Automatically sanitizes values when they appear to be used in file paths
func (r *ReplacementCommandRunner) replaceInArgs(args []string, token, value string) []string {
	replaced := make([]string, len(args))
	for i, arg := range args {
		replacedArg := strings.ReplaceAll(arg, token, value)

		// Check if this argument looks like a file path that contains the replaced value
		if r.isLikelyFilePath(arg, token) {
			// Sanitize the value for use in file paths
			sanitizedValue := r.sanitizeForFilename(value)
			replacedArg = strings.ReplaceAll(arg, token, sanitizedValue)

			if sanitizedValue != value {
				r.logger.WithFields(logger.Fields{
					"original":  value,
					"sanitized": sanitizedValue,
					"argument":  arg,
				}).Info("Sanitized value for filename")
			}
		}

		replaced[i] = replacedArg
	}
	return replaced
}

// isLikelyFilePath determines if an argument containing a token is likely a file path
func (r *ReplacementCommandRunner) isLikelyFilePath(arg, token string) bool {
	// Check if the argument contains common file path indicators
	containsToken := strings.Contains(arg, token)
	if !containsToken {
		r.logger.WithFields(logger.Fields{
			"argument": arg,
			"token":    token,
		}).Debug("Argument doesn't contain token")
		return false
	}

	argLower := strings.ToLower(arg)

	// Skip if it looks like a URL parameter (starts with http/https protocol patterns)
	if strings.Contains(argLower, "://") {
		r.logger.WithFields(logger.Fields{
			"argument": arg,
		}).Debug("Argument contains protocol, not a file path")
		return false
	}

	// Look for file path indicators
	fileExtensions := []string{
		".txt", ".json", ".xml", ".csv", ".log", ".out", ".html", ".pdf",
	}

	filenameIndicators := []string{
		"output", "result", "scan", "report", "log", "file",
	}

	// Check for file extensions
	for _, ext := range fileExtensions {
		if strings.Contains(argLower, ext) {
			r.logger.WithFields(logger.Fields{
				"argument":  arg,
				"extension": ext,
			}).Debug("Argument contains file extension, likely file path")
			return true
		}
	}

	// Check for filename-related words
	for _, indicator := range filenameIndicators {
		if strings.Contains(argLower, indicator) {
			r.logger.WithFields(logger.Fields{
				"argument":  arg,
				"indicator": indicator,
			}).Debug("Argument contains filename indicator, likely file path")
			return true
		}
	}

	// Check for path separators (but not in URL context)
	if strings.Contains(arg, "/") || strings.Contains(arg, "\\") {
		// Additional check: make sure it's not a URL path
		if !strings.Contains(argLower, "http") && !strings.Contains(argLower, "fuzz") {
			r.logger.WithFields(logger.Fields{
				"argument": arg,
			}).Debug("Argument contains path separators, likely file path")
			return true
		}
	}

	r.logger.WithFields(logger.Fields{
		"argument": arg,
	}).Debug("Argument doesn't match any file path patterns")
	return false
}

// sanitizeForFilename converts a value (like a URL) into a safe filename component
func (r *ReplacementCommandRunner) sanitizeForFilename(value string) string {
	// Remove or replace characters that are invalid in filenames
	sanitized := value

	// Remove protocols
	protocolRegex := regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*://`)
	sanitized = protocolRegex.ReplaceAllString(sanitized, "")

	// Replace invalid filename characters with underscores
	// Including query parameters and fragment identifiers
	invalidChars := regexp.MustCompile(`[<>:"/\\|?*=&#]`)
	sanitized = invalidChars.ReplaceAllString(sanitized, "_")

	// Replace multiple consecutive underscores with single underscore
	multipleUnderscores := regexp.MustCompile(`_+`)
	sanitized = multipleUnderscores.ReplaceAllString(sanitized, "_")

	// Remove leading/trailing underscores and dots
	sanitized = strings.Trim(sanitized, "_.")

	// Handle special cases
	if sanitized == "" {
		sanitized = "sanitized_value"
	}

	// Limit length to prevent very long filenames
	maxLength := 100
	if len(sanitized) > maxLength {
		sanitized = sanitized[:maxLength]
		sanitized = strings.TrimRight(sanitized, "_.")
	}

	return sanitized
}
