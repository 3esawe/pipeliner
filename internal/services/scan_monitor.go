package services

import (
	"context"
	"os"
	"path/filepath"
	"pipeliner/internal/dao"
	"pipeliner/internal/models"
	"pipeliner/pkg/logger"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type ScanMonitor struct {
	scanDao     dao.ScanDAO
	logger      *logger.Logger
	scanMutexes *sync.Map
	artifacts   *ArtifactProcessor
}

func newScanMonitor(scanDao dao.ScanDAO, logger *logger.Logger, scanMutexes *sync.Map, artifacts *ArtifactProcessor) *ScanMonitor {
	return &ScanMonitor{
		scanDao:     scanDao,
		logger:      logger,
		scanMutexes: scanMutexes,
		artifacts:   artifacts,
	}
}

func (m *ScanMonitor) getScanMutex(scanID string) *sync.Mutex {
	value, _ := m.scanMutexes.LoadOrStore(scanID, &sync.Mutex{})
	return value.(*sync.Mutex)
}

func (m *ScanMonitor) MonitorScanProgress(scanID, scanType, scanDir string, ctx context.Context, done chan struct{}) {
	defer close(done)

	if scanDir == "" {
		m.logger.Warn("Scan directory missing for monitoring", logger.Fields{"scan_id": scanID})
		return
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		m.monitorSubdomains(scanID, scanDir, ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		m.monitorArtifacts(scanID, scanDir, ctx)
	}()

	wg.Wait()
	m.logger.Info("All monitors finished", logger.Fields{"scan_id": scanID})
}

func (m *ScanMonitor) monitorArtifacts(scanID, scanDir string, ctx context.Context) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		m.logger.Error("Failed to create artifact watcher", logger.Fields{"error": err, "scan_id": scanID})
		return
	}
	defer watcher.Close()

	if err := watcher.Add(scanDir); err != nil {
		m.logger.Error("Error adding directory to watcher", logger.Fields{"error": err, "dir": scanDir, "scan_id": scanID})
		return
	}

	m.artifacts.UpdateArtifacts(scanID, scanDir)

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

			if event.Op&(fsnotify.Create|fsnotify.Write) != 0 {
				filename := filepath.Base(event.Name)
				ext := strings.ToLower(filepath.Ext(event.Name))

				isArtifact := false
				if ext == ".jpeg" || ext == ".jpg" || ext == ".png" {
					isArtifact = true
				}
				if filename == "nmap_output.xml" {
					isArtifact = true
				}
				if strings.HasSuffix(filename, "_ffuf_output.json") {
					isArtifact = true
				}

				if isArtifact {
					mu.Lock()
					updatePending = true
					mu.Unlock()
				}
			}

		case <-ticker.C:
			mu.Lock()
			if updatePending {
				m.artifacts.UpdateArtifacts(scanID, scanDir)
				updatePending = false
			}
			mu.Unlock()

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			m.logger.Error("Artifact watcher error", logger.Fields{"error": err, "dir": scanDir, "scan_id": scanID})

		case <-ctx.Done():
			m.logger.Info("Stopping artifact monitor, performing final update", logger.Fields{"dir": scanDir, "scan_id": scanID})
			m.artifacts.UpdateArtifacts(scanID, scanDir)
			return
		}
	}
}

func (m *ScanMonitor) monitorSubdomains(scanID, scanDir string, ctx context.Context) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		m.logger.Error("Failed to create subdomain watcher", logger.Fields{"error": err, "scan_id": scanID})
		return
	}
	defer watcher.Close()

	httpxPath := filepath.Join(scanDir, "httpx_output.txt")

	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	fileExists := false
	for !fileExists {
		select {
		case <-timeout:
			m.logger.Warn("Timeout waiting for httpx_output.txt", logger.Fields{"scan_id": scanID})
			return
		case <-ticker.C:
			if _, err := os.Stat(httpxPath); err == nil {
				fileExists = true
			}
		case <-ctx.Done():
			return
		}
	}

	if err := watcher.Add(httpxPath); err != nil {
		m.logger.Error("Error adding httpx_output.txt to watcher", logger.Fields{"error": err, "file": httpxPath, "scan_id": scanID})
		return
	}

	m.logger.Info("Started monitoring subdomain discovery", logger.Fields{"scan_id": scanID, "file": httpxPath})

	var lastSize int64
	var mu sync.Mutex

	m.processSubdomainUpdate(scanID, httpxPath, &lastSize)

	updateTicker := time.NewTicker(2 * time.Second)
	defer updateTicker.Stop()

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

		case <-updateTicker.C:
			mu.Lock()
			if updatePending {
				m.processSubdomainUpdate(scanID, httpxPath, &lastSize)
				updatePending = false
			}
			mu.Unlock()

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			m.logger.Error("Subdomain watcher error", logger.Fields{"error": err, "file": httpxPath, "scan_id": scanID})

		case <-ctx.Done():
			m.logger.Info("Stopping subdomain monitor, performing final update", logger.Fields{"file": httpxPath, "scan_id": scanID})
			m.processSubdomainUpdate(scanID, httpxPath, &lastSize)
			return
		}
	}
}

func (m *ScanMonitor) processSubdomainUpdate(scanID, filePath string, lastSize *int64) {
	file, err := os.Open(filePath)
	if err != nil {
		m.logger.Error("Failed to open httpx_output.txt", logger.Fields{"error": err, "file": filePath, "scan_id": scanID})
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		m.logger.Error("Failed to stat httpx_output.txt", logger.Fields{"error": err, "file": filePath, "scan_id": scanID})
		return
	}

	currentSize := stat.Size()
	if currentSize <= *lastSize {
		return
	}

	if _, err := file.Seek(*lastSize, 0); err != nil {
		m.logger.Error("Failed to seek httpx_output.txt", logger.Fields{"error": err, "file": filePath, "scan_id": scanID})
		return
	}

	newContent := make([]byte, currentSize-*lastSize)
	n, err := file.Read(newContent)
	if err != nil {
		m.logger.Error("Failed to read new content from httpx_output.txt", logger.Fields{"error": err, "file": filePath, "scan_id": scanID})
		return
	}
	newContent = newContent[:n]

	lines := strings.Split(string(newContent), "\n")
	var validLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			validLines = append(validLines, trimmed)
		}
	}

	if len(validLines) > 0 {
		mu := m.getScanMutex(scanID)
		mu.Lock()
		defer mu.Unlock()

		scan, err := m.scanDao.GetScanByUUID(scanID)
		if err != nil {
			m.logger.Error("Failed to load scan for subdomain update", logger.Fields{"error": err, "scan_id": scanID})
			return
		}

		for _, line := range validLines {
			subdomain := models.Subdomain{
				Domain: line,
				Status: "discovered",
			}
			scan.Subdomains = append(scan.Subdomains, subdomain)
		}

		scan.NumberOfDomains = len(scan.Subdomains)

		if scan.Status != "completed" && scan.Status != "failed" {
			scan.Status = "running"
		}

		if err := m.scanDao.UpdateScan(scan); err != nil {
			m.logger.Error("Failed to update scan with new subdomains", logger.Fields{"error": err, "scan_id": scanID})
			return
		}

		m.logger.Info("Added new subdomains", logger.Fields{
			"scan_id": scanID,
			"count":   len(validLines),
			"total":   len(scan.Subdomains),
		})
	}

	*lastSize = currentSize
}
