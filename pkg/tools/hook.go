package tools

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
)

type HookContext struct {
	ctx        context.Context
	OutputDir  string
	ToolName   string
	ToolConfig ToolConfig
	Options    *Options
	OtherData  map[string]interface{} // for extensibility
}

type Hook interface {
	Name() string
	PostHook(ctx HookContext) error
}

var hookRegistry = make(map[string]Hook)

func RegisterHook(command string, hook Hook) {
	if _, exists := hookRegistry[command]; exists {
		log.Error(fmt.Sprintf("hook for command %s already registered", command))
	}
	hookRegistry[command] = hook
}

func GetHook(command string) Hook {
	return hookRegistry[command]
}
