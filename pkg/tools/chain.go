package tools

import (
	"context"
	"fmt"
	"runtime"
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

	results := make(chan string, len(tools))
	// Launch all tools concurrently
	for _, tool := range tools {
		wg.Add(1)
		go func(t Tool) {
			defer wg.Done()
			select {
			case <-ctx.Done():
				errChan <- toolError{tool: t.Name(), err: ctx.Err()}
				return
			default:
				// continue
			}

			if err := t.Run(ctx, options); err != nil {
				errChan <- toolError{tool: t.Name(), err: err}
				return
			}

			select {
			case <-ctx.Done():
				// If ctx is canceled after tool finished, don’t report success
				errChan <- toolError{tool: t.Name(), err: ctx.Err()}
				return
			case results <- fmt.Sprintf("tool %s completed successfully", t.Name()):
				// normal success path
			}

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

type HybridStrategy struct{}

func (hybrid *HybridStrategy) Run(ctx context.Context, tools []Tool, options *Options) error {
	log.Info("Executing tools in hybrid (DAG-based)")

	// Build and validate the graph
	g, err := newDepGraph(tools)
	if err != nil {
		return err
	}
	if err := g.validate(); err != nil {
		return err
	}

	tracker := newStageTracker(tools)

	// Worker pool
	workers := runtime.NumCPU()
	if workers < 1 {
		workers = 1
	}
	log.Infof("Hybrid DAG workers: %d", workers)

	ready := make(chan Tool, len(tools))
	results := make(chan runResult, len(tools))
	errs := make([]toolError, 0)
	var wg sync.WaitGroup

	// Seed initial ready tools
	for _, t := range g.initialReady() {
		log.Debugf("Initial ready: %s", t.Name())
		ready <- t
	}

	// Workers
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case t, ok := <-ready:
					if !ok {
						return
					}
					runErr := t.Run(ctx, options)
					results <- runResult{name: t.Name(), err: runErr}
				}
			}
		}()
	}

	// Scheduler loop
	doneCount := 0
	total := len(tools)

	for doneCount < total {
		select {
		case <-ctx.Done():
			close(ready)
			wg.Wait()
			return ctx.Err()

		case r := <-results:
			// Update accounting for the completed tool
			doneCount++
			success := (r.err == nil)
			if !success {
				errs = append(errs, toolError{tool: r.name, err: r.err})
				log.Errorf("Tool %s failed: %v", r.name, r.err)
			} else {
				log.Infof("Tool %s completed successfully", r.name)
			}

			completedStage := tracker.markCompleted(r.name)
			if completedStage != "" {
				log.Infof("Stage %s completed. Triggering hooks...", completedStage)
				for _, hook := range GetHooksForStage(completedStage) {
					wg.Add(1)
					h := hook
					go func() {
						defer wg.Done()
						hookCtx := HookContext{
							ctx:       ctx,
							OutputDir: ".",
							ToolName:  r.name,
							Options:   options,
						}
						if err := h.PostHook(hookCtx); err != nil {
							log.Errorf("Hook %s failed: %v", h.Name(), err)
						}
					}()
				}
				go func() {
					wg.Wait()
					log.Infof("All hooks for stage %s completed", completedStage)
				}()
			}

			newReady, skipped := g.onComplete(r.name, success)
			for _, s := range skipped {
				doneCount++
				errs = append(errs, toolError{tool: s, err: fmt.Errorf("skipped due to failed dependency")})
				log.Warnf("Tool %s skipped (failed dependency)", s)
			}
			for _, t := range newReady {
				ready <- t
			}

		}
	}

	go func() {
		wg.Wait()
		close(ready)
		close(results)
	}()

	if len(errs) > 0 {
		log.Errorf("Hybrid DAG execution completed with %d error(s)", len(errs))
		for _, e := range errs {
			log.Errorf("  %s: %v", e.tool, e.err)
		}
		return fmt.Errorf("%d tool(s) failed", len(errs))
	}

	log.Infof("All %d tools completed successfully", total)
	return nil
}

type CommandRunner interface {
	Run(ctx context.Context, command string, args []string) error
}

type toolError struct {
	tool string
	err  error
}

type runResult struct {
	name string
	err  error
}
