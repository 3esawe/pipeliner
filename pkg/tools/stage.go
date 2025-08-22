package tools

import (
	"sync"
)

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
	completed      map[string]bool // toolName -> finished
	stageTools     map[Stage][]string
	stageCompleted map[Stage]bool // track reported stages
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
			continue // already reported
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

var stageHooks = make(map[Stage][]Hook)

func RegisterHookForStage(stage Stage, hook Hook) {
	stageHooks[stage] = append(stageHooks[stage], hook)
}

func GetHooksForStage(stage Stage) []Hook {
	return stageHooks[stage]
}
