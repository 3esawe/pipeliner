package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"pipeliner/pkg/logger"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var utilsLogger = logger.NewLogger(logrus.InfoLevel)

type ConfigOptions struct {
	ConfigPath  string
	ConfigName  string
	ConfigType  string
	EnvPrefix   string
	DefaultsMap map[string]interface{}
}

var projectConfigPath, _ = filepath.Abs("./config")

var projectRoot, _ = filepath.Abs(".")

func NewViperConfig(scanType string) (*viper.Viper, error) {

	return NewViperConfigWithOptions(ConfigOptions{
		ConfigPath: projectConfigPath,
		ConfigName: scanType,
		ConfigType: "yaml",
		EnvPrefix:  "PIPELINER",
	})
}

func NewViperConfigWithOptions(opts ConfigOptions) (*viper.Viper, error) {

	v := viper.New()
	v.SetConfigType(opts.ConfigType)

	configPaths := []string{opts.ConfigPath}
	if opts.ConfigPath != "./config" {
		configPaths = append(configPaths, "./config")
	}
	configPaths = append(configPaths, "/etc/pipeliner", "$HOME/.pipeliner")

	for _, path := range configPaths {
		v.AddConfigPath(path)
	}

	v.SetConfigName(opts.ConfigName)

	if opts.EnvPrefix != "" {
		v.SetEnvPrefix(opts.EnvPrefix)
		v.AutomaticEnv()
		v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	}

	for key, value := range opts.DefaultsMap {
		v.SetDefault(key, value)
	}

	utilsLogger.Infof("Searching for config file: %s in paths: %v", opts.ConfigName, configPaths)

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return nil, fmt.Errorf("config file '%s' not found in paths: %v", opts.ConfigName, configPaths)
		}
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	utilsLogger.Infof("Loaded config file: %s", v.ConfigFileUsed())
	return v, nil
}

type ScanDirectoryOptions struct {
	BaseDir     string
	ScanType    string
	DomainName  string
	Timestamp   time.Time
	Permissions os.FileMode
}

func CreateScanDirectory(scanType, domainName string) (string, error) {
	return CreateScanDirectoryWithOptions(ScanDirectoryOptions{
		BaseDir:     filepath.Join(projectRoot, "scans"),
		ScanType:    scanType,
		DomainName:  domainName,
		Timestamp:   time.Now(),
		Permissions: 0755,
	})
}

func CreateScanDirectoryWithOptions(opts ScanDirectoryOptions) (string, error) {
	safeDomainName := sanitizeForFilesystem(opts.DomainName)

	dirName := fmt.Sprintf("%s_%s_%s",
		opts.ScanType,
		safeDomainName,
		opts.Timestamp.Format("2006-01-02_15-04-05"))

	dir := filepath.Join(opts.BaseDir, dirName)

	if err := os.MkdirAll(dir, opts.Permissions); err != nil {
		utilsLogger.Errorf("Error creating scan directory: %v", err)
		return "", fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		utilsLogger.Errorf("Error getting absolute path: %v", err)
		return dir, fmt.Errorf("failed to get absolute path for %s: %w", dir, err)
	}

	utilsLogger.Infof("Created scan directory: %s", absDir)
	return absDir, nil
}

func sanitizeForFilesystem(input string) string {
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
		" ", "_",
	)

	sanitized := replacer.Replace(input)

	sanitized = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 { // Control characters
			return -1 // Remove character
		}
		return r
	}, sanitized)

	if sanitized == "" {
		sanitized = "unknown"
	}

	if len(sanitized) > 100 {
		sanitized = sanitized[:100]
	}

	return sanitized
}

func ValidateConfig(v *viper.Viper) error {
	requiredFields := []string{"execution_mode", "tools"}
	for _, field := range requiredFields {
		if !v.IsSet(field) {
			return fmt.Errorf("required configuration field '%s' is missing", field)
		}
	}

	executionMode := v.GetString("execution_mode")
	validModes := map[string]bool{
		"sequential": true,
		"concurrent": true,
		"hybrid":     true,
	}

	if !validModes[executionMode] {
		return fmt.Errorf("invalid execution_mode '%s', must be one of: sequential, concurrent, hybrid", executionMode)
	}

	tools := v.Get("tools")
	if tools == nil {
		return fmt.Errorf("tools configuration is required")
	}

	if toolSlice, ok := tools.([]interface{}); ok {
		if len(toolSlice) == 0 {
			return fmt.Errorf("at least one tool must be configured")
		}
	} else {
		return fmt.Errorf("tools configuration must be a list")
	}

	return nil
}

func GetConfigPath() string {
	if path := os.Getenv("PIPELINER_CONFIG_PATH"); path != "" {
		return path
	}
	return "./config"
}

func EnsureDirectoryExists(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return os.MkdirAll(path, 0755)
	}
	return nil
}
