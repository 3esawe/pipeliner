package services

import (
	"os"
	"path/filepath"
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
	log        *logger.Logger
	configPath string
}

func NewConfigService() ConfigServiceMethods {
	configPath, err := filepath.Abs("./config")
	if err != nil {
		configPath = "./config"
	}

	return &configService{
		log:        logger.NewLogger(logrus.Level(logrus.InfoLevel)),
		configPath: configPath,
	}
}

func (c *configService) GetScanModules() []tools.ChainConfig {

	files, err := os.ReadDir(c.configPath)
	if err != nil {
		c.log.WithError(err).Error("Failed to read config directory")
		return nil
	}

	toolConfig := make([]tools.ChainConfig, 0)

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".yaml") {
			continue
		}

		data, err := os.ReadFile(c.configPath + "/" + file.Name())
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
