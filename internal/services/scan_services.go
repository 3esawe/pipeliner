package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"pipeliner/internal/dao"
	"pipeliner/internal/models"
	"pipeliner/pkg/engine"
	"pipeliner/pkg/logger"
	"pipeliner/pkg/tools"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
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

var ErrScanNotFound = errors.New("scan not found")

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
	scanDir := e.ScanDirectory()

	if scanDir != "" {
		s.monitorScanProgress(scan.UUID, scan.ScanType, scanDir, ctx)
	} else {
		s.logger.Warn("Scan directory not available for monitoring", logger.Fields{"scan_id": id})
	}

	go func(scanID, scanType, domain string) {
		defer func() {
			cancel()
			if r := recover(); r != nil {
				s.logger.Error("panic in background scan", logger.Fields{"scan_id": scanID, "panic": r})
			}
		}()

		runErr := e.RunHTTP(scanType, domain)

		if runErr != nil {
			s.logger.Error("RunHTTP failed", logger.Fields{"scan_id": scanID, "error": runErr})
			s.markScanFailed(scanID)
			return
		}

		s.logger.Info("Scan completed successfully", logger.Fields{"scan_id": scanID})
		if err := s.markScanCompleted(scanID); err != nil {
			s.logger.Error("Failed to finalize scan", logger.Fields{"scan_id": scanID, "error": err})
		}
	}(id, scan.ScanType, scan.Domain)

	return id, nil
}

func (s *scanService) GetScanByUUID(id string) (*models.Scan, error) {
	scan, err := s.scanDao.GetScanByUUID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrScanNotFound
		}
		return nil, err
	}
	return scan, nil
}

func (s *scanService) ListScans() ([]models.Scan, error) {
	return s.scanDao.ListScans()
}

func (s *scanService) DeleteScan(id string) error {
	return s.scanDao.DeleteScan(id)
}

func (s *scanService) monitorScanProgress(scanID, scanType, scanDir string, ctx context.Context) {
	if scanDir == "" {
		s.logger.Warn("Scan directory missing for monitoring", logger.Fields{"scan_id": scanID})
		return
	}

	filesToMonitor := s.getFilesToMonitor(scanType)

	for _, fileConfig := range filesToMonitor {
		go s.monitorFile(scanID, scanDir, fileConfig, ctx)
	}

	// Monitor screenshots directory for real-time updates
	go s.monitorScreenshots(scanID, scanDir, ctx)
}

type FileMonitorConfig struct {
	FilePath string
	Metric   string // "domains", "ports", etc.
}

func (s *scanService) monitorScreenshots(scanID, scanDir string, ctx context.Context) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		s.logger.Error("Failed to create screenshot watcher", logger.Fields{"error": err, "scan_id": scanID})
		return
	}
	defer watcher.Close()

	if err := watcher.Add(scanDir); err != nil {
		s.logger.Error("Error adding directory to watcher", logger.Fields{"error": err, "dir": scanDir, "scan_id": scanID})
		return
	}

	// Initial scan of existing screenshots
	s.updateScreenshots(scanID, scanDir)

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	updatePending := false
	var mu sync.Mutex

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			// Check if it's a screenshot file
			if event.Op&(fsnotify.Create|fsnotify.Write) != 0 {
				ext := strings.ToLower(filepath.Ext(event.Name))
				if ext == ".jpeg" || ext == ".jpg" || ext == ".png" {
					mu.Lock()
					updatePending = true
					mu.Unlock()
				}
			}

		case <-ticker.C:
			mu.Lock()
			if updatePending {
				s.updateScreenshots(scanID, scanDir)
				updatePending = false
			}
			mu.Unlock()

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			s.logger.Error("Screenshot watcher error", logger.Fields{"error": err, "dir": scanDir, "scan_id": scanID})

		case <-ctx.Done():
			s.logger.Info("Stopping screenshot monitor", logger.Fields{"dir": scanDir, "scan_id": scanID})
			return
		}
	}
}

func (s *scanService) updateScreenshots(scanID, scanDir string) {
	scan, err := s.scanDao.GetScanByUUID(scanID)
	if err != nil {
		s.logger.Error("Failed to load scan for screenshot update", logger.Fields{"error": err, "scan_id": scanID})
		return
	}

	if err := s.saveScreenShotPaths(scan, scanDir); err != nil {
		s.logger.Error("Failed to update screenshot paths", logger.Fields{"error": err, "scan_id": scanID})
		return
	}

	if err := s.scanDao.UpdateScan(scan); err != nil {
		s.logger.Error("Failed to persist screenshot update", logger.Fields{"error": err, "scan_id": scanID})
		return
	}

	s.logger.Info("Updated screenshot paths", logger.Fields{"scan_id": scanID, "screenshots": scan.ScreenshotsPath})
}

func (s *scanService) saveScreenShotPaths(scan *models.Scan, scanDir string) error {
	if scanDir == "" {
		s.logger.Warn("Scan directory not provided for screenshot persistence", logger.Fields{"scan_id": scan.UUID})
		return nil
	}

	patterns := []string{"*.jpeg", "*.jpg", "*.png"}
	seen := make(map[string]struct{})
	var paths []string
	scanDirName := filepath.Base(scanDir)

	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(scanDir, pattern))
		if err != nil {
			s.logger.Error("Failed to glob screenshot files", logger.Fields{"error": err, "pattern": pattern, "scan_dir": scanDir})
			continue
		}
		for _, match := range matches {
			filename := filepath.Base(match)
			if filename == "" {
				continue
			}
			key := strings.ToLower(filename)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			relative := filepath.Join(scanDirName, filename)
			paths = append(paths, relative)
		}
	}

	sort.Strings(paths)

	encoded, err := json.Marshal(paths)
	if err != nil {
		s.logger.Error("Failed to encode screenshot paths", logger.Fields{"error": err, "scan_dir": scanDir})
		return err
	}

	scan.ScreenshotsPath = string(encoded)
	return nil
}

func (s *scanService) getFilesToMonitor(scanType string) []FileMonitorConfig {
	switch scanType {
	case "subdomain":
		return []FileMonitorConfig{
			{FilePath: "httpx_output.txt", Metric: "domains"},
			{FilePath: ".txt", Metric: "vulns"},
		}
	case "portscan":
		return []FileMonitorConfig{
			{FilePath: "nmap_output.txt", Metric: "ports"},
		}
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

func (s *scanService) monitorFile(scanID, scanDir string, config FileMonitorConfig, ctx context.Context) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		s.logger.Error("Failed to create file watcher", logger.Fields{"error": err, "file": config.FilePath, "scan_id": scanID})
		return
	}
	defer watcher.Close()

	fullPath := filepath.Join(scanDir, config.FilePath)

	if !s.waitForFile(fullPath, ctx) {
		return
	}

	if err := watcher.Add(fullPath); err != nil {
		s.logger.Error("Error adding file to watcher", logger.Fields{"error": err, "file": fullPath, "scan_id": scanID})
		return
	}

	var lastSize int64
	var mu sync.Mutex

	s.processFileUpdate(scanID, config, fullPath, &lastSize)

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
				s.processFileUpdate(scanID, config, fullPath, &lastSize)
				updatePending = false
			}
			mu.Unlock()

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			s.logger.Error("File watcher error", logger.Fields{"error": err, "file": fullPath, "scan_id": scanID})

		case <-ctx.Done():
			s.logger.Info("Stopping file monitor", logger.Fields{"file": fullPath, "scan_id": scanID})
			return
		}
	}
}

func (s *scanService) processFileUpdate(scanID string, config FileMonitorConfig, filePath string, lastSize *int64) {
	file, err := os.Open(filePath)
	if err != nil {
		s.logger.Error("Failed to open file", logger.Fields{"error": err, "file": filePath, "scan_id": scanID})
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		s.logger.Error("Failed to stat file", logger.Fields{"error": err, "file": filePath, "scan_id": scanID})
		return
	}

	currentSize := stat.Size()
	if currentSize <= *lastSize {
		return
	}

	if _, err := file.Seek(*lastSize, io.SeekStart); err != nil {
		s.logger.Error("Failed to seek file", logger.Fields{"error": err, "file": filePath, "scan_id": scanID})
		return
	}

	newContent := make([]byte, currentSize-*lastSize)
	n, err := file.Read(newContent)
	if err != nil && err != io.EOF {
		s.logger.Error("Failed to read new content", logger.Fields{"error": err, "file": filePath, "scan_id": scanID})
		return
	}
	newContent = newContent[:n]

	lines := strings.Split(string(newContent), "\n")
	newCount := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			newCount++
		}
	}

	if newCount > 0 {
		s.incrementScanMetric(scanID, config.Metric, newCount)
	}
	*lastSize = currentSize
}

func (s *scanService) incrementScanMetric(scanID, metric string, increment int) {
	if increment <= 0 {
		return
	}

	scan, err := s.scanDao.GetScanByUUID(scanID)
	if err != nil {
		s.logger.Error("Failed to load scan for metric update", logger.Fields{"error": err, "scan_id": scanID, "metric": metric})
		return
	}

	switch metric {
	case "domains":
		scan.NumberOfDomains += increment
	default:
		s.logger.Warn("Unknown metric type for scan update", logger.Fields{"scan_id": scanID, "metric": metric})
	}

	if scan.Status != "completed" {
		scan.Status = "running"
	}

	if err := s.scanDao.UpdateScan(scan); err != nil {
		s.logger.Error("Failed to update scan metric", logger.Fields{"error": err, "scan_id": scanID, "metric": metric})
		return
	}

	s.logger.Info("Updated scan metric", logger.Fields{
		"scan_id":   scanID,
		"metric":    metric,
		"increment": increment,
		"total":     scan.NumberOfDomains,
	})
}

func (s *scanService) markScanFailed(scanID string) {
	scan, err := s.scanDao.GetScanByUUID(scanID)
	if err != nil {
		s.logger.Error("Failed to load scan for failure update", logger.Fields{"error": err, "scan_id": scanID})
		return
	}
	if scan == nil {
		s.logger.Warn("Scan not found while marking failed", logger.Fields{"scan_id": scanID})
		return
	}

	scan.Status = "failed"
	if err := s.scanDao.UpdateScan(scan); err != nil {
		s.logger.Error("Failed to persist failed scan status", logger.Fields{"error": err, "scan_id": scanID})
	}
}

func (s *scanService) markScanCompleted(scanID string) error {
	scan, err := s.scanDao.GetScanByUUID(scanID)
	if err != nil {
		return fmt.Errorf("load scan: %w", err)
	}
	if scan == nil {
		return fmt.Errorf("scan %s not found", scanID)
	}

	scan.Status = "completed"

	if err := s.scanDao.UpdateScan(scan); err != nil {
		return fmt.Errorf("persist scan completion: %w", err)
	}

	return nil
}
