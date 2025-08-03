package tools

import (
	"context"
	"fmt"
)

type Tool interface {
	Name() string
	Run(ctx context.Context, options interface{}) error
}

type ToolData struct {
	Name        string
	Description string
	Config      map[string]interface{}
}

func (t *ToolData) GetName() string {
	return t.Name
}

func (t *ToolData) GetDescription() string {
	return t.Description
}

func (t *ToolData) GetConfig() string {
	return fmt.Sprintf("%v", t.Config)
}

func (t *ToolData) SetConfig(config map[string]interface{}) {
	t.Config = config
}
