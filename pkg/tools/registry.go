package tools

import "sync"

type SimpleToolRegistry struct {
	configs map[string]*ToolConfig
	mutex   sync.RWMutex
}

func NewSimpleToolRegistry() *SimpleToolRegistry {
	return &SimpleToolRegistry{
		configs: make(map[string]*ToolConfig),
	}
}

func (r *SimpleToolRegistry) RegisterTool(config ToolConfig) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	configCopy := config
	r.configs[config.Name] = &configCopy
}

func (r *SimpleToolRegistry) GetToolConfig(name string) (*ToolConfig, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	config, exists := r.configs[name]
	if !exists {
		return nil, false
	}

	configCopy := *config
	return &configCopy, true
}

func (r *SimpleToolRegistry) GetAllToolConfigs() map[string]*ToolConfig {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	result := make(map[string]*ToolConfig)
	for name, config := range r.configs {
		configCopy := *config
		result[name] = &configCopy
	}

	return result
}
