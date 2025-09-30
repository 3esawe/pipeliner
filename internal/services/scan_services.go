package services

import (
	"context"
	"io"
	"os"
	"pipeliner/internal/dao"
	"pipeliner/internal/models"
	"pipeliner/pkg/engine"
	"pipeliner/pkg/logger"
	"pipeliner/pkg/tools"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type ScanServiceMethods interface {
	StartScan(scan *models.Scan) (string, error)
	GetScanByUUID(id string) (*models.Scan, error)
	ListScans() ([]models.Scan, error)
	DeleteScan(id string) error
}

type scanService struct {
	scanDao dao.ScanDAO
	logger  *logger.Logger
}

func NewScanService(scanDao dao.ScanDAO) ScanServiceMethods {
	return &scanService{scanDao: scanDao, logger: logger.NewLogger(logrus.InfoLevel)}
}

func (s *scanService) StartScan(scan *models.Scan) (string, error) {
	id := uuid.New().String()
	scan.UUID = id
	scan.Status = "started"

	e, err := engine.NewPiplinerEngine()
	if err != nil {
		s.logger.Error("Failed to create engine", logger.Fields{"error": err})
		return "", err
	}

	if err := e.PrepareScan(&tools.Options{
		ScanType: scan.ScanType,
		Domain:   scan.Domain,
	}); err != nil {
		s.logger.Error("PrepareScan failed", logger.Fields{"error": err})
		return "", err
	}

	if err := s.scanDao.SaveScan(scan); err != nil {
		s.logger.Error("SaveScan failed", logger.Fields{"error": err})
		return "", err
	}

	ctx, cancel := context.WithCancel(context.Background())

	go s.monitorScanProgress(scan, ctx)

	go func(scanID, scanType, domain string) {
		defer func() {
			cancel() // Stop monitoring when scan is done
			if r := recover(); r != nil {
				s.logger.Error("panic in background scan", logger.Fields{"scan_id": scanID, "panic": r})
			}
		}()

		if runErr := e.RunHTTP(scanType, domain); runErr != nil {
			s.logger.Error("RunHTTP failed", logger.Fields{"scan_id": scanID, "error": runErr})
			scan.Status = "failed"
		} else {
			s.logger.Info("Scan completed successfully", logger.Fields{"scan_id": scanID})
			scan.Status = "completed"
		}

		if err := s.scanDao.UpdateScan(scan); err != nil {
			s.logger.Error("UpdateScan failed", logger.Fields{"error": err, "scan_id": scanID})
		}
	}(id, scan.ScanType, scan.Domain)

	return id, nil
}

func (s *scanService) GetScanByUUID(id string) (*models.Scan, error) {
	return s.scanDao.GetScanByUUID(id)
}

func (s *scanService) ListScans() ([]models.Scan, error) {
	return s.scanDao.ListScans()
}

func (s *scanService) DeleteScan(id string) error {
	return s.scanDao.DeleteScan(id)
}

func (s *scanService) monitorScanProgress(scan *models.Scan, ctx context.Context) {
	filesToMonitor := s.getFilesToMonitor(scan.ScanType)

	for _, fileConfig := range filesToMonitor {
		go s.monitorFile(scan, fileConfig, ctx)
	}
}

type FileMonitorConfig struct {
	FilePath string
	Metric   string // "domains", "ports", etc.
}

func (s *scanService) getFilesToMonitor(scanType string) []FileMonitorConfig {
	switch scanType {
	case "subdomain":
		return []FileMonitorConfig{
			{FilePath: "httpx_output.txt", Metric: "domains"},
		}
	case "portscan":
		return []FileMonitorConfig{
			{FilePath: "nmap_output.txt", Metric: "ports"},
		}
	// Add more scan types as needed
	default:
		return []FileMonitorConfig{
			{FilePath: "httpx_output.txt", Metric: "domains"},
		}
	}
}

func (s *scanService) waitForFile(filePath string, ctx context.Context) bool {
	if _, err := os.Stat(filePath); err == nil {
		return true // fule exists
	}

	s.logger.Info("Waiting for file to be created", logger.Fields{"file": filePath})

	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			s.logger.Error("Timeout waiting for file to be created", logger.Fields{"file": filePath})
			return false

		case <-ticker.C:
			if _, err := os.Stat(filePath); err == nil {
				s.logger.Info("File created, starting monitoring", logger.Fields{"file": filePath})
				return true
			}

		case <-ctx.Done():
			s.logger.Info("Context cancelled while waiting for file", logger.Fields{"file": filePath})
			return false
		}
	}
}

func (s *scanService) monitorFile(scan *models.Scan, config FileMonitorConfig, ctx context.Context) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		s.logger.Error("Failed to create file watcher", logger.Fields{"error": err, "file": config.FilePath})
		return
	}
	defer watcher.Close()

	if !s.waitForFile(config.FilePath, ctx) {
		return
	}

	if err := watcher.Add(config.FilePath); err != nil {
		s.logger.Error("Error adding file to watcher", logger.Fields{"error": err, "file": config.FilePath})
		return
	}

	var lastSize int64 = 0
	var mu sync.Mutex

	// Throttle updates to avoid too frequent database writes
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	updatePending := false

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			if event.Op&fsnotify.Write == fsnotify.Write {
				mu.Lock()
				updatePending = true
				mu.Unlock()
			}

		case <-ticker.C:
			mu.Lock()
			if updatePending {
				s.processFileUpdate(scan, config, &lastSize)
				updatePending = false
			}
			mu.Unlock()

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			s.logger.Error("File watcher error", logger.Fields{"error": err, "file": config.FilePath})

		case <-ctx.Done():
			s.logger.Info("Stopping file monitor", logger.Fields{"file": config.FilePath})
			return
		}
	}
}

func (s *scanService) processFileUpdate(scan *models.Scan, config FileMonitorConfig, lastSize *int64) {
	file, err := os.Open(config.FilePath)
	if err != nil {
		s.logger.Error("Failed to open file", logger.Fields{"error": err, "file": config.FilePath})
		return
	}
	defer file.Close()

	// Get current file size
	stat, err := file.Stat()
	if err != nil {
		s.logger.Error("Failed to stat file", logger.Fields{"error": err, "file": config.FilePath})
		return
	}

	currentSize := stat.Size()
	if currentSize <= *lastSize {
		return // No new content
	}

	// Seek to the last read position
	if _, err := file.Seek(*lastSize, io.SeekStart); err != nil {
		s.logger.Error("Failed to seek file", logger.Fields{"error": err, "file": config.FilePath})
		return
	}

	// Read only the new content
	newContent := make([]byte, currentSize-*lastSize)
	if _, err := file.Read(newContent); err != nil {
		s.logger.Error("Failed to read new content", logger.Fields{"error": err, "file": config.FilePath})
		return
	}

	// Count new lines
	newLines := strings.Split(string(newContent), "\n")
	newCount := 0
	for _, line := range newLines {
		if strings.TrimSpace(line) != "" {
			newCount++
		}
	}

	// Update the appropriate metric
	s.updateScanMetric(scan, config.Metric, newCount)
	*lastSize = currentSize
}

func (s *scanService) updateScanMetric(scan *models.Scan, metric string, increment int) {
	switch metric {
	case "domains":
		scan.NumberOfDomains += increment
	}

	scan.Status = "running"
	s.logger.Info("Updated scan metric", logger.Fields{
		"scan_id":   scan.UUID,
		"metric":    metric,
		"increment": increment,
		"total":     scan.NumberOfDomains,
	})
	if err := s.scanDao.UpdateScan(scan); err != nil {
		s.logger.Error("Failed to update scan", logger.Fields{
			"error":   err,
			"scan_id": scan.UUID,
			"metric":  metric,
		})
	} else {
		s.logger.Debug("Updated scan metric", logger.Fields{
			"scan_id":   scan.UUID,
			"metric":    metric,
			"increment": increment,
			"total":     scan.NumberOfDomains,
		})
	}
}
