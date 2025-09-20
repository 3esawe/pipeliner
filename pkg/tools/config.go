package tools

import (
	"fmt"
	"pipeliner/pkg/logger"
	"reflect"
	"time"
)

type Options struct {
	ScanType    string
	Domain      string
	Timeout     time.Duration
	WorkingDir  string
	Environment map[string]string
	DryRun      bool
	Logger      *logger.Logger
}

// DefaultOptions returns a new Options instance with sensible defaults
func DefaultOptions() *Options {
	return &Options{
		Timeout:     2 * time.Hour,
		WorkingDir:  ".",
		Environment: make(map[string]string),
		DryRun:      false,
		Logger:      nil,
	}
}

// Validate checks if the options are valid
func (o *Options) Validate() error {
	if o.ScanType == "" {
		return fmt.Errorf("scan type is required")
	}
	if o.Domain == "" {
		return fmt.Errorf("domain is required")
	}
	if o.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	return nil
}

type FlagConfig struct {
	Flag         string `yaml:"flag" mapstructure:"flag"`
	Option       string `yaml:"option" mapstructure:"option"`
	Required     bool   `yaml:"required" mapstructure:"required"`
	Default      string `yaml:"default" mapstructure:"default"`
	IsBoolean    bool   `yaml:"is_boolean" mapstructure:"is_boolean"`
	IsPositional bool   `yaml:"is_positional" mapstructure:"is_positional"`
}

type ToolConfig struct {
	Name        string        `yaml:"name"`
	Description string        `yaml:"description"`
	Type        string        `yaml:"type" mapstructure:"type"`
	Command     string        `yaml:"command"`
	Replace     string        `yaml:"replace,omitempty"`
	ReplaceFrom string        `yaml:"replace_from,omitempty" mapstructure:"replace_from"`
	Flags       []FlagConfig  `yaml:"flags"`
	DependsOn   []string      `yaml:"depends_on" mapstructure:"depends_on"`
	Timeout     time.Duration `yaml:"timeout,omitempty" mapstructure:"timeout"`
	Retries     int           `yaml:"retries,omitempty" mapstructure:"retries"`
	PostHooks   []string      `yaml:"posthooks,omitempty" mapstructure:"posthooks"`
}

// Validate checks if the tool configuration is valid
func (tc *ToolConfig) Validate() error {
	if tc.Name == "" {
		return fmt.Errorf("tool name is required")
	}
	if tc.Command == "" {
		return fmt.Errorf("tool command is required for %s", tc.Name)
	}
	if tc.Retries < 0 {
		return fmt.Errorf("retries must be non-negative for tool %s", tc.Name)
	}

	// Validate flag configurations
	for i, flag := range tc.Flags {
		if err := flag.Validate(); err != nil {
			return fmt.Errorf("invalid flag config at index %d for tool %s: %w", i, tc.Name, err)
		}
	}

	return nil
}

// Validate checks if the flag configuration is valid
func (fc *FlagConfig) Validate() error {
	if fc.Flag == "" && !fc.IsPositional {
		return fmt.Errorf("flag is required when not positional")
	}
	return nil
}

type ChainConfig struct {
	Description   string        `yaml:"description"`
	ExecutionMode string        `yaml:"execution_mode"`
	Tools         []ToolConfig  `yaml:"tools"`
	GlobalTimeout time.Duration `yaml:"global_timeout,omitempty" mapstructure:"global_timeout"`
}

// Validate checks if the chain configuration is valid
func (cc *ChainConfig) Validate() error {
	if len(cc.Tools) == 0 {
		return fmt.Errorf("at least one tool is required")
	}

	validModes := map[string]bool{
		"sequential": true,
		"concurrent": true,
		"hybrid":     true,
	}

	if !validModes[cc.ExecutionMode] {
		return fmt.Errorf("invalid execution mode: %s", cc.ExecutionMode)
	}

	// Validate each tool
	toolNames := make(map[string]bool)
	for i, tool := range cc.Tools {
		if err := tool.Validate(); err != nil {
			return fmt.Errorf("invalid tool config at index %d: %w", i, err)
		}

		// Check for duplicate tool names
		if toolNames[tool.Name] {
			return fmt.Errorf("duplicate tool name: %s", tool.Name)
		}
		toolNames[tool.Name] = true

		// Validate dependencies exist
		for _, dep := range tool.DependsOn {
			if !toolNames[dep] {
				// This is a forward dependency - we'll need to check it exists later
				// For now, we'll allow it and validate in a second pass
			}
		}
	}

	// Second pass: validate all dependencies exist
	for _, tool := range cc.Tools {
		for _, dep := range tool.DependsOn {
			if !toolNames[dep] {
				return fmt.Errorf("tool %s depends on unknown tool %s", tool.Name, dep)
			}
		}
	}

	return nil
}

func (tc *ToolConfig) BuildArgs(options interface{}) ([]string, error) {
	var args []string
	optionsValue := reflect.ValueOf(options)

	if optionsValue.Kind() == reflect.Ptr {
		optionsValue = optionsValue.Elem()
	}

	for _, flag := range tc.Flags {
		// Handle positional arguments
		if flag.IsPositional {
			args = append(args, flag.Flag)
			continue
		}

		// Skip flags with empty option names (pure flags)
		if flag.Option == "" {
			if flag.Flag != "" {
				if flag.Default != "" {
					args = append(args, flag.Flag, flag.Default)
				} else if flag.IsBoolean {
					args = append(args, flag.Flag)
				}
			}
			continue
		}

		fieldValue := optionsValue.FieldByName(flag.Option)
		if !fieldValue.IsValid() {
			if flag.Default != "" {
				args = append(args, flag.Flag, flag.Default)
				continue
			} else if flag.Required {
				return nil, fmt.Errorf("required field '%s' not found in options", flag.Option)
			}
			continue
		}

		value := fmt.Sprintf("%v", fieldValue.Interface())

		if flag.IsBoolean {
			if value == "true" {
				args = append(args, flag.Flag)
			}
			continue
		}

		if flag.Required && value == "" {
			return nil, fmt.Errorf("required option '%s' missing", flag.Option)
		}

		if value == "" && flag.Default != "" {
			value = flag.Default
		}

		if value != "" {
			args = append(args, flag.Flag, value)
		}
	}
	return args, nil
}
