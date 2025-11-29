package web

import (
	"encoding/json"
	"net/http"
	"pipeliner/internal/services"
	"pipeliner/pkg/logger"
	"pipeliner/templates"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type ScanWebHandler struct {
	scanService   services.ScanServiceMethods
	configService services.ConfigServiceMethods
	logger        *logger.Logger
}

func NewScanWebHandler(scanService services.ScanServiceMethods, configService services.ConfigServiceMethods) *ScanWebHandler {
	return &ScanWebHandler{
		scanService:   scanService,
		configService: configService,
		logger:        logger.NewLogger(logrus.Level(logrus.InfoLevel)),
	}
}

func (h *ScanWebHandler) ScansPage(c *gin.Context) {
	var pagination struct {
		Page  int `form:"page"`
		Limit int `form:"limit"`
	}

	if err := c.ShouldBindQuery(&pagination); err != nil {
		h.logger.Warn("Failed to bind pagination params, using defaults", logger.Fields{"error": err})
	}

	if pagination.Page < 1 {
		pagination.Page = 1
	}
	if pagination.Limit < 1 {
		pagination.Limit = 20
	}
	if pagination.Limit > 100 {
		pagination.Limit = 100
	}

	scans, total, err := h.scanService.ListScansWithPagination(pagination.Page, pagination.Limit)
	if err != nil {
		h.logger.Error("Failed to list scans", logger.Fields{"error": err})
		c.Status(500)
		return
	}

	totalPages := int(total) / pagination.Limit
	if int(total)%pagination.Limit != 0 {
		totalPages++
	}

	paginationMeta := templates.PaginationInfo{
		Page:       pagination.Page,
		Limit:      pagination.Limit,
		Total:      int(total),
		TotalPages: totalPages,
		HasNext:    pagination.Page < totalPages,
		HasPrev:    pagination.Page > 1,
	}

	h.logger.Info("Rendering ScansPage", logger.Fields{
		"scan_count": len(scans),
		"page":       pagination.Page,
		"total":      total,
	})

	if err := templates.GetScans(scans, paginationMeta).Render(c, c.Writer); err != nil {
		h.logger.Error("Failed to render scans template", logger.Fields{"error": err})
		c.Status(500)
		return
	}
	c.Status(200)
}

func (h *ScanWebHandler) StartScanPage(c *gin.Context) {
	configs := h.configService.GetScanModules()
	h.logger.Info("Rendering StartScanPage", logger.Fields{
		"config_count": len(configs),
	})

	if err := templates.StartScan(configs).Render(c, c.Writer); err != nil {
		h.logger.Error("Failed to render start scan template", logger.Fields{"error": err})
		c.Status(500)
		return
	}

	c.Status(200)
}

func (h *ScanWebHandler) ScanDetailPage(c *gin.Context) {
	scanID := c.Param("id")
	if scanID == "" {
		h.logger.Warn("Scan detail requested without ID", logger.Fields{})
		c.Status(http.StatusBadRequest)
		return
	}

	scan, err := h.scanService.GetScanByUUID(scanID)
	if err != nil {
		h.logger.Error("Failed to load scan detail", logger.Fields{"error": err, "scan_id": scanID})
		c.Status(http.StatusInternalServerError)
		return
	}

	if scan == nil {
		h.logger.Warn("Scan not found", logger.Fields{"scan_id": scanID})
		c.Status(http.StatusNotFound)
		return
	}

	if c.GetHeader("HX-Request") != "" {
		if err := templates.ScanDetailContent(scan).Render(c, c.Writer); err != nil {
			h.logger.Error("Failed to render scan detail partial", logger.Fields{"error": err, "scan_id": scanID})
			c.Status(http.StatusInternalServerError)
			return
		}
	} else {
		if err := templates.ScanDetailPage(scan).Render(c, c.Writer); err != nil {
			h.logger.Error("Failed to render scan detail page", logger.Fields{"error": err, "scan_id": scanID})
			c.Status(http.StatusInternalServerError)
			return
		}
	}

	c.Status(http.StatusOK)
}

func (h *ScanWebHandler) ScreenShotsPage(c *gin.Context) {
	scanID := c.Param("id")
	if scanID == "" {
		h.logger.Warn("Screenshot page requested without scan ID", logger.Fields{})
		c.Status(http.StatusBadRequest)
		return
	}

	scan, err := h.scanService.GetScanByUUID(scanID)
	if err != nil {
		h.logger.Error("Failed to load scan for screenshots", logger.Fields{"error": err, "scan_id": scanID})
		c.Status(http.StatusInternalServerError)
		return
	}

	if scan == nil {
		h.logger.Warn("Scan not found for screenshots", logger.Fields{"scan_id": scanID})
		c.Status(http.StatusNotFound)
		return
	}

	if scan.ScreenshotsPath == "" || scan.ScreenshotsPath == "[]" {
		h.logger.Warn("No screenshots available for scan", logger.Fields{"scan_id": scanID})
		c.Status(http.StatusNotFound)
		return
	}

	var paths []string
	if err := json.Unmarshal([]byte(scan.ScreenshotsPath), &paths); err != nil {
		h.logger.Error("Failed to decode screenshot paths", logger.Fields{"error": err, "scan_id": scanID})
		c.Status(http.StatusInternalServerError)
		return
	}

	if err := templates.ScanScreenshotsPage(scan, paths).Render(c, c.Writer); err != nil {
		h.logger.Error("Failed to render screenshots page", logger.Fields{"error": err, "scan_id": scanID})
		c.Status(http.StatusInternalServerError)
		return
	}

	c.Status(http.StatusOK)
}

func (h *ScanWebHandler) SubdomainsPage(c *gin.Context) {
	scanID := c.Param("id")
	if scanID == "" {
		h.logger.Warn("Subdomains page requested without scan ID", logger.Fields{})
		c.Status(http.StatusBadRequest)
		return
	}

	var pagination struct {
		Page  int `form:"page"`
		Limit int `form:"limit"`
	}

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
		h.logger.Error("Failed to load scan for subdomains", logger.Fields{"error": err, "scan_id": scanID})
		c.Status(http.StatusInternalServerError)
		return
	}

	if scan == nil {
		h.logger.Warn("Scan not found for subdomains", logger.Fields{"scan_id": scanID})
		c.Status(http.StatusNotFound)
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

	paginationMeta := templates.PaginationInfo{
		Page:       pagination.Page,
		Limit:      pagination.Limit,
		Total:      totalSubdomains,
		TotalPages: totalPages,
		HasNext:    pagination.Page < totalPages,
		HasPrev:    pagination.Page > 1,
	}

	h.logger.Info("Rendering SubdomainsPage", logger.Fields{
		"scan_id":          scanID,
		"subdomain_count":  len(paginatedSubdomains),
		"total_subdomains": totalSubdomains,
		"page":             pagination.Page,
	})

	if err := templates.ScanSubdomainsPage(scan, paginatedSubdomains, paginationMeta).Render(c, c.Writer); err != nil {
		h.logger.Error("Failed to render subdomains page", logger.Fields{"error": err, "scan_id": scanID})
		c.Status(http.StatusInternalServerError)
		return
	}

	c.Status(http.StatusOK)
}
