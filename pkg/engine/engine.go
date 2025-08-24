// pkg/engine/engine.go
package engine

import (
	"context"
	"fmt"
	"pipeliner/internal/notification"
	"pipeliner/internal/utils"
	output "pipeliner/pkg/io_utils"
	"pipeliner/pkg/tools"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type EnginePiplinerOpts struct {
	ctx      context.Context
	options  *tools.Options
	config   *viper.Viper
	runner   tools.CommandRunner
	periodic int
	notifier *notification.NotificationClient
}

type OptFunc func(*EnginePiplinerOpts)

type PiplinerEngine struct {
	EnginePiplinerOpts
}

func NewPiplinerEngine(ctx context.Context, opts ...OptFunc) *PiplinerEngine {
	log.Info("Initializing Pipeliner Engine...")
	o := EnginePiplinerOpts{
		ctx:      ctx,
		options:  &tools.Options{},
		runner:   &SimpleRunner{},
		periodic: 5, // in hours
	}

	for _, opt := range opts {
		opt(&o)
	}

	return &PiplinerEngine{
		EnginePiplinerOpts: o,
	}
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

func (e *PiplinerEngine) PrepareScan(options *tools.Options) error {
	e.options = options
	if e.options.ScanType != "" {
		var err error
		e.config, err = utils.NewViperConfig(e.options.ScanType)
		if err != nil {
			log.Errorf("Failed to load config: %v", err)
			return fmt.Errorf("failed to load config: %w", err)
		}
		err = utils.ValidateConfig(e.config)
		if err != nil {
			log.Errorf("Failed to validate config: %v", err)
			return fmt.Errorf("failed to validate config: %w", err)
		}

		_, err = utils.CreateAndChangeScanDirectory(e.options.ScanType, e.options.Domain)
		if err != nil {
			log.Errorf("Failed to create scan directory: %v", err)
			return fmt.Errorf("failed to create scan directory: %w", err)
		}

		go output.WatchDirectory(e.ctx)
	}
	return nil
}

func (e *PiplinerEngine) Run() error {
	ticker := time.NewTicker(time.Hour * time.Duration(e.periodic))
	defer ticker.Stop()

	log.Info("Running Pipeliner Engine...")
	if err := e.runTools(); err != nil {
		log.Errorf("Initial tool run failed: %v", err)
		return fmt.Errorf("tool execution failed")
	}

	for {
		select {
		case <-e.ctx.Done():
			log.Info("Stopping Pipeliner Engine...")
			return nil
		case <-ticker.C:
			log.Info("Running Periodic Pipeliner")
			if err := e.runTools(); err != nil {
				log.Errorf("Pipeline Engine stopped due to error %v or context being cancelled", err)
				return fmt.Errorf("tool execution failed")
			}
		}
	}

}

func (e *PiplinerEngine) runTools() error {
	chainConfig := tools.ChainConfig{
		ExecutionMode: e.config.GetString("execution_mode"),
	}
	if err := e.unmarshalConfig(&chainConfig); err != nil {
		log.Errorf("failed to parse tool chain config: %v", err)
		return fmt.Errorf("failed to parse tool chain config: %w", err)
	}

	log.Infof("Loaded %d tools from config", len(chainConfig.Tools))

	toolInstances, err := e.createToolInstances(chainConfig.Tools)
	if err != nil {
		log.Errorf("failed to create tool instances: %v", err)
		return fmt.Errorf("failed to create tool instances: %w", err)
	}

	var strategy tools.ExecutionStrategy
	switch chainConfig.ExecutionMode {
	case "concurrent":
		log.Info("Using concurrent execution strategy")
		strategy = &tools.ConcurrentStrategy{}
	case "hybrid":
		log.Info("Using hybrid execution strategy")
		strategy = &tools.HybridStrategy{}
	default:
		log.Info("Using sequential execution strategy")
		strategy = &tools.SequentialStrategy{}
	}

	if err := strategy.Run(e.ctx, toolInstances, e.options); err != nil {
		log.Error(err)
		return err
	}

	log.Info("Waiting for periodic scan to run")
	return nil
}

func (e *PiplinerEngine) unmarshalConfig(chainConfig *tools.ChainConfig) error {
	if err := e.config.Unmarshal(chainConfig); err != nil {
		log.Errorf("failed to parse tool chain config: %v", err)
		return fmt.Errorf("failed to parse tool chain config: %v", err)
	}
	return nil
}

func (e *PiplinerEngine) createToolInstances(toolConfigs []tools.ToolConfig) ([]tools.Tool, error) {
	var toolInstances []tools.Tool
	for _, toolConfig := range toolConfigs {
		if toolConfig.Command == "" {
			log.Errorf("tool command not set for %s", toolConfig.Name)
			return nil, fmt.Errorf("tool command not set for %s", toolConfig.Name)
		}
		tool := tools.NewConfigurableTool(toolConfig.Name, toolConfig.Type, toolConfig, e.runner)
		toolInstances = append(toolInstances, tool)
	}
	return toolInstances, nil
}

func (e *PiplinerEngine) GetOptions() *tools.Options {
	return e.options
}
