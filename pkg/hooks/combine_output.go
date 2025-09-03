package hooks

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"pipeliner/pkg/logger"
	"pipeliner/pkg/tools"
	"strings"

	"github.com/sirupsen/logrus"
)

type CombineOutput struct {
	logger *logger.Logger
}

// NewCombineOutput creates a new CombineOutput hook
func NewCombineOutput() *CombineOutput {
	return &CombineOutput{
		logger: logger.NewLogger(logrus.InfoLevel),
	}
}

func (c *CombineOutput) Name() string {
	return "combine_output"
}

func (c *CombineOutput) Description() string {
	return "Combines subdomain enumeration outputs from multiple tools into a single file (httpx_input.txt) for downstream processing"
}

// ExecuteForStage implements StageHook interface - runs when all domain enumeration tools complete
func (c *CombineOutput) ExecuteForStage(ctx tools.HookContext) error {
	outputFile, err := os.Create(filepath.Join(ctx.OutputDir, "httpx_input.txt"))
	if err != nil {
		return fmt.Errorf("failed to create httpx_input.txt: %w", err)
	}
	defer outputFile.Close()

	seenDomains := make(map[string]bool)

	err = filepath.Walk(ctx.OutputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasPrefix(info.Name(), "subdomain_") {
			inputFile, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("failed to open %s: %w", path, err)
			}
			defer inputFile.Close()

			scanner := bufio.NewScanner(inputFile)
			for scanner.Scan() {
				domain := strings.TrimSpace(scanner.Text())
				if domain == "" {
					continue
				}

				if !seenDomains[domain] {
					_, err := outputFile.WriteString(domain + "\n")
					if err != nil {
						return fmt.Errorf("failed to write to httpx_input.txt: %w", err)
					}
					seenDomains[domain] = true
				}
			}

			if err := scanner.Err(); err != nil {
				return fmt.Errorf("error scanning file %s: %w", path, err)
			}
		}

		return nil
	})

	return err
}

// PostHook implements legacy Hook interface for backward compatibility
// Deprecated: This hook should only be used as a StageHook, not as a PostHook
func (c *CombineOutput) PostHook(ctx tools.HookContext) error {
	return c.ExecuteForStage(ctx)
}
