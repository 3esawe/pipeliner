package handlers

import (
	"pipeliner/internal/models"
	"pipeliner/internal/services"
	"pipeliner/pkg/logger"
	"pipeliner/templates"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type ScanHandler struct {
	scanService services.ScanServiceMethods
	logger      *logger.Logger
}

func NewScanHandler(scanService services.ScanServiceMethods) *ScanHandler {
	return &ScanHandler{scanService: scanService, logger: logger.NewLogger(logrus.Level(logrus.InfoLevel))}
}

func (h *ScanHandler) HomePage(c *gin.Context) {
	c.Status(200)
	templates.Home().Render(c, c.Writer)
}

func (h *ScanHandler) StartScan(c *gin.Context) {
	var scanModel models.Scan
	var ScanRequest ScanRequest
	if err := c.ShouldBindJSON(&ScanRequest); err != nil {
		h.logger.Error("Failed to bind JSON:", logger.Fields{"error": err})
		c.JSON(400, gin.H{"error": "Invalid request payload"})
		return
	}

	scanModel.ScanType = ScanRequest.ScanType
	scanModel.Domain = ScanRequest.Domain
	h.logger.Info("Starting scan", logger.Fields{"scanType": scanModel.ScanType, "domain": scanModel.Domain})
	id, err := h.scanService.StartScan(&scanModel)
	if err != nil {
		h.logger.Error("Failed to start scan:", logger.Fields{"error": err})
		c.JSON(500, gin.H{"error": "Failed to start scan"})
		return
	}
	c.JSON(200, ScanResponse{ScanID: id})
}

func (h *ScanHandler) GetScanByUUID(c *gin.Context) {
	scanID := c.Param("id")
	scan, err := h.scanService.GetScanByUUID(scanID)
	if err != nil {
		h.logger.Error("Failed to get scan:", logger.Fields{"error": err})
		c.JSON(500, gin.H{"error": "Failed to get scan"})
		return
	}
	if scan == nil {
		h.logger.Error("Scan not found", logger.Fields{"scan_id": scanID})
		c.JSON(404, gin.H{"error": "Scan not found"})
		return
	}
	c.JSON(200, scan)
}
