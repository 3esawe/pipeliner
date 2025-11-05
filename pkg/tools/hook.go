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
	OtherData  map[string]interface{}
}

type PostHook interface {
	Name() string
	Description() string
	Execute(ctx HookContext) error
}

type StageHook interface {
	Name() string
	Description() string
	ExecuteForStage(ctx HookContext) error
}

type Hook interface {
	Name() string
	Description() string
	PostHook(ctx HookContext) error
}

type PostHookInfo struct {
	Name        string
	Description string
	Hook        PostHook
}

type StageHookInfo struct {
	Name        string
	Description string
	Hook        StageHook
}

var (
	postHookRegistry   = make(map[string]*PostHookInfo)
	legacyHookRegistry = make(map[string]*PostHookInfo)
	hookLogger         = logger.NewLogger(logrus.InfoLevel)
)

func RegisterPostHook(name string, hook PostHook) {
	if _, exists := postHookRegistry[name]; exists {
		hookLogger.WithFields(logger.Fields{"hook": name}).Warn("PostHook already registered, overwriting")
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

func GetPostHook(name string) PostHook {
	if hookInfo, exists := postHookRegistry[name]; exists {
		return hookInfo.Hook
	}
	if hookInfo, exists := legacyHookRegistry[name]; exists {
		return hookInfo.Hook
	}
	return nil
}

func RegisterHook(name string, hook Hook) {
	if _, exists := legacyHookRegistry[name]; exists {
		hookLogger.WithFields(logger.Fields{"hook": name}).Warn("Legacy hook already registered, overwriting")
	}

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

func GetHook(name string) Hook {
	if hookInfo, exists := legacyHookRegistry[name]; exists {
		if wrapper, ok := hookInfo.Hook.(*legacyHookWrapper); ok {
			return wrapper.hook
		}
	}
	return nil
}

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

func ListAvailableHooks() []PostHookInfo {
	allHooks := make([]PostHookInfo, 0, len(postHookRegistry)+len(legacyHookRegistry))

	for _, hookInfo := range postHookRegistry {
		allHooks = append(allHooks, *hookInfo)
	}

	for _, hookInfo := range legacyHookRegistry {
		allHooks = append(allHooks, *hookInfo)
	}

	return allHooks
}
