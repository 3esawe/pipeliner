package handlers

import (
	"errors"
	"pipeliner/internal/models"
	"pipeliner/internal/services"
	"pipeliner/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type ScanHandler struct {
	scanService services.ScanServiceMethods
	logger      *logger.Logger
}

func NewScanHandler(scanService services.ScanServiceMethods) *ScanHandler {
	return &ScanHandler{scanService: scanService, logger: logger.NewLogger(logrus.Level(logrus.InfoLevel))}
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
		if errors.Is(err, services.ErrScanNotFound) {
			h.logger.Warn("Scan not found", logger.Fields{"scan_id": scanID})
			c.JSON(404, gin.H{"error": "Scan not found"})
			return
		}
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

func (h *ScanHandler) ListScans(c *gin.Context) {
	scans, err := h.scanService.ListScans()
	if err != nil {
		h.logger.Error("Failed to list scans:", logger.Fields{"error": err})
		c.JSON(500, gin.H{"error": "Failed to list scans"})
		return
	}
	c.JSON(200, scans)
}

func (h *ScanHandler) DeleteScan(c *gin.Context) {
	scanID := c.Param("id")
	if scanID == "" {
		h.logger.Error("Scan ID missing in delete request")
		c.JSON(400, gin.H{"error": "Scan ID is required"})
		return
	}

	if err := h.scanService.DeleteScan(scanID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			h.logger.Warn("Scan not found for deletion", logger.Fields{"scan_id": scanID})
			c.JSON(404, gin.H{"error": "Scan not found"})
			return
		}
		h.logger.Error("Failed to delete scan", logger.Fields{"error": err, "scan_id": scanID})
		c.JSON(500, gin.H{"error": "Failed to delete scan"})
		return
	}

	c.Status(204)
}
