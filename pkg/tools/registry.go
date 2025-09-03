package tools

import "sync"

// SimpleToolRegistry implements ToolRegistry interface
type SimpleToolRegistry struct {
	configs map[string]*ToolConfig
	mutex   sync.RWMutex
}

// NewSimpleToolRegistry creates a new tool registry
func NewSimpleToolRegistry() *SimpleToolRegistry {
	return &SimpleToolRegistry{
		configs: make(map[string]*ToolConfig),
	}
}

// RegisterTool adds a tool configuration to the registry
func (r *SimpleToolRegistry) RegisterTool(config ToolConfig) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Make a copy to avoid external modifications
	configCopy := config
	r.configs[config.Name] = &configCopy
}

// GetToolConfig retrieves a tool configuration by name
func (r *SimpleToolRegistry) GetToolConfig(name string) (*ToolConfig, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	config, exists := r.configs[name]
	if !exists {
		return nil, false
	}

	// Return a copy to prevent external modifications
	configCopy := *config
	return &configCopy, true
}

// GetAllToolConfigs returns all registered tool configurations
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
