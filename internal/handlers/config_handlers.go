package handlers

import (
	"pipeliner/internal/services"
	"pipeliner/pkg/logger"
	"pipeliner/templates"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type ConfigHandler struct {
	configService services.ConfigServiceMethods
	logger        *logger.Logger
}

func NewConfigHandler(configService services.ConfigServiceMethods) *ConfigHandler {
	return &ConfigHandler{
		configService: configService,
		logger:        logger.NewLogger(logrus.Level(logrus.InfoLevel)),
	}
}

func (h *ConfigHandler) ConfigPage(c *gin.Context) {
	configs := h.configService.GetScanModules()

	h.logger.Info("ConfigPage called", logger.Fields{
		"config_count": len(configs),
	})

	// Debug: log each config
	for i, config := range configs {
		h.logger.Info("Config details", logger.Fields{
			"index":       i,
			"description": config.Description,
			"exec_mode":   config.ExecutionMode,
			"tool_count":  len(config.Tools),
		})
	}

	c.Status(200)
	templates.CurrentConfig(configs).Render(c, c.Writer)
}

func (h *ConfigHandler) GetScanModules(c *gin.Context) {
	c.JSON(200, h.configService.GetScanModules())
}
