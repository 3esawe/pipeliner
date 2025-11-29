package handlers

import (
	"errors"
	"pipeliner/internal/models"
	"pipeliner/internal/services"
	"pipeliner/pkg/engine"
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
	scanModel.SensitivePatterns = ScanRequest.SensitivePatterns
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
	var pagination PaginationRequest

	if err := c.ShouldBindQuery(&pagination); err != nil {
		h.logger.Warn("Failed to bind pagination params, using defaults", logger.Fields{"error": err})
	}

	if pagination.Page < 1 {
		pagination.Page = 1
	}
	if pagination.Limit < 1 {
		pagination.Limit = 10
	}
	if pagination.Limit > 100 {
		pagination.Limit = 100
	}

	scans, total, err := h.scanService.ListScansWithPagination(pagination.Page, pagination.Limit)
	if err != nil {
		h.logger.Error("Failed to list scans:", logger.Fields{"error": err})
		c.JSON(500, gin.H{"error": "Failed to list scans"})
		return
	}

	totalPages := int(total) / pagination.Limit
	if int(total)%pagination.Limit != 0 {
		totalPages++
	}

	response := PaginatedScansResponse{
		Scans: scans,
		Pagination: PaginationMeta{
			Page:       pagination.Page,
			Limit:      pagination.Limit,
			Total:      int(total),
			TotalPages: totalPages,
			HasNext:    pagination.Page < totalPages,
			HasPrev:    pagination.Page > 1,
		},
	}

	c.JSON(200, response)
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

func (h *ScanHandler) GetQueueStatus(c *gin.Context) {
	queue := engine.GetGlobalQueue()
	running, queued, maxConcurrent := queue.GetStatus()

	c.JSON(200, gin.H{
		"running":        running,
		"queued":         queued,
		"max_concurrent": maxConcurrent,
		"available":      maxConcurrent - running,
	})
}

func (h *ScanHandler) GetScanSubdomains(c *gin.Context) {
	scanID := c.Param("id")
	if scanID == "" {
		h.logger.Error("Scan ID missing in subdomains request")
		c.JSON(400, gin.H{"error": "Scan ID is required"})
		return
	}

	var pagination PaginationRequest

	if err := c.ShouldBindQuery(&pagination); err != nil {
		h.logger.Warn("Failed to bind pagination params, using defaults", logger.Fields{"error": err})
	}

	if pagination.Page < 1 {
		pagination.Page = 1
	}
	if pagination.Limit < 1 {
		pagination.Limit = 50
	}
	if pagination.Limit > 200 {
		pagination.Limit = 200
	}

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

	// Paginate subdomains
	totalSubdomains := len(scan.Subdomains)
	offset := (pagination.Page - 1) * pagination.Limit
	end := offset + pagination.Limit

	if offset > totalSubdomains {
		offset = totalSubdomains
	}
	if end > totalSubdomains {
		end = totalSubdomains
	}

	paginatedSubdomains := scan.Subdomains[offset:end]

	totalPages := totalSubdomains / pagination.Limit
	if totalSubdomains%pagination.Limit != 0 {
		totalPages++
	}

	response := gin.H{
		"scan_id":    scan.UUID,
		"domain":     scan.Domain,
		"subdomains": paginatedSubdomains,
		"pagination": PaginationMeta{
			Page:       pagination.Page,
			Limit:      pagination.Limit,
			Total:      totalSubdomains,
			TotalPages: totalPages,
			HasNext:    pagination.Page < totalPages,
			HasPrev:    pagination.Page > 1,
		},
	}

	c.JSON(200, response)
}
