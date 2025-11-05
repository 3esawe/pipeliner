package tools

import (
	"pipeliner/pkg/logger"
	"sync"

	"github.com/sirupsen/logrus"
)

var stageLogger = logger.NewLogger(logrus.InfoLevel)

type Stage string

const (
	StageSubdomain      Stage = "subdomain_enum"
	StageRecon          Stage = "recon"
	StageFingerPrinting Stage = "fingerprint"
	StageVuln           Stage = "vuln_scan"
)

func stageForToolType(toolType string) Stage {
	switch toolType {
	case "domain_enum":
		return StageSubdomain
	case "recon":
		return StageRecon
	case "fingerprint":
		return StageFingerPrinting
	case "vuln":
		return StageVuln
	default:
		return Stage("")
	}
}

type stageTracker struct {
	mu             sync.Mutex
	completed      map[string]bool
	stageTools     map[Stage][]string
	stageCompleted map[Stage]bool
}

func newStageTracker(tools []Tool) *stageTracker {
	st := &stageTracker{
		completed:      make(map[string]bool),
		stageTools:     make(map[Stage][]string),
		stageCompleted: make(map[Stage]bool),
	}
	for _, t := range tools {
		stage := stageForToolType(t.Type())
		if stage != "" {
			st.stageTools[stage] = append(st.stageTools[stage], t.Name())
		}
	}
	return st
}

func (st *stageTracker) markCompleted(toolName string) Stage {
	st.mu.Lock()
	defer st.mu.Unlock()

	st.completed[toolName] = true

	for stage, tools := range st.stageTools {
		if st.stageCompleted[stage] {
			continue
		}
		done := true
		for _, t := range tools {
			if !st.completed[t] {
				done = false
				break
			}
		}
		if done {
			st.stageCompleted[stage] = true
			return stage
		}
	}
	return ""
}

var stageHooks = make(map[Stage][]StageHook)

func RegisterStageHook(stage Stage, hook StageHook) {
	stageHooks[stage] = append(stageHooks[stage], hook)
	stageLogger.Infof("Registered stage hook: %s for stage %s", hook.Name(), stage)
}

func GetStageHooks(stage Stage) []StageHook {
	return stageHooks[stage]
}

func RegisterHookForStage(stage Stage, hook Hook) {
	wrapper := &legacyStageHookWrapper{hook: hook}
	RegisterStageHook(stage, wrapper)
}

func GetHooksForStage(stage Stage) []Hook {
	stageHooks := GetStageHooks(stage)
	legacyHooks := make([]Hook, 0, len(stageHooks))

	for _, stageHook := range stageHooks {
		if wrapper, ok := stageHook.(*legacyStageHookWrapper); ok {
			legacyHooks = append(legacyHooks, wrapper.hook)
		}
	}

	return legacyHooks
}

type legacyStageHookWrapper struct {
	hook Hook
}

func (w *legacyStageHookWrapper) Name() string {
	return w.hook.Name()
}

func (w *legacyStageHookWrapper) Description() string {
	return w.hook.Description()
}

func (w *legacyStageHookWrapper) ExecuteForStage(ctx HookContext) error {
	return w.hook.PostHook(ctx)
}
