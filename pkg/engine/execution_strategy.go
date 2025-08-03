// pkg/engine/execution_strategy.go
package engine

import (
	"context"
	"fmt"
	toolpackage "pipeliner/pkg/tools"
	"sync"

	log "github.com/sirupsen/logrus"
)

type ExecutionStrategy interface {
	Run(ctx context.Context, tools []toolpackage.Tool, options interface{}) error
}

type SequentialStrategy struct{}

func (s *SequentialStrategy) Run(ctx context.Context, tools []toolpackage.Tool, options interface{}) error {
	for i, tool := range tools {
		log.Infof("Running tool %d/%d: %s", i+1, len(tools), tool.Name())
		if err := tool.Run(ctx, options); err != nil {
			log.Errorf("%s failed: %v", tool.Name(), err)
			return fmt.Errorf("%s failed: %w", tool.Name(), err)
		}
		log.Infof("%s completed successfully", tool.Name())
	}
	return nil
}

type ConcurrentStrategy struct{}

func (c *ConcurrentStrategy) Run(ctx context.Context, tools []toolpackage.Tool, options interface{}) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(tools))

	for i, tool := range tools {
		wg.Add(1)
		go func(i int, t toolpackage.Tool) {
			defer wg.Done()
			log.Infof("Starting tool %d/%d: %s", i+1, len(tools), t.Name())
			if err := t.Run(ctx, options); err != nil {
				log.Errorf("%s failed: %v", t.Name(), err)
				errChan <- fmt.Errorf("%s failed: %w", t.Name(), err)
				return
			}
			log.Infof("%s completed successfully", t.Name())
		}(i, tool)
	}

	wg.Wait()
	close(errChan)

	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("%d tools failed: %v", len(errors), errors)
	}
	return nil
}
