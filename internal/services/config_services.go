package services

import (
	"os"
	"pipeliner/pkg/logger"
	"pipeliner/pkg/tools"
	"strings"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

type ConfigServiceMethods interface {
	GetScanModules() []tools.ChainConfig
}

type configService struct {
	log *logger.Logger
}

func NewConfigService() ConfigServiceMethods {
	return &configService{
		log: logger.NewLogger(logrus.Level(logrus.InfoLevel)),
	}
}

func (c *configService) GetScanModules() []tools.ChainConfig {

	configPath := "./config"

	files, err := os.ReadDir(configPath)
	if err != nil {
		c.log.WithError(err).Error("Failed to read config directory")
		return nil
	}

	toolConfig := make([]tools.ChainConfig, 0)

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".yaml") {
			continue
		}

		data, err := os.ReadFile(configPath + "/" + file.Name())
		if err != nil {
			c.log.WithError(err).WithField("file", file.Name()).Error("Failed to read config file")
			continue
		}

		var meta tools.ChainConfig
		if err := yaml.Unmarshal(data, &meta); err != nil {
			c.log.WithError(err).WithField("file", file.Name()).Error("Failed to parse config file")
			continue
		}

		toolConfig = append(toolConfig, meta)
	}

	return toolConfig

}
