package tools

import (
	"fmt"
	"reflect"
)

type Options struct {
	ScanType string
	Domain   string
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
				args = append(args, flag.Flag, flag.Default) // this  is to handle flags that are just flags without options
			}
			continue
		}

		fieldValue := optionsValue.FieldByName(flag.Option)
		if !fieldValue.IsValid() {
			if flag.Default != "" {
				args = append(args, flag.Flag, flag.Default)
				continue
			} else {
				return nil, fmt.Errorf("field '%s' not found in options", flag.Option)
			}
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
