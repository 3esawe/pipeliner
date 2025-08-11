// pkg/engine/engine.go
package engine

import (
	"context"
	"fmt"
	"pipeliner/internal/utils"
	"pipeliner/pkg/tools"
	toolpackage "pipeliner/pkg/tools"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type Engine struct {
	ctx     context.Context
	options *tools.Options
	config  *viper.Viper
	runner  *CommandRunner
}

func NewEngine(ctx context.Context) *Engine {
	log.Info("Initializing Pipeliner Engine...")
	return &Engine{
		ctx:    ctx,
		runner: &CommandRunner{},
	}
}

func (e *Engine) SetOptions(options *tools.Options) {
	e.options = options
	if e.options.ScanType != "" {
		e.config = utils.NewViperConfig(e.options.ScanType)
		utils.CreateAndChangeScanDirectory(e.options.ScanType, e.options.Domain)
	}
}

func (e *Engine) Run() error {
	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()

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
			log.Info("Running Pipeliner Engine...")
			if err := e.runTools(); err != nil {
				log.Errorf("Pipeline Engine stopped due to error %v or context being cancelled", err)
				return fmt.Errorf("tool execution failed")
			}
		}
	}

}

func (e *Engine) runTools() error {
	chainConfig := tools.ChainConfig{
		ExecutionMode: e.config.GetString("execution_mode"),
	}
	if err := e.unmarshalConfig(&chainConfig); err != nil {
		log.Errorf("failed to parse tool chain config: %w", err)
		return fmt.Errorf("failed to parse tool chain config: %w", err)
	}

	log.Infof("Loaded %d tools from config", len(chainConfig.Tools))

	toolInstances, err := e.createToolInstances(chainConfig.Tools)
	if err != nil {
		log.Errorf("failed to create tool instances: %w", err)
		return fmt.Errorf("failed to create tool instances: %w", err)
	}

	var strategy tools.ExecutionStrategy
	switch chainConfig.ExecutionMode {
	case "concurrent":
		log.Info("Using concurrent execution strategy")
		strategy = &tools.ConcurrentStrategy{}
	default:
		log.Info("Using sequential execution strategy")
		strategy = &tools.SequentialStrategy{}
	}

	if err := strategy.Run(e.ctx, toolInstances, e.options); err != nil {
		log.Error(err)
		return err
	}

	return nil
}

func (e *Engine) unmarshalConfig(chainConfig *tools.ChainConfig) error {
	if err := e.config.Unmarshal(chainConfig); err != nil {
		log.Errorf("failed to parse tool chain config: %v", err)
		return fmt.Errorf("failed to parse tool chain config: %v", err)
	}
	return nil
}

func (e *Engine) createToolInstances(toolConfigs []tools.ToolConfig) ([]tools.Tool, error) {
	var toolInstances []tools.Tool
	for _, toolConfig := range toolConfigs {
		if toolConfig.Command == "" {
			log.Errorf("tool command not set for %s", toolConfig.Name)
			return nil, fmt.Errorf("tool command not set for %s", toolConfig.Name)
		}
		tool := tools.NewConfigurableTool(toolConfig.Name, toolConfig, e.runner)
		toolInstances = append(toolInstances, tool)
	}
	return toolInstances, nil
}

func (e *Engine) GetOptions() *toolpackage.Options {
	return e.options
}
