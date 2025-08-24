package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// ConfigOptions holds configuration loading options
type ConfigOptions struct {
	ConfigPath  string
	ConfigName  string
	ConfigType  string
	EnvPrefix   string
	DefaultsMap map[string]interface{}
}

// NewViperConfig creates a new Viper configuration with better error handling and validation
func NewViperConfig(scanType string) (*viper.Viper, error) {
	return NewViperConfigWithOptions(ConfigOptions{
		ConfigPath: "./config",
		ConfigName: scanType,
		ConfigType: "yaml",
		EnvPrefix:  "PIPELINER",
	})
}

// NewViperConfigWithOptions creates a Viper configuration with custom options
func NewViperConfigWithOptions(opts ConfigOptions) (*viper.Viper, error) {
	v := viper.New()

	// Set configuration type and search paths
	v.SetConfigType(opts.ConfigType)

	// Add multiple search paths for flexibility
	configPaths := []string{opts.ConfigPath}
	if opts.ConfigPath != "./config" {
		configPaths = append(configPaths, "./config")
	}
	configPaths = append(configPaths, "/etc/pipeliner", "$HOME/.pipeliner")

	for _, path := range configPaths {
		v.AddConfigPath(path)
	}

	v.SetConfigName(opts.ConfigName)

	// Enable environment variable support
	if opts.EnvPrefix != "" {
		v.SetEnvPrefix(opts.EnvPrefix)
		v.AutomaticEnv()
		v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	}

	// Set defaults if provided
	for key, value := range opts.DefaultsMap {
		v.SetDefault(key, value)
	}

	log.Infof("Searching for config file: %s in paths: %v", opts.ConfigName, configPaths)

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return nil, fmt.Errorf("config file '%s' not found in paths: %v", opts.ConfigName, configPaths)
		}
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	log.Infof("Loaded config file: %s", v.ConfigFileUsed())
	return v, nil
}

// ScanDirectoryOptions holds options for creating scan directories
type ScanDirectoryOptions struct {
	BaseDir     string
	ScanType    string
	DomainName  string
	Timestamp   time.Time
	Permissions os.FileMode
}

// CreateAndChangeScanDirectory creates a timestamped scan directory and changes to it
func CreateAndChangeScanDirectory(scanType, domainName string) (string, error) {
	return CreateAndChangeScanDirectoryWithOptions(ScanDirectoryOptions{
		BaseDir:     "./scans",
		ScanType:    scanType,
		DomainName:  domainName,
		Timestamp:   time.Now(),
		Permissions: 0755,
	})
}

// CreateAndChangeScanDirectoryWithOptions creates a scan directory with custom options
func CreateAndChangeScanDirectoryWithOptions(opts ScanDirectoryOptions) (string, error) {
	// Sanitize domain name for filesystem
	safeDomainName := sanitizeForFilesystem(opts.DomainName)

	// Create directory path
	dirName := fmt.Sprintf("%s_%s_%s",
		opts.ScanType,
		safeDomainName,
		opts.Timestamp.Format("2006-01-02_15-04-05"))

	dir := filepath.Join(opts.BaseDir, dirName)

	// Create the directory with proper permissions
	if err := os.MkdirAll(dir, opts.Permissions); err != nil {
		log.Errorf("Error creating scan directory: %v", err)
		return "", fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Get absolute path before changing directory
	absDir, err := filepath.Abs(dir)
	if err != nil {
		log.Errorf("Error getting absolute path: %v", err)
		return dir, fmt.Errorf("failed to get absolute path for %s: %w", dir, err)
	}

	// Change to the new directory
	if err := os.Chdir(absDir); err != nil {
		log.Errorf("Error changing to scan directory: %v", err)
		return absDir, fmt.Errorf("failed to change directory to %s: %w", absDir, err)
	}

	log.Infof("Created and changed to scan directory: %s", absDir)
	return absDir, nil
}

// sanitizeForFilesystem removes or replaces characters that are invalid in filenames
func sanitizeForFilesystem(input string) string {
	// Replace invalid characters with underscores
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

	// Remove any remaining problematic characters
	sanitized = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 { // Control characters
			return -1 // Remove character
		}
		return r
	}, sanitized)

	// Ensure it's not empty and not too long
	if sanitized == "" {
		sanitized = "unknown"
	}

	// Limit length to avoid filesystem issues
	if len(sanitized) > 100 {
		sanitized = sanitized[:100]
	}

	return sanitized
}

// ValidateConfig validates common configuration fields
func ValidateConfig(v *viper.Viper) error {
	// Check required fields
	requiredFields := []string{"execution_mode", "tools"}
	for _, field := range requiredFields {
		if !v.IsSet(field) {
			return fmt.Errorf("required configuration field '%s' is missing", field)
		}
	}

	// Validate execution mode
	executionMode := v.GetString("execution_mode")
	validModes := map[string]bool{
		"sequential": true,
		"concurrent": true,
		"hybrid":     true,
	}

	if !validModes[executionMode] {
		return fmt.Errorf("invalid execution_mode '%s', must be one of: sequential, concurrent, hybrid", executionMode)
	}

	// Validate tools configuration
	tools := v.Get("tools")
	if tools == nil {
		return fmt.Errorf("tools configuration is required")
	}

	// Check if tools is a slice
	if toolSlice, ok := tools.([]interface{}); ok {
		if len(toolSlice) == 0 {
			return fmt.Errorf("at least one tool must be configured")
		}
	} else {
		return fmt.Errorf("tools configuration must be a list")
	}

	return nil
}

// GetConfigPath returns the path where config files are expected to be found
func GetConfigPath() string {
	if path := os.Getenv("PIPELINER_CONFIG_PATH"); path != "" {
		return path
	}
	return "./config"
}

// EnsureDirectoryExists creates a directory if it doesn't exist
func EnsureDirectoryExists(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return os.MkdirAll(path, 0755)
	}
	return nil
}
