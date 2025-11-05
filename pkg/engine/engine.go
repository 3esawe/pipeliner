package engine

import (
	"context"
	"fmt"
	"pipeliner/internal/notification"
	"pipeliner/internal/utils"
	"pipeliner/pkg/errors"
	output "pipeliner/pkg/io_utils"
	"pipeliner/pkg/logger"
	"pipeliner/pkg/runner"
	"pipeliner/pkg/tools"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type EnginePiplinerOpts struct {
	ctx      context.Context
	options  *tools.Options
	config   *viper.Viper
	runner   tools.CommandRunner
	periodic int
	notifier *notification.NotificationClient
	scanDir  string
	logger   *logger.Logger
}

type OptFunc func(*EnginePiplinerOpts)

type PiplinerEngine struct {
	EnginePiplinerOpts
}

func NewPiplinerEngine(optFuncs ...OptFunc) (*PiplinerEngine, error) {
	engineOpts := EnginePiplinerOpts{
		ctx: context.Background(),
	}

	for _, optFunc := range optFuncs {
		optFunc(&engineOpts)
	}

	if engineOpts.runner == nil {
		baseRunner := runner.NewSimpleRunner()
		engineOpts.runner = runner.NewReplacementCommandRunner(baseRunner)
	}

	if engineOpts.logger == nil {
		defaultLogger := logger.NewLogger(logrus.InfoLevel)
		engineOpts.logger = defaultLogger
	}

	return &PiplinerEngine{
		EnginePiplinerOpts: engineOpts,
	}, nil
}

func WithRunner(runnerFunc tools.CommandRunner) OptFunc {
	return func(opts *EnginePiplinerOpts) {
		opts.runner = runnerFunc
	}
}

func WithOptions(options *tools.Options) OptFunc {
	return func(opts *EnginePiplinerOpts) {
		opts.options = options
	}
}

func WithPeriodic(period int) OptFunc {
	return func(epo *EnginePiplinerOpts) {
		epo.periodic = period
	}
}

func WithNotificationClient(client *notification.NotificationClient) OptFunc {
	return func(opts *EnginePiplinerOpts) {
		opts.notifier = client
	}
}

func WithContext(ctx context.Context) OptFunc {
	return func(opts *EnginePiplinerOpts) {
		opts.ctx = ctx
	}
}

func (e *PiplinerEngine) PrepareScan(options *tools.Options) error {
	if options == nil {
		return fmt.Errorf("options cannot be nil")
	}
	e.options = options
	e.options.Logger = e.logger

	if e.options.ScanType != "" {
		var err error
		e.config, err = utils.NewViperConfig(e.options.ScanType)
		if err != nil {
			e.logger.Error("Failed to load config", logger.Fields{"error": err})
			return errors.ErrInvalidConfig
		}
		err = utils.ValidateConfig(e.config)
		if err != nil {
			e.logger.Error("Failed to validate config", logger.Fields{"error": err})
			return errors.ErrInvalidConfig
		}

		dir, err := utils.CreateScanDirectory(e.options.ScanType, e.options.Domain)
		if err != nil {
			e.logger.Error("Failed to create scan directory", logger.Fields{"error": err})
			return fmt.Errorf("failed to create scan directory: %w", err)
		}
		e.scanDir = dir
		e.options.WorkingDir = dir

		go output.WatchDirectoryWithPath(e.ctx, dir)
	}
	return nil
}

func (e *PiplinerEngine) RunHTTP(scanType, domain string) (err error) {
	if e.scanDir == "" {
		dir, err := utils.CreateScanDirectory(scanType, domain)
		if err != nil {
			e.logger.Error("Failed to create scan directory", logger.Fields{"error": err})
			return fmt.Errorf("failed to create scan directory: %w", err)
		}
		e.scanDir = dir

		if e.options == nil {
			e.options = tools.DefaultOptions()
		}
		e.options.WorkingDir = dir
	} else {
		if e.options != nil {
			e.options.WorkingDir = e.scanDir
		}
	}

	e.logger.Info("Starting HTTP scan", logger.Fields{"domain": domain, "module": scanType})
	if err := e.runTools(); err != nil {
		e.logger.Error("HTTP scan failed", logger.Fields{"error": err})
		return errors.ErrToolExecutionFailed
	}

	e.logger.Info("HTTP scan completed", logger.Fields{"domain": domain, "module": scanType})
	return nil
}

func (e *PiplinerEngine) Run() error {
	ticker := time.NewTicker(time.Hour * time.Duration(e.periodic))
	defer ticker.Stop()

	e.logger.Info("Starting Pipeliner Engine")
	if err := e.runTools(); err != nil {
		e.logger.Error("Initial tool run failed", logger.Fields{"error": err})
		return errors.ErrToolExecutionFailed
	}

	for {
		select {
		case <-e.ctx.Done():
			e.logger.Info("Stopping Pipeliner Engine")
			return nil
		case <-ticker.C:
			e.logger.Info("Running periodic pipeline")
			if err := e.runTools(); err != nil {
				e.logger.Error("Periodic pipeline failed", logger.Fields{"error": err})
				return errors.ErrToolExecutionFailed
			}
		}
	}

}

func (e *PiplinerEngine) runTools() error {
	chainConfig := tools.ChainConfig{
		ExecutionMode: e.config.GetString("execution_mode"),
	}
	if err := e.unmarshalConfig(&chainConfig); err != nil {
		e.logger.Error("Failed to parse tool chain config", logger.Fields{"error": err})
		return errors.ErrInvalidConfig
	}

	e.logger.Info("Loaded tools from config", logger.Fields{"tool_count": len(chainConfig.Tools)})

	toolInstances, err := e.createToolInstances(chainConfig.Tools)
	if err != nil {
		e.logger.Error("Failed to create tool instances", logger.Fields{"error": err})
		return err
	}

	var strategy tools.ExecutionStrategy
	switch chainConfig.ExecutionMode {
	case "concurrent":
		e.logger.Info("Using concurrent execution strategy")
		strategy = &tools.ConcurrentStrategy{}
	case "hybrid":
		e.logger.Info("Using hybrid execution strategy")
		strategy = &tools.HybridStrategy{}
	default:
		e.logger.Info("Using sequential execution strategy")
		strategy = &tools.SequentialStrategy{}
	}

	if err := strategy.Run(e.ctx, toolInstances, e.options); err != nil {
		e.logger.Error("Strategy execution failed", logger.Fields{"error": err})
		return err
	}

	return nil
}

func (e *PiplinerEngine) unmarshalConfig(chainConfig *tools.ChainConfig) error {
	if err := e.config.Unmarshal(chainConfig); err != nil {
		e.logger.Error("Failed to parse tool chain config", logger.Fields{"error": err})
		return errors.ErrInvalidConfig
	}
	return nil
}

func (e *PiplinerEngine) createToolInstances(toolConfigs []tools.ToolConfig) ([]tools.Tool, error) {
	var toolInstances []tools.Tool

	registry := tools.NewSimpleToolRegistry()
	for _, toolConfig := range toolConfigs {
		registry.RegisterTool(toolConfig)
	}

	for _, toolConfig := range toolConfigs {
		if toolConfig.Command == "" {
			e.logger.Error("Tool command not set", logger.Fields{"tool_name": toolConfig.Name})
			return nil, &errors.ConfigError{
				Field:   "command",
				Value:   toolConfig.Name,
				Message: "tool command cannot be empty",
			}
		}

		tool := tools.NewConfigurableToolWithRegistry(toolConfig.Name, toolConfig.Type, toolConfig, e.runner, registry)
		toolInstances = append(toolInstances, tool)
	}
	return toolInstances, nil
}

func (e *PiplinerEngine) GetOptions() *tools.Options {
	return e.options
}

func (e *PiplinerEngine) ScanDirectory() string {
	return e.scanDir
}
