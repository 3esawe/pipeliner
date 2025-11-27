package tools

import (
	"context"
	"fmt"
	"pipeliner/pkg/errors"
	"pipeliner/pkg/logger"
	"runtime"
	"sync"

	"github.com/sirupsen/logrus"
)

var chainLogger = logger.NewLogger(logrus.InfoLevel)

func getOutputDir(options *Options) string {
	if options != nil && options.WorkingDir != "" {
		return options.WorkingDir
	}
	return "."
}

func executePostHooks(ctx context.Context, toolName string, hookNames []string, options *Options) error {
	if len(hookNames) == 0 {
		return nil
	}

	if options.Logger != nil {
		options.Logger.Info("Executing post hooks for tool", logger.Fields{
			"hook_count": len(hookNames),
			"tool_name":  toolName,
		})
	} else {
		chainLogger.Infof("Executing %d post hooks for tool %s", len(hookNames), toolName)
	}

	for _, hookName := range hookNames {
		postHook := GetPostHook(hookName)
		if postHook == nil {
			legacyHook := GetHook(hookName)
			if legacyHook == nil {
				if options.Logger != nil {
					options.Logger.Warn("Post hook not found for tool", logger.Fields{
						"hook_name": hookName,
						"tool_name": toolName,
					})
				} else {
					chainLogger.Warnf("Post hook %s not found for tool %s", hookName, toolName)
				}
				continue
			}

			hookCtx := HookContext{
				ctx:       ctx,
				OutputDir: getOutputDir(options),
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
					chainLogger.Errorf("Post hook %s failed for tool %s: %v", hookName, toolName, err)
				}
				return errors.NewToolError(toolName, fmt.Errorf("post hook %s failed: %w", hookName, err))
			}
		} else {
			hookCtx := HookContext{
				ctx:       ctx,
				OutputDir: getOutputDir(options),
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
					chainLogger.Errorf("Post hook %s failed for tool %s: %v", hookName, toolName, err)
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
			chainLogger.Infof("Post hook %s completed successfully for tool %s", hookName, toolName)
		}
	}

	return nil
}

func executeStageHooks(ctx context.Context, stage Stage, stageName string, options *Options) error {
	hooks := GetStageHooks(stage)
	if len(hooks) == 0 {
		return nil
	}

	chainLogger.Infof("Executing %d stage hooks for stage %s", len(hooks), stageName)

	var wg sync.WaitGroup
	errChan := make(chan error, len(hooks))

	for _, hook := range hooks {
		wg.Add(1)
		go func(h StageHook) {
			defer wg.Done()
			hookCtx := HookContext{
				ctx:       ctx,
				OutputDir: getOutputDir(options),
				ToolName:  stageName,
				Options:   options,
			}
			if err := h.ExecuteForStage(hookCtx); err != nil {
				chainLogger.Errorf("Stage hook %s failed for stage %s: %v", h.Name(), stageName, err)
				errChan <- fmt.Errorf("stage hook %s failed for stage %s: %w", h.Name(), stageName, err)
			} else {
				chainLogger.Infof("Stage hook %s completed successfully for stage %s", h.Name(), stageName)
			}
		}(hook)
	}

	wg.Wait()
	close(errChan)

	// Check if any stage hooks failed
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	chainLogger.Infof("All stage hooks for stage %s completed successfully", stageName)
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

type ToolError struct {
	Tool string
	Err  error
}

type PartialExecutionError struct {
	FailedTools []ToolError
	Message     string
}

func (e *PartialExecutionError) Error() string {
	return e.Message
}

func NewPartialExecutionError(failedTools []ToolError) *PartialExecutionError {
	return &PartialExecutionError{
		FailedTools: failedTools,
		Message:     fmt.Sprintf("%d tool(s) failed", len(failedTools)),
	}
}

type SequentialStrategy struct{}

func (s *SequentialStrategy) Run(ctx context.Context, tools []Tool, options *Options) error {
	chainLogger.Info("Executing tools sequentially")

	tracker := newStageTracker(tools)
	successCount := 0
	var failedTools []ToolError

	for _, tool := range tools {
		err := tool.Run(ctx, options)
		if err != nil {
			chainLogger.Errorf("Tool %s failed: %v", tool.Name(), err)
			failedTools = append(failedTools, ToolError{Tool: tool.Name(), Err: err})
			continue
		}

		if err := executePostHooks(ctx, tool.Name(), tool.PostHooks(), options); err != nil {
			chainLogger.Errorf("Post hooks failed for tool %s: %v", tool.Name(), err)
			failedTools = append(failedTools, ToolError{Tool: tool.Name(), Err: fmt.Errorf("post hooks failed: %w", err)})
			continue
		}

		completedStage := tracker.markCompleted(tool.Name())
		if completedStage != "" {
			chainLogger.Infof("Stage %s completed. Triggering stage hooks...", completedStage)
			if err := executeStageHooks(ctx, completedStage, string(completedStage), options); err != nil {
				chainLogger.Errorf("Stage hooks failed for stage %s: %v", completedStage, err)
			}
		}

		successCount++
	}

	if len(failedTools) > 0 {
		chainLogger.Warnf("%d tool(s) failed, but %d completed successfully", len(failedTools), successCount)
		return NewPartialExecutionError(failedTools)
	}

	chainLogger.Infof("All %d tools completed successfully", successCount)
	return nil
}

type ConcurrentStrategy struct{}

func (s *ConcurrentStrategy) Run(ctx context.Context, tools []Tool, options *Options) error {
	chainLogger.Info("Executing tools concurrently")

	tracker := newStageTracker(tools)
	var wg sync.WaitGroup
	// Create channels for results
	errChan := make(chan ToolError, len(tools))
	completedTools := make(chan Tool, len(tools))

	for _, tool := range tools {
		wg.Add(1)
		go func(t Tool) {
			defer wg.Done()
			select {
			case <-ctx.Done():
				errChan <- ToolError{Tool: t.Name(), Err: ctx.Err()}
				return
			default:
			}

			if err := t.Run(ctx, options); err != nil {
				errChan <- ToolError{Tool: t.Name(), Err: err}
				return
			}

			select {
			case <-ctx.Done():
				errChan <- ToolError{Tool: t.Name(), Err: ctx.Err()}
				return
			case completedTools <- t:
			}
		}(tool)
	}

	go func() {
		wg.Wait()
		close(errChan)
		close(completedTools)
	}()

	successCount := 0
	var errors []ToolError
	var completedList []Tool

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
				chainLogger.Infof("Tool %s completed successfully", tool.Name())
			}
		}
	}

	for _, tool := range completedList {
		if err := executePostHooks(ctx, tool.Name(), tool.PostHooks(), options); err != nil {
			chainLogger.Errorf("Post hooks failed for tool %s: %v", tool.Name(), err)
			errors = append(errors, ToolError{Tool: tool.Name(), Err: fmt.Errorf("post hooks failed: %w", err)})
		} else {
			completedStage := tracker.markCompleted(tool.Name())
			if completedStage != "" {
				chainLogger.Infof("Stage %s completed. Triggering stage hooks...", completedStage)
				if err := executeStageHooks(ctx, completedStage, string(completedStage), options); err != nil {
					chainLogger.Errorf("Stage hooks failed for stage %s: %v", completedStage, err)
				}
			}
		}
	}

	if len(errors) > 0 {
		chainLogger.Warnf("Concurrent execution completed with %d error(s), but %d succeeded", len(errors), successCount)
		return NewPartialExecutionError(errors)
	}

	chainLogger.Infof("All %d tools completed successfully", successCount)
	return nil
}

type HybridStrategy struct{}

func (hybrid *HybridStrategy) Run(ctx context.Context, tools []Tool, options *Options) error {
	chainLogger.Info("Executing tools in hybrid (DAG-based)")

	// Build and validate the graph
	g, err := newDepGraph(tools)
	if err != nil {
		return err
	}
	if err := g.validate(); err != nil {
		return err
	}

	tracker := newStageTracker(tools)

	workers := runtime.NumCPU()
	if workers < 1 {
		workers = 1
	}
	chainLogger.Infof("Hybrid DAG workers: %d", workers)

	ready := make(chan Tool, len(tools))
	results := make(chan runResult, len(tools))
	errs := make([]ToolError, 0)
	var wg sync.WaitGroup

	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	for _, t := range g.initialReady() {
		chainLogger.Infof("Initial ready: %s", t.Name())
		ready <- t
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			chainLogger.Infof("Worker %d started", workerID)

			for {
				select {
				case <-workerCtx.Done():
					chainLogger.Infof("Worker %d stopping due to context cancellation", workerID)
					return
				case t, ok := <-ready:
					if !ok {
						chainLogger.Infof("Worker %d stopping - ready channel closed", workerID)
						return
					}

					chainLogger.Infof("Worker %d executing tool %s", workerID, t.Name())
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
			doneCount++
			success := (r.err == nil)
			if !success {
				errs = append(errs, ToolError{Tool: r.name, Err: r.err})
				chainLogger.Errorf("Tool %s failed: %v", r.name, r.err)
			} else {
				chainLogger.Infof("Tool %s completed successfully", r.name)

				if tool := findToolByName(tools, r.name); tool != nil {
					if err := executePostHooks(ctx, tool.Name(), tool.PostHooks(), options); err != nil {
						chainLogger.Errorf("Post hooks failed for tool %s: %v", tool.Name(), err)
						errs = append(errs, ToolError{Tool: r.name, Err: err})
						success = false
					}
				}
			}

			completedStage := tracker.markCompleted(r.name)
			if completedStage != "" {
				chainLogger.Infof("Stage %s completed. Triggering stage hooks...", completedStage)
				if err := executeStageHooks(ctx, completedStage, string(completedStage), options); err != nil {
					chainLogger.Errorf("Stage hooks failed for stage %s: %v", completedStage, err)
				}
			}

			newReady, skipped := g.onComplete(r.name, success)
			for _, s := range skipped {
				doneCount++
				errs = append(errs, ToolError{Tool: s, Err: fmt.Errorf("skipped due to failed dependency")})
				chainLogger.Warnf("Tool %s skipped (failed dependency)", s)
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
		chainLogger.Warnf("Hybrid DAG execution completed with %d error(s), but %d succeeded", len(errs), total-len(errs))
		for _, e := range errs {
			chainLogger.Errorf("  %s: %v", e.Tool, e.Err)
		}
		return NewPartialExecutionError(errs)
	}

	chainLogger.Infof("All %d tools completed successfully", total)
	return nil
}

type runResult struct {
	name string
	err  error
}
