# Summary of Changes: URL/Value Replacement Feature Implementation

## Overview
This document summarizes all changes made to implement the extensible URL/Value replacement feature for the Pipeliner tool. This feature allows tools to iterate over dependency output files and execute commands for each value individually.

## Problem Statement
The original request was to add extensibility for tools that don't accept file input but need string/URL replacement (e.g., `ffuf` needing to execute once per URL from `httpx` output). The initial implementation had a brittle static configuration that would break if customers changed YAML filenames.

## Solution Architecture

### 1. Interface Consolidation
**Problem**: Multiple similar interfaces existed across packages causing confusion:
- `CommandRunner` in `pkg/runner/command.go` (exec.Cmd-based)
- `CommandRunner` in `pkg/tools/chain.go` (string + args-based)  
- `ToolsCommandRunner` in `pkg/runner/replacement.go` (duplicate)

**Solution**: Consolidated to a single, clean interface hierarchy:
```go
// pkg/tools/interfaces.go
type CommandRunner interface {
    Run(ctx context.Context, command string, args []string) error
}

type ReplacementCommandRunner interface {
    CommandRunner  // Embedded for Liskov Substitution Principle
    RunWithReplacement(ctx context.Context, command string, args []string, replaceToken, replaceFromFile string) error
}
```

**Why embedding CommandRunner in ReplacementCommandRunner:**
- **Liskov Substitution Principle**: Any `ReplacementCommandRunner` can be used as a `CommandRunner`
- **Backward Compatibility**: Existing tools continue to work without modification
- **Progressive Enhancement**: Tools automatically use replacement when configured, fall back to normal execution otherwise

### 2. Dynamic Configuration Resolution
**Problem**: Static hardcoded filename mappings that break with customer YAML changes.

**Solution**: Implemented dynamic configuration resolution system:

#### ToolRegistry System
```go
// pkg/tools/registry.go
type ToolRegistry interface {
    GetToolConfig(name string) (*ToolConfig, bool)
    GetAllToolConfigs() map[string]*ToolConfig
}

type SimpleToolRegistry struct {
    configs map[string]*ToolConfig
    mutex   sync.RWMutex
}
```

#### Dynamic Output File Detection
```go
// pkg/tools/tool.go
func (t *ConfigurableTool) extractOutputFileFromConfig(config *ToolConfig) string {
    // Look for output flags in the tool configuration
    for _, flag := range config.Flags {
        if t.isOutputFlag(flag) {
            return flag.Default
        }
    }
    // Fallback to default pattern
    return fmt.Sprintf("%s_output.txt", config.Name)
}
```

### 3. Configuration Enhancement
**Changes to ToolConfig:**
```go
// pkg/tools/config.go
type ToolConfig struct {
    // ... existing fields
    Replace     string `yaml:"replace,omitempty"`           // Token to replace (e.g., "{{URL}}")
    ReplaceFrom string `yaml:"replace_from,omitempty"`      // Optional source file override
}
```

### 4. Replacement Implementation
**New ReplacementCommandRunner:**
```go
// pkg/runner/replacement.go
type ReplacementCommandRunner struct {
    baseRunner BaseCommandRunner
}

func (r *ReplacementCommandRunner) RunWithReplacement(
    ctx context.Context, 
    command string, 
    args []string, 
    replaceToken, 
    replaceFromFile string
) error {
    // Read file, replace tokens, execute for each line
}
```

### 5. Tool Enhancement
**Enhanced ConfigurableTool:**
```go
// pkg/tools/tool.go
func (t *ConfigurableTool) Run(ctx context.Context, options *Options) error {
    // ... build args
    if t.config.Replace != "" {
        err = t.runWithReplacement(ctx, args, options)  // Use replacement logic
    } else {
        err = t.runner.Run(ctx, t.config.Command, args) // Normal execution
    }
}
```

### 6. Engine Integration
**Updated engine to provide registry:**
```go
// pkg/engine/engine.go
func (e *PiplinerEngine) createToolInstances(toolConfigs []tools.ToolConfig) ([]tools.Tool, error) {
    // Create registry and populate with all tool configurations
    registry := tools.NewSimpleToolRegistry()
    for _, toolConfig := range toolConfigs {
        registry.RegisterTool(toolConfig)
    }
    
    // Create tools with registry access
    for _, toolConfig := range toolConfigs {
        tool := tools.NewConfigurableToolWithRegistry(
            toolConfig.Name, toolConfig.Type, toolConfig, e.runner, registry
        )
        toolInstances = append(toolInstances, tool)
    }
}
```

## Files Created/Modified

### New Files
1. `pkg/runner/replacement.go` - Replacement command runner implementation
2. `pkg/runner/replacement_test.go` - Unit tests for replacement functionality
3. `pkg/tools/registry.go` - Tool configuration registry implementation
4. `pkg/tools/replacement_integration_test.go` - Integration tests
5. `config/replacement_demo.yaml` - Example configuration showcasing the feature
6. `docs/REPLACEMENT_FEATURE.md` - Feature documentation

### Modified Files
1. `pkg/tools/config.go` - Added `Replace` and `ReplaceFrom` fields
2. `pkg/tools/interfaces.go` - Consolidated and cleaned interfaces
3. `pkg/tools/tool.go` - Enhanced with replacement logic and registry support
4. `pkg/tools/chain.go` - Removed duplicate interface definitions
5. `pkg/engine/engine.go` - Integrated registry system
6. `config/full_recon.yaml` - Updated ffuf configuration to demonstrate feature

## Configuration Examples

### Before (Brittle):
```yaml
- name: ffuf
  command: ffuf
  depends_on: ["httpx"]
  flags:
    - flag: "-u"
      default: "{{URL}}/FUZZ"  # Would break - no replacement logic
```

### After (Extensible):
```yaml
- name: ffuf
  command: ffuf
  replace: "{{URL}}"           # Token to replace
  replace_from: "httpx_results.txt"  # Optional: specify source file
  depends_on: ["httpxbb"]
  flags:
    - flag: "-u"
      default: "{{URL}}/FUZZ"   # Token gets replaced with each URL
```

## Key Benefits

1. **Customer-Controlled**: No hardcoded assumptions about filenames
2. **Backward Compatible**: Existing configurations continue to work
3. **Extensible**: Easy to add new replacement patterns
4. **Dynamic**: Automatically adapts to configuration changes
5. **Clean Architecture**: Single responsibility, clear interfaces
6. **Well-Tested**: Comprehensive unit and integration tests

## Migration Path

- **Existing Tools**: No changes required, continue working as before
- **New Replacement Tools**: Add `replace` field to enable feature
- **Custom Patterns**: Use `replace_from` to specify non-standard output files

## Testing

- Unit tests verify replacement logic works correctly
- Integration tests verify dynamic configuration resolution
- All existing tests continue to pass (backward compatibility verified)

This implementation successfully addresses the original brittleness issue while providing a clean, extensible foundation for future enhancements.
