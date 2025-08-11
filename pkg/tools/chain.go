package tools

import (
	"context"
	"fmt"
	"sync"

	log "github.com/sirupsen/logrus"
)

type ExecutionStrategy interface {
	Run(ctx context.Context, tools []Tool, options *Options) error
}

type SequentialStrategy struct{}

func (s *SequentialStrategy) Run(ctx context.Context, tools []Tool, options *Options) error {
	log.Info("Executing tools sequentially")

	successCount := 0

	for _, tool := range tools {
		err := tool.Run(ctx, options)
		if err != nil {
			return fmt.Errorf("failed to run tool %s", tool.Name())
		}
		log.Infof("Tool %s completed successfully\n", tool.Name())
		successCount++
	}

	log.Infof("All %d tools completed successfully", successCount)
	return nil
}

type ConcurrentStrategy struct{}

func (s *ConcurrentStrategy) Run(ctx context.Context, tools []Tool, options *Options) error {
	log.Info("Executing tools concurrently")

	var wg sync.WaitGroup
	// Create channels for results
	errChan := make(chan toolError, len(tools))
	// collect tool name

	results := make(chan string, len(tools))
	// Launch all tools concurrently
	for _, tool := range tools {
		wg.Add(1)
		go func(t Tool) {
			defer wg.Done()
			if err := t.Run(ctx, options); err != nil {
				errChan <- toolError{tool: t.Name(), err: err}
			}
			results <- t.Name()
		}(tool)
	}

	go func() {
		wg.Wait()
		close(errChan)
		close(results)
	}()

	successCount := 0
	var errors []toolError

	for errChan != nil || results != nil {
		select {
		case err, ok := <-errChan:
			if !ok {
				errChan = nil
			} else {
				errors = append(errors, err)
			}
		case toolName, ok := <-results:
			if !ok {
				results = nil
			} else {
				successCount++
				log.Infof("Tool %s completed successfully\n", toolName)
			}
		}
	}

	if len(errors) > 0 {
		log.Errorf("Concurrent execution completed with %d error(s)", len(errors))
		for _, e := range errors {
			log.Errorf("  %s: %v", e.tool, e.err)
		}
		return fmt.Errorf("%d tool(s) failed", len(errors))
	}

	log.Infof("All %d tools completed successfully", successCount)

	return nil
}

type CommandRunner interface {
	Run(ctx context.Context, command string, args []string) error
}

type toolError struct {
	tool string
	err  error
}
