package web

import (
	"net/http"
	"pipeliner/pkg/logger"
	"pipeliner/templates"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type IndexHandler struct {
	logger *logger.Logger
}

func NewIndexHandler() *IndexHandler {
	return &IndexHandler{
		logger: logger.NewLogger(logrus.Level(logrus.InfoLevel)),
	}
}

func (h *IndexHandler) HomePage(c *gin.Context) {
	if err := templates.Home().Render(c, c.Writer); err != nil {
		h.logger.Error("Failed to render home template", err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusOK)
}
