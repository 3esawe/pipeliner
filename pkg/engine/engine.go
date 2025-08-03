package engine

import (
	"context"
	"fmt"
	"pipeliner/internal/utils"
	"pipeliner/pkg/runner"
	"pipeliner/pkg/tools"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type ProgressEvent struct {
	Tool      string
	Status    string // "started", "running", "completed", "failed"
	Progress  int    // 0-100 percentage
	Message   string
	Timestamp time.Time
}

type Engine struct {
	ctx      context.Context
	options  *Options
	config   *viper.Viper
	runner   runner.CommandRunner
	progress chan ProgressEvent
}

func NewEngine(ctx context.Context) *Engine {
	log.Info("Initializing Pipeliner Engine...")
	options := ParseOptions()
	config := utils.NewViperConfig(options.ScanType)
	utils.CreateAndChangeScanDirectory(options.ScanType, options.Domain)
	return &Engine{
		ctx:      ctx,
		options:  options,
		config:   config,
		runner:   &CommandRunner{},
		progress: make(chan ProgressEvent, 100), // Buffered channel for progress events
	}
}

func (e *Engine) Run() error {

	chainConfig := tools.ChainConfig{
		ExecutionMode: e.config.GetString("execution_mode"),
	}
	if err := e.config.Unmarshal(&chainConfig); err != nil {
		log.Errorf("failed to parse tool chain config: %v", err)
		return fmt.Errorf("failed to parse tool chain config: %v", err)
	}

	log.Infof("Loaded %d tools from config", len(chainConfig.Tools))

	toolInstances := make([]tools.Tool, 0, len(chainConfig.Tools))
	for _, toolConfig := range chainConfig.Tools {
		if toolConfig.Command == "" {
			log.Errorf("tool command not set for %s", toolConfig.Name)
			return fmt.Errorf("tool command not set for %s", toolConfig.Name)
		}
		tool := tools.NewConfigurableTool(toolConfig.Name, toolConfig, e.runner)
		toolInstances = append(toolInstances, tool)
	}
	var strategy ExecutionStrategy
	log.Infof("Execution mode: %s", chainConfig.ExecutionMode)
	switch chainConfig.ExecutionMode {
	case "concurrent":
		log.Info("Using concurrent execution strategy")
		strategy = &ConcurrentStrategy{}
	default:
		log.Info("Using sequential execution strategy")
		strategy = &SequentialStrategy{}
	}

	return strategy.Run(e.ctx, toolInstances, e.options)
}
