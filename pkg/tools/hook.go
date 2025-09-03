package tools

import (
	"context"
	"pipeliner/pkg/logger"

	"github.com/sirupsen/logrus"
)

type HookContext struct {
	ctx        context.Context
	OutputDir  string
	ToolName   string
	ToolConfig ToolConfig
	Options    *Options
	OtherData  map[string]interface{} // for extensibility
}

// PostHook interface for user-defined hooks that run after individual tools
// These can be defined in YAML configurations under "posthooks"
type PostHook interface {
	Name() string
	Description() string
	Execute(ctx HookContext) error
}

// StageHook interface for system-controlled hooks that run when all tools in a stage complete
// These are registered in code and run automatically
type StageHook interface {
	Name() string
	Description() string
	ExecuteForStage(ctx HookContext) error
}

// Hook interface for backward compatibility - implements both PostHook and StageHook
// TODO: Deprecated - use PostHook or StageHook interfaces directly
type Hook interface {
	Name() string
	Description() string
	PostHook(ctx HookContext) error
}

// HookInfo contains metadata about a registered post hook
type PostHookInfo struct {
	Name        string
	Description string
	Hook        PostHook
}

// StageHookInfo contains metadata about a registered stage hook
type StageHookInfo struct {
	Name        string
	Description string
	Hook        StageHook
}

var postHookRegistry = make(map[string]*PostHookInfo)
var legacyHookRegistry = make(map[string]*PostHookInfo) // For backward compatibility
var hookLogger = logger.NewLogger(logrus.InfoLevel)

// RegisterPostHook registers a user-defined hook that can be used in YAML configurations
func RegisterPostHook(name string, hook PostHook) {
	if _, exists := postHookRegistry[name]; exists {
		hookLogger.WithFields(logger.Fields{
			"hook": name,
		}).Warn("PostHook already registered, overwriting")
	}
	postHookRegistry[name] = &PostHookInfo{
		Name:        name,
		Description: hook.Description(),
		Hook:        hook,
	}
	hookLogger.WithFields(logger.Fields{
		"hook":        name,
		"description": hook.Description(),
	}).Info("Registered post hook")
}

// GetPostHook retrieves a registered post hook by name
func GetPostHook(name string) PostHook {
	if hookInfo, exists := postHookRegistry[name]; exists {
		return hookInfo.Hook
	}
	// Check legacy registry for backward compatibility
	if hookInfo, exists := legacyHookRegistry[name]; exists {
		return hookInfo.Hook
	}
	return nil
}

// RegisterHook registers a legacy hook for backward compatibility
// Deprecated: Use RegisterPostHook for new implementations
func RegisterHook(name string, hook Hook) {
	if _, exists := legacyHookRegistry[name]; exists {
		hookLogger.WithFields(logger.Fields{
			"hook": name,
		}).Warn("Legacy hook already registered, overwriting")
	}

	// Wrap legacy hook to implement PostHook interface
	wrapper := &legacyHookWrapper{hook: hook}
	legacyHookRegistry[name] = &PostHookInfo{
		Name:        name,
		Description: hook.Description(),
		Hook:        wrapper,
	}
	hookLogger.WithFields(logger.Fields{
		"hook":        name,
		"description": hook.Description(),
	}).Info("Registered legacy hook")
}

// GetHook retrieves a legacy hook for backward compatibility
// Deprecated: Use GetPostHook for new implementations
func GetHook(name string) Hook {
	if hookInfo, exists := legacyHookRegistry[name]; exists {
		if wrapper, ok := hookInfo.Hook.(*legacyHookWrapper); ok {
			return wrapper.hook
		}
	}
	return nil
}

// legacyHookWrapper wraps legacy Hook interface to implement PostHook interface
type legacyHookWrapper struct {
	hook Hook
}

func (w *legacyHookWrapper) Name() string {
	return w.hook.Name()
}

func (w *legacyHookWrapper) Description() string {
	return w.hook.Description()
}

func (w *legacyHookWrapper) Execute(ctx HookContext) error {
	return w.hook.PostHook(ctx)
}

// ListAvailableHooks returns a list of all registered hooks with their descriptions
func ListAvailableHooks() []PostHookInfo {
	allHooks := make([]PostHookInfo, 0, len(postHookRegistry)+len(legacyHookRegistry))

	// Add post hooks
	for _, hookInfo := range postHookRegistry {
		allHooks = append(allHooks, *hookInfo)
	}

	// Add legacy hooks
	for _, hookInfo := range legacyHookRegistry {
		allHooks = append(allHooks, *hookInfo)
	}

	return allHooks
}
