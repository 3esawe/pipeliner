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

type ReplacementCommandRunner struct {
	baseRunner tools.CommandRunner
	logger     *logger.Logger
}

func NewReplacementCommandRunner(baseRunner tools.CommandRunner) *ReplacementCommandRunner {
	return &ReplacementCommandRunner{
		baseRunner: baseRunner,
		logger:     logger.NewLogger(logrus.InfoLevel),
	}
}

func (r *ReplacementCommandRunner) Run(ctx context.Context, command string, args []string) error {
	r.logger.WithFields(logger.Fields{
		"command": command,
		"args":    args,
	}).Info("Running command")
	return r.baseRunner.Run(ctx, command, args)
}

func (r *ReplacementCommandRunner) RunWithReplacement(ctx context.Context, command string, args []string, replaceToken, replaceFromFile string) error {
	r.logger.WithFields(logger.Fields{
		"command":          command,
		"args":             args,
		"token":            replaceToken,
		"replacement_file": replaceFromFile,
	}).Info("Running replacement command")

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
		}
	}

	return nil
}

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
		if line != "" && !strings.HasPrefix(line, "#") {
			values = append(values, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file %s: %w", filename, err)
	}

	return values, nil
}

func (r *ReplacementCommandRunner) replaceInArgs(args []string, token, value string) []string {
	replaced := make([]string, len(args))
	for i, arg := range args {
		replacedArg := strings.ReplaceAll(arg, token, value)

		if r.isLikelyFilePath(arg, token) {
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

func (r *ReplacementCommandRunner) isLikelyFilePath(arg, token string) bool {
	containsToken := strings.Contains(arg, token)
	if !containsToken {
		return false
	}

	argLower := strings.ToLower(arg)

	if strings.Contains(argLower, "://") {
		return false
	}

	fileExtensions := []string{
		".txt", ".json", ".xml", ".csv", ".log", ".out", ".html", ".pdf",
	}

	filenameIndicators := []string{
		"output", "result", "scan", "report", "log", "file",
	}

	for _, ext := range fileExtensions {
		if strings.Contains(argLower, ext) {
			return true
		}
	}

	for _, indicator := range filenameIndicators {
		if strings.Contains(argLower, indicator) {
			return true
		}
	}

	if strings.Contains(arg, "/") || strings.Contains(arg, "\\") {
		if !strings.Contains(argLower, "http") && !strings.Contains(argLower, "fuzz") {
			return true
		}
	}

	return false
}

func (r *ReplacementCommandRunner) sanitizeForFilename(value string) string {
	sanitized := value

	protocolRegex := regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*://`)
	sanitized = protocolRegex.ReplaceAllString(sanitized, "")

	invalidChars := regexp.MustCompile(`[<>:"/\\|?*=&#]`)
	sanitized = invalidChars.ReplaceAllString(sanitized, "_")

	multipleUnderscores := regexp.MustCompile(`_+`)
	sanitized = multipleUnderscores.ReplaceAllString(sanitized, "_")

	sanitized = strings.Trim(sanitized, "_.")

	if sanitized == "" {
		sanitized = "sanitized_value"
	}

	maxLength := 100
	if len(sanitized) > maxLength {
		sanitized = sanitized[:maxLength]
		sanitized = strings.TrimRight(sanitized, "_.")
	}

	return sanitized
}
