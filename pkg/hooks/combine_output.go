package hooks

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"pipeliner/pkg/tools"
	"strings"
)

type CombineOutput struct{}

func (c *CombineOutput) Name() string {
	return "combine_output"
}

func (c *CombineOutput) PostHook(ctx tools.HookContext) error {
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
