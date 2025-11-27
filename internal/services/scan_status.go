package services

import (
	"fmt"
	"pipeliner/internal/dao"
	"pipeliner/internal/models"
	"pipeliner/pkg/logger"
	"pipeliner/pkg/tools"
)

type ScanStatusManager struct {
	scanDao dao.ScanDAO
	logger  *logger.Logger
}

func newScanStatusManager(scanDao dao.ScanDAO, logger *logger.Logger) *ScanStatusManager {
	return &ScanStatusManager{
		scanDao: scanDao,
		logger:  logger,
	}
}

func (m *ScanStatusManager) UpdateStatus(scanID, status string) error {
	scan, err := m.scanDao.GetScanByUUID(scanID)
	if err != nil {
		return err
	}
	scan.Status = status
	return m.scanDao.UpdateScan(scan)
}

func (m *ScanStatusManager) MarkFailed(scanID string) {
	m.MarkFailedWithReason(scanID, "Unknown error - check scan logs")
}

func (m *ScanStatusManager) MarkFailedWithReason(scanID string, reason string) {
	scan, err := m.scanDao.GetScanByUUID(scanID)
	if err != nil {
		m.logger.Error("Failed to load scan for failure update", logger.Fields{"error": err, "scan_id": scanID})
		return
	}
	if scan == nil {
		m.logger.Warn("Scan not found while marking failed", logger.Fields{"scan_id": scanID})
		return
	}

	scan.Status = "failed"
	scan.ErrorMessage = reason

	if err := m.scanDao.UpdateScan(scan); err != nil {
		m.logger.Error("Failed to persist failed scan status", logger.Fields{"error": err, "scan_id": scanID})
	}

	m.logger.Error("Scan marked as failed", logger.Fields{
		"scan_id": scanID,
		"reason":  reason,
	})
}

func (m *ScanStatusManager) MarkCompleted(scanID string) error {
	scan, err := m.scanDao.GetScanByUUID(scanID)
	if err != nil {
		return fmt.Errorf("load scan: %w", err)
	}
	if scan == nil {
		return fmt.Errorf("scan %s not found", scanID)
	}

	scan.Status = "completed"

	if err := m.scanDao.UpdateScan(scan); err != nil {
		return fmt.Errorf("persist scan completion: %w", err)
	}

	return nil
}

func (m *ScanStatusManager) MarkCompletedWithWarnings(scanID string, failedTools []tools.ToolError) error {
	scan, err := m.scanDao.GetScanByUUID(scanID)
	if err != nil {
		return fmt.Errorf("load scan: %w", err)
	}
	if scan == nil {
		return fmt.Errorf("scan %s not found", scanID)
	}

	scan.Status = "completed_with_warnings"

	scan.FailedTools = make([]models.ToolFailure, 0, len(failedTools))
	for _, tool := range failedTools {
		scan.FailedTools = append(scan.FailedTools, models.ToolFailure{
			ToolName: tool.Tool,
			Error:    tool.Err.Error(),
		})
	}

	if err := m.scanDao.UpdateScan(scan); err != nil {
		return fmt.Errorf("persist scan completion with warnings: %w", err)
	}

	return nil
}
