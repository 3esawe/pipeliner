package tools

import (
	"context"
	"fmt"
	"pipeliner/pkg/errors"
	"pipeliner/pkg/logger"
	"runtime"
	"sync"

	log "github.com/sirupsen/logrus"
)

// executePostHooks runs user-defined post hooks for an individual tool
// These hooks run after EACH tool completes (user-controlled via YAML)
func executePostHooks(ctx context.Context, toolName string, hookNames []string, options *Options) error {
	if len(hookNames) == 0 {
		return nil
	}

	// Use logger from options, fallback to direct logging if not available
	if options.Logger != nil {
		options.Logger.Info("Executing post hooks for tool", logger.Fields{
			"hook_count": len(hookNames),
			"tool_name":  toolName,
		})
	} else {
		log.Infof("Executing %d post hooks for tool %s", len(hookNames), toolName)
	}

	for _, hookName := range hookNames {
		postHook := GetPostHook(hookName)
		if postHook == nil {
			// Try legacy hook for backward compatibility
			legacyHook := GetHook(hookName)
			if legacyHook == nil {
				if options.Logger != nil {
					options.Logger.Warn("Post hook not found for tool", logger.Fields{
						"hook_name": hookName,
						"tool_name": toolName,
					})
				} else {
					log.Warnf("Post hook %s not found for tool %s", hookName, toolName)
				}
				continue
			}
			// Use legacy hook
			hookCtx := HookContext{
				ctx:       ctx,
				OutputDir: ".",
				ToolName:  toolName,
				Options:   options,
			}

			if err := legacyHook.PostHook(hookCtx); err != nil {
				if options.Logger != nil {
					options.Logger.Error("Post hook failed for tool", logger.Fields{
						"hook_name": hookName,
						"tool_name": toolName,
						"error":     err,
					})
				} else {
					log.Errorf("Post hook %s failed for tool %s: %v", hookName, toolName, err)
				}
				return errors.NewToolError(toolName, fmt.Errorf("post hook %s failed: %w", hookName, err))
			}
		} else {
			// Use new PostHook interface
			hookCtx := HookContext{
				ctx:       ctx,
				OutputDir: ".",
				ToolName:  toolName,
				Options:   options,
			}

			if err := postHook.Execute(hookCtx); err != nil {
				if options.Logger != nil {
					options.Logger.Error("Post hook failed for tool", logger.Fields{
						"hook_name": hookName,
						"tool_name": toolName,
						"error":     err,
					})
				} else {
					log.Errorf("Post hook %s failed for tool %s: %v", hookName, toolName, err)
				}
				return errors.NewToolError(toolName, fmt.Errorf("post hook %s failed: %w", hookName, err))
			}
		}

		if options.Logger != nil {
			options.Logger.Info("Post hook completed successfully for tool", logger.Fields{
				"hook_name": hookName,
				"tool_name": toolName,
			})
		} else {
			log.Infof("Post hook %s completed successfully for tool %s", hookName, toolName)
		}
	}

	return nil
}

// executeStageHooks runs system-defined hooks when ALL tools in a stage complete
// These hooks run once per stage completion (system-controlled)
func executeStageHooks(ctx context.Context, stage Stage, stageName string, options *Options) error {
	hooks := GetStageHooks(stage)
	if len(hooks) == 0 {
		return nil
	}

	log.Infof("Executing %d stage hooks for stage %s", len(hooks), stageName)

	// Run stage hooks concurrently but wait for all to complete
	var wg sync.WaitGroup
	errChan := make(chan error, len(hooks))

	for _, hook := range hooks {
		wg.Add(1)
		go func(h StageHook) {
			defer wg.Done()
			hookCtx := HookContext{
				ctx:       ctx,
				OutputDir: ".",
				ToolName:  stageName, // Use stage name as tool name for stage hooks
				Options:   options,
			}
			if err := h.ExecuteForStage(hookCtx); err != nil {
				log.Errorf("Stage hook %s failed for stage %s: %v", h.Name(), stageName, err)
				errChan <- fmt.Errorf("stage hook %s failed for stage %s: %w", h.Name(), stageName, err)
			} else {
				log.Infof("Stage hook %s completed successfully for stage %s", h.Name(), stageName)
			}
		}(hook)
	}

	// Wait for all stage hooks to complete
	wg.Wait()
	close(errChan)

	// Check if any stage hooks failed
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	log.Infof("All stage hooks for stage %s completed successfully", stageName)
	return nil
}

// findToolByName finds a tool by name in the tools slice
func findToolByName(tools []Tool, name string) Tool {
	for _, tool := range tools {
		if tool.Name() == name {
			return tool
		}
	}
	return nil
}

type ExecutionStrategy interface {
	Run(ctx context.Context, tools []Tool, options *Options) error
}

type SequentialStrategy struct{}

func (s *SequentialStrategy) Run(ctx context.Context, tools []Tool, options *Options) error {
	log.Info("Executing tools sequentially")

	tracker := newStageTracker(tools)
	successCount := 0

	for _, tool := range tools {
		err := tool.Run(ctx, options)
		if err != nil {
			return fmt.Errorf("failed to run tool %s: %w", tool.Name(), err)
		}

		// Execute post hooks for this tool
		if err := executePostHooks(ctx, tool.Name(), tool.PostHooks(), options); err != nil {
			return fmt.Errorf("post hooks failed for tool %s: %w", tool.Name(), err)
		}

		// Check if a stage completed
		completedStage := tracker.markCompleted(tool.Name())
		if completedStage != "" {
			log.Infof("Stage %s completed. Triggering stage hooks...", completedStage)

			// Execute system-controlled stage hooks
			if err := executeStageHooks(ctx, completedStage, string(completedStage), options); err != nil {
				log.Errorf("Stage hooks failed for stage %s: %v", completedStage, err)
				// Don't fail the entire pipeline for stage hook failures, just log
			}
		}

		successCount++
	}

	log.Infof("All %d tools completed successfully", successCount)
	return nil
}

type ConcurrentStrategy struct{}

func (s *ConcurrentStrategy) Run(ctx context.Context, tools []Tool, options *Options) error {
	log.Info("Executing tools concurrently")

	tracker := newStageTracker(tools)
	var wg sync.WaitGroup
	// Create channels for results
	errChan := make(chan toolError, len(tools))
	completedTools := make(chan Tool, len(tools))

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
				// If ctx is canceled after tool finished, don't report success
				errChan <- toolError{tool: t.Name(), err: ctx.Err()}
				return
			case completedTools <- t:
				// Tool completed successfully, will handle hooks later
			}

		}(tool)
	}

	go func() {
		wg.Wait()
		close(errChan)
		close(completedTools)
	}()

	successCount := 0
	var errors []toolError
	var completedList []Tool

	// Collect results
	for errChan != nil || completedTools != nil {
		select {
		case err, ok := <-errChan:
			if !ok {
				errChan = nil
			} else {
				errors = append(errors, err)
			}
		case tool, ok := <-completedTools:
			if !ok {
				completedTools = nil
			} else {
				completedList = append(completedList, tool)
				successCount++
				log.Infof("Tool %s completed successfully", tool.Name())
			}
		}
	}

	// If there were errors, don't proceed with hooks
	if len(errors) > 0 {
		log.Errorf("Concurrent execution completed with %d error(s)", len(errors))
		for _, e := range errors {
			log.Errorf("  %s: %v", e.tool, e.err)
		}
		return fmt.Errorf("%d tool(s) failed", len(errors))
	}

	// Execute post hooks for all completed tools sequentially
	for _, tool := range completedList {
		if err := executePostHooks(ctx, tool.Name(), tool.PostHooks(), options); err != nil {
			log.Errorf("Post hooks failed for tool %s: %v", tool.Name(), err)
			return fmt.Errorf("post hooks failed for tool %s: %w", tool.Name(), err)
		}

		// Check if a stage completed after this tool
		completedStage := tracker.markCompleted(tool.Name())
		if completedStage != "" {
			log.Infof("Stage %s completed. Triggering stage hooks...", completedStage)

			// Execute system-controlled stage hooks
			if err := executeStageHooks(ctx, completedStage, string(completedStage), options); err != nil {
				log.Errorf("Stage hooks failed for stage %s: %v", completedStage, err)
				// Don't fail the entire pipeline for stage hook failures, just log
			}
		}
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

	// Create a separate context for workers to ensure clean shutdown
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Seed initial ready tools
	for _, t := range g.initialReady() {
		log.Debugf("Initial ready: %s", t.Name())
		ready <- t
	}

	// Workers
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			log.Debugf("Worker %d started", workerID)

			for {
				select {
				case <-workerCtx.Done():
					log.Debugf("Worker %d stopping due to context cancellation", workerID)
					return
				case t, ok := <-ready:
					if !ok {
						log.Debugf("Worker %d stopping - ready channel closed", workerID)
						return
					}

					log.Debugf("Worker %d executing tool %s", workerID, t.Name())
					runErr := t.Run(workerCtx, options)

					select {
					case results <- runResult{name: t.Name(), err: runErr}:
					case <-workerCtx.Done():
						return
					}
				}
			}
		}(i)
	}

	// Scheduler loop
	doneCount := 0
	total := len(tools)

	defer func() {
		cancel() // Signal workers to stop
		close(ready)
		wg.Wait() // Wait for all workers to finish
	}()

	for doneCount < total {
		select {
		case <-ctx.Done():
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

				// Execute post hooks for this specific tool
				if tool := findToolByName(tools, r.name); tool != nil {
					if err := executePostHooks(ctx, tool.Name(), tool.PostHooks(), options); err != nil {
						log.Errorf("Post hooks failed for tool %s: %v", tool.Name(), err)
						// Mark as failed since post hooks failed
						errs = append(errs, toolError{tool: r.name, err: err})
						success = false
					}
				}
			}

			completedStage := tracker.markCompleted(r.name)
			if completedStage != "" {
				log.Infof("Stage %s completed. Triggering stage hooks...", completedStage)

				// Execute system-controlled stage hooks
				if err := executeStageHooks(ctx, completedStage, string(completedStage), options); err != nil {
					log.Errorf("Stage hooks failed for stage %s: %v", completedStage, err)
					// Don't fail the entire pipeline for stage hook failures, just log
				}
			}

			newReady, skipped := g.onComplete(r.name, success)
			for _, s := range skipped {
				doneCount++
				errs = append(errs, toolError{tool: s, err: fmt.Errorf("skipped due to failed dependency")})
				log.Warnf("Tool %s skipped (failed dependency)", s)
			}
			for _, t := range newReady {
				select {
				case ready <- t:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}
	}

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
