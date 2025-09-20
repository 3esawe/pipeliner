package scan

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"pipeliner/internal/notification"
	"pipeliner/pkg/engine"
	hooks "pipeliner/pkg/hooks"
	"pipeliner/pkg/logger"
	tools "pipeliner/pkg/tools"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// Config holds application configuration
type Config struct {
	Module        string
	Domain        string
	Verbose       bool
	ConfigPath    string
	Timeout       time.Duration
	PeriodicHours int
}

// App represents the main application
type App struct {
	config        *Config
	logger        *logger.Logger
	discordClient *notification.NotificationClient
}

// NewApp creates a new application instance
func NewApp(config *Config) (*App, error) {
	// Initialize logger
	logLevel := logrus.InfoLevel
	if config.Verbose {
		logLevel = logrus.DebugLevel
	}
	appLogger := logger.NewLogger(logLevel)

	// Initialize Discord client if configured
	var discordClient *notification.NotificationClient
	if token := os.Getenv("DISCORD_TOKEN"); token != "" {
		var err error
		discordClient, err = notification.NewNotificationClient()
		if err != nil {
			appLogger.WithError(err).Warn("Failed to initialize Discord client")
		} else {
			appLogger.Info("Discord notifications enabled")
		}
	} else {
		appLogger.Info("DISCORD_TOKEN not set - Discord notifications disabled")
	}

	return &App{
		config:        config,
		logger:        appLogger,
		discordClient: discordClient,
	}, nil
}

// Close cleans up application resources
func (a *App) Close() error {
	if a.discordClient != nil {
		return a.discordClient.Close()
	}
	return nil
}

// Run executes the scan command
func (a *App) Run(ctx context.Context) error {
	// Create engine with proper configuration
	engineInstance, err := engine.NewPiplinerEngine(
		engine.WithContext(ctx),
		engine.WithPeriodic(a.config.PeriodicHours),
		engine.WithNotificationClient(a.discordClient))
	if err != nil {
		return fmt.Errorf("failed to create pipeliner engine: %w", err)
	}

	// Prepare scan options with validation
	options := tools.DefaultOptions()
	options.ScanType = a.config.Module
	options.Domain = a.config.Domain
	options.Timeout = a.config.Timeout

	if err := options.Validate(); err != nil {
		return fmt.Errorf("invalid options: %w", err)
	}

	// Set options
	if err := engineInstance.PrepareScan(options); err != nil {
		return fmt.Errorf("failed to prepare scan: %w", err)
	}

	// Run engine in goroutine
	errChan := make(chan error, 1)
	go func() {
		defer close(errChan)
		errChan <- engineInstance.Run()
	}()

	// Wait for completion or cancellation
	select {
	case err := <-errChan:
		if err != nil {
			a.logger.WithError(err).Error("Engine execution failed")
			return err
		}
	case <-ctx.Done():
		a.logger.Info("Application context cancelled, waiting for engine to stop...")
		// Give engine some time to stop gracefully
		timeout := time.NewTimer(30 * time.Second)
		defer timeout.Stop()

		select {
		case err := <-errChan:
			if err != nil {
				a.logger.WithError(err).Error("Engine execution failed during shutdown")
				return err
			}
		case <-timeout.C:
			a.logger.Warn("Engine shutdown timed out")
			return fmt.Errorf("engine shutdown timed out")
		}
	}

	a.logger.Info("All tools finished execution")
	return nil
}

// getConfigDescription attempts to extract a description from a YAML config file
func getConfigDescription(configPath string) string {
	type ConfigMeta struct {
		Description string `yaml:"description,omitempty"`
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}

	var meta ConfigMeta
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return ""
	}

	return meta.Description
}

// NewScanCommand creates the scan command
func NewScanCommand() *cobra.Command {
	config := &Config{
		Timeout:       30 * time.Minute, // Default timeout
		PeriodicHours: 5,                // Default periodic interval
	}

	scanCmd := &cobra.Command{
		Use:   "scan",
		Short: "Scan using specified pipeline module",
		Long:  `Scan using the specified pipeline module configuration`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			// Create application instance
			app, err := NewApp(config)
			if err != nil {
				return fmt.Errorf("failed to initialize application: %w", err)
			}
			defer func() {
				if closeErr := app.Close(); closeErr != nil {
					app.logger.WithError(closeErr).Error("Error closing application")
				}
			}()

			// Setup graceful shutdown
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Handle signals
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				sig := <-sigChan
				app.logger.WithFields(logger.Fields{
					"signal": sig.String(),
				}).Info("Received shutdown signal")
				cancel()
			}()

			// Run the application
			return app.Run(ctx)
		},
	}

	// Setup scan command flags
	scanCmd.Flags().StringVarP(&config.Module, "module", "m", "", "Pipeline module to execute (required)")
	scanCmd.Flags().StringVarP(&config.Domain, "domain", "d", "", "Target domain for scanning")
	scanCmd.Flags().BoolVarP(&config.Verbose, "verbose", "v", false, "Enable verbose logging")
	scanCmd.Flags().StringVar(&config.ConfigPath, "config", "./config", "Configuration directory path")
	scanCmd.Flags().DurationVar(&config.Timeout, "timeout", 30*time.Minute, "Global timeout for operations")
	scanCmd.Flags().IntVar(&config.PeriodicHours, "periodic-hours", 5, "Hours between periodic scans")

	// Mark required flags
	scanCmd.MarkFlagRequired("module")

	return scanCmd
}

// NewListConfigsCommand creates the list-configs command
func NewListConfigsCommand() *cobra.Command {
	config := &Config{
		ConfigPath: "./config", // Default config path
	}

	listConfigsCmd := &cobra.Command{
		Use:   "list-configs",
		Short: "List available configuration files",
		Long:  `List all available configuration files and their descriptions`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			configPath := config.ConfigPath
			if configPath == "" {
				configPath = "./config"
			}

			files, err := os.ReadDir(configPath)
			if err != nil {
				return fmt.Errorf("failed to read config directory %s: %w", configPath, err)
			}

			fmt.Println("Available Configurations:")
			fmt.Println("========================")

			for _, file := range files {
				if !strings.HasSuffix(file.Name(), ".yaml") && !strings.HasSuffix(file.Name(), ".yml") {
					continue
				}

				configFile := filepath.Join(configPath, file.Name())
				description := getConfigDescription(configFile)

				fmt.Printf("\n• %s\n", strings.TrimSuffix(file.Name(), filepath.Ext(file.Name())))
				fmt.Printf("  File: %s\n", file.Name())
				if description != "" {
					fmt.Printf("  Description: %s\n", description)
				}
			}

			if len(files) == 0 {
				fmt.Printf("No configuration files found in %s\n", configPath)
			}

			return nil
		},
	}

	// Setup list-configs command flags
	listConfigsCmd.Flags().StringVar(&config.ConfigPath, "config", "./config", "Configuration directory path")

	return listConfigsCmd
}

// NewListHooksCommand creates the list-hooks command
func NewListHooksCommand() *cobra.Command {
	listHooksCmd := &cobra.Command{
		Use:   "list-hooks",
		Short: "List available hooks",
		Long:  `List all available hooks and their descriptions`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			// Initialize hooks first
			InitHooks()

			hooks := tools.ListAvailableHooks()

			fmt.Println("Available Hooks:")
			fmt.Println("===============")

			for _, hook := range hooks {
				fmt.Printf("\n• %s\n", hook.Name)
				if hook.Description != "" {
					fmt.Printf("  Description: %s\n", hook.Description)
				}
			}

			if len(hooks) == 0 {
				fmt.Println("No hooks available")
			}

			return nil
		},
	}

	return listHooksCmd
}

// InitHooks initializes all hooks
func InitHooks() {
	// Create hook instances
	combineOutput := hooks.NewCombineOutput()
	notifierHook := hooks.NewNotifierHook(hooks.NotifierHookConfig{
		Filename: "nuclei_output.txt",
	})

	// =====================================
	// STAGE HOOKS (System-controlled)
	// =====================================
	// These run automatically when ALL tools in a stage complete

	// CombineOutput: Combines all subdomain enumeration results into httpx_input.txt
	// Runs when ALL domain_enum tools complete
	tools.RegisterStageHook(tools.StageSubdomain, combineOutput)

	// NotifierHook: Sends notifications for vulnerability scan results
	// Runs when ALL vuln_scan tools complete

	// =====================================
	// POST HOOKS (User-controlled via YAML)
	// =====================================
	// These can be specified in YAML configurations under "posthooks"

	// NotifierHook: Can be used for individual tool notifications
	tools.RegisterPostHook("NotifierHook", notifierHook)

}
