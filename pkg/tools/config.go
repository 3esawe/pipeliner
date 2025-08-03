package tools

import (
	"fmt"
	"reflect"
)

type FlagConfig struct {
	Flag         string `yaml:"flag"`
	Option       string `yaml:"option"`
	Required     bool   `yaml:"required"`
	Default      string `yaml:"default"`
	IsBoolean    bool   `yaml:"is_boolean"`
	IsPositional bool   `yaml:"is_positional"`
}

type ToolConfig struct {
	Name        string       `yaml:"name"`
	Description string       `yaml:"description"`
	Command     string       `yaml:"command"`
	Flags       []FlagConfig `yaml:"flags"`
}

type ChainConfig struct {
	ExecutionMode string       `yaml:"execution_mode"`
	Tools         []ToolConfig `yaml:"tools"`
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
				args = append(args, flag.Flag)
			}
			continue
		}

		fieldValue := optionsValue.FieldByName(flag.Option)
		if !fieldValue.IsValid() {
			return nil, fmt.Errorf("field '%s' not found in options", flag.Option)
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
