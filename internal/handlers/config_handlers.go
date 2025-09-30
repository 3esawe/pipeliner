package handlers

import (
	"pipeliner/internal/services"
	"pipeliner/pkg/logger"

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

func (h *ConfigHandler) GetScanModules(c *gin.Context) {
	c.JSON(200, h.configService.GetScanModules())
}
