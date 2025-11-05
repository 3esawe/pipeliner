package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"pipeliner/internal/dao"
	"pipeliner/internal/models"
	"pipeliner/internal/notification"
	"pipeliner/pkg/engine"
	"pipeliner/pkg/logger"
	"pipeliner/pkg/parsers"
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
	scanDao            dao.ScanDAO
	logger             *logger.Logger
	scanMutexes        sync.Map
	notificationClient *notification.NotificationClient
}

var ErrScanNotFound = errors.New("scan not found")

func NewScanService(scanDao dao.ScanDAO) ScanServiceMethods {
	notifClient, err := notification.NewNotificationClient()
	if err != nil {
		logger.NewLogger(logrus.InfoLevel).WithError(err).Warn("Failed to initialize notification client - notifications disabled")
	}

	return &scanService{
		scanDao:            scanDao,
		logger:             logger.NewLogger(logrus.InfoLevel),
		scanMutexes:        sync.Map{},
		notificationClient: notifClient,
	}
}

func (s *scanService) getScanMutex(scanID string) *sync.Mutex {
	value, _ := s.scanMutexes.LoadOrStore(scanID, &sync.Mutex{})
	return value.(*sync.Mutex)
}

func (s *scanService) StartScan(scan *models.Scan) (string, error) {
	id := uuid.New().String()
	scan.UUID = id
	scan.Status = "queued"

	if err := s.scanDao.SaveScan(scan); err != nil {
		s.logger.Error("SaveScan failed", logger.Fields{"error": err})
		return "", err
	}

	go s.executeScan(id, scan.ScanType, scan.Domain)

	return id, nil
}

func (s *scanService) executeScan(scanID, scanType, domain string) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("panic in background scan", logger.Fields{"scan_id": scanID, "panic": r})
			s.markScanFailed(scanID)
		}
	}()

	queue := engine.GetGlobalQueue()
	err := queue.ExecuteWithQueue(func() error {
		if err := s.updateScanStatus(scanID, "running"); err != nil {
			s.logger.Error("Failed to update scan to running", logger.Fields{"scan_id": scanID, "error": err})
		}

		s.logger.Info("Starting scan execution", logger.Fields{"scan_id": scanID, "scan_type": scanType, "domain": domain})

		e, err := engine.NewPiplinerEngine()
		if err != nil {
			s.logger.Error("Failed to create engine", logger.Fields{"error": err, "scan_id": scanID})
			return err
		}

		if err := e.PrepareScan(&tools.Options{
			ScanType: scanType,
			Domain:   domain,
		}); err != nil {
			s.logger.Error("PrepareScan failed", logger.Fields{"error": err, "scan_id": scanID})
			return err
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		scanDir := e.ScanDirectory()

		var monitoringDone chan struct{}
		if scanDir != "" {
			monitoringDone = make(chan struct{})
			go s.monitorScanProgress(scanID, scanType, scanDir, ctx, monitoringDone)
		} else {
			s.logger.Warn("Scan directory not available for monitoring", logger.Fields{"scan_id": scanID})
		}

		runErr := e.RunHTTP(scanType, domain)

		cancel()

		if monitoringDone != nil {
			s.logger.Info("Waiting for monitors to complete final processing", logger.Fields{"scan_id": scanID})
			<-monitoringDone
			s.logger.Info("Monitors completed, finalizing scan status", logger.Fields{"scan_id": scanID})
		}

		return runErr
	})

	if err != nil {
		s.logger.Error("Scan execution failed", logger.Fields{"scan_id": scanID, "error": err})
		s.markScanFailed(scanID)
		return
	}

	s.logger.Info("Scan completed successfully", logger.Fields{"scan_id": scanID})
	if err := s.markScanCompleted(scanID); err != nil {
		s.logger.Error("Failed to finalize scan", logger.Fields{"scan_id": scanID, "error": err})
	}
}

func (s *scanService) updateScanStatus(scanID, status string) error {
	scan, err := s.scanDao.GetScanByUUID(scanID)
	if err != nil {
		return err
	}
	scan.Status = status
	return s.scanDao.UpdateScan(scan)
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

func (s *scanService) monitorScanProgress(scanID, scanType, scanDir string, ctx context.Context, done chan struct{}) {
	defer close(done) // Signal completion when this function exits

	if scanDir == "" {
		s.logger.Warn("Scan directory missing for monitoring", logger.Fields{"scan_id": scanID})
		return
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		s.monitorSubdomains(scanID, scanDir, ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		s.monitorArtifacts(scanID, scanDir, ctx)
	}()

	wg.Wait()
	s.logger.Info("All monitors finished", logger.Fields{"scan_id": scanID})
}

func (s *scanService) monitorArtifacts(scanID, scanDir string, ctx context.Context) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		s.logger.Error("Failed to create artifact watcher", logger.Fields{"error": err, "scan_id": scanID})
		return
	}
	defer watcher.Close()

	if err := watcher.Add(scanDir); err != nil {
		s.logger.Error("Error adding directory to watcher", logger.Fields{"error": err, "dir": scanDir, "scan_id": scanID})
		return
	}

	s.updateArtifacts(scanID, scanDir)

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

			// Check if it's an artifact file (screenshots, nmap, or ffuf)
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
				s.updateArtifacts(scanID, scanDir)
				updatePending = false
			}
			mu.Unlock()

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			s.logger.Error("Artifact watcher error", logger.Fields{"error": err, "dir": scanDir, "scan_id": scanID})

		case <-ctx.Done():
			s.logger.Info("Stopping artifact monitor, performing final update", logger.Fields{"dir": scanDir, "scan_id": scanID})
			s.updateArtifacts(scanID, scanDir)
			return
		}
	}
}

func (s *scanService) monitorSubdomains(scanID, scanDir string, ctx context.Context) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		s.logger.Error("Failed to create subdomain watcher", logger.Fields{"error": err, "scan_id": scanID})
		return
	}
	defer watcher.Close()

	httpxPath := filepath.Join(scanDir, "httpx_output.txt")

	// Wait for httpx_output.txt to be created (max 5 minutes)
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	fileExists := false
	for !fileExists {
		select {
		case <-timeout:
			s.logger.Warn("Timeout waiting for httpx_output.txt", logger.Fields{"scan_id": scanID})
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
		s.logger.Error("Error adding httpx_output.txt to watcher", logger.Fields{"error": err, "file": httpxPath, "scan_id": scanID})
		return
	}

	s.logger.Info("Started monitoring subdomain discovery", logger.Fields{"scan_id": scanID, "file": httpxPath})

	var lastSize int64
	var mu sync.Mutex

	// Initial processing
	s.processSubdomainUpdate(scanID, httpxPath, &lastSize)

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
				s.processSubdomainUpdate(scanID, httpxPath, &lastSize)
				updatePending = false
			}
			mu.Unlock()

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			s.logger.Error("Subdomain watcher error", logger.Fields{"error": err, "file": httpxPath, "scan_id": scanID})

		case <-ctx.Done():
			s.logger.Info("Stopping subdomain monitor, performing final update", logger.Fields{"file": httpxPath, "scan_id": scanID})
			s.processSubdomainUpdate(scanID, httpxPath, &lastSize)
			return
		}
	}
}

func (s *scanService) processSubdomainUpdate(scanID, filePath string, lastSize *int64) {
	file, err := os.Open(filePath)
	if err != nil {
		s.logger.Error("Failed to open httpx_output.txt", logger.Fields{"error": err, "file": filePath, "scan_id": scanID})
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		s.logger.Error("Failed to stat httpx_output.txt", logger.Fields{"error": err, "file": filePath, "scan_id": scanID})
		return
	}

	currentSize := stat.Size()
	if currentSize <= *lastSize {
		return
	}

	if _, err := file.Seek(*lastSize, 0); err != nil {
		s.logger.Error("Failed to seek httpx_output.txt", logger.Fields{"error": err, "file": filePath, "scan_id": scanID})
		return
	}

	newContent := make([]byte, currentSize-*lastSize)
	n, err := file.Read(newContent)
	if err != nil {
		s.logger.Error("Failed to read new content from httpx_output.txt", logger.Fields{"error": err, "file": filePath, "scan_id": scanID})
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
		// Lock this specific scan to prevent race conditions with artifact monitoring
		mu := s.getScanMutex(scanID)
		mu.Lock()
		defer mu.Unlock()

		scan, err := s.scanDao.GetScanByUUID(scanID)
		if err != nil {
			s.logger.Error("Failed to load scan for subdomain update", logger.Fields{"error": err, "scan_id": scanID})
			return
		}

		// Add new subdomains
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

		if err := s.scanDao.UpdateScan(scan); err != nil {
			s.logger.Error("Failed to update scan with new subdomains", logger.Fields{"error": err, "scan_id": scanID})
			return
		}

		s.logger.Info("Added new subdomains", logger.Fields{
			"scan_id": scanID,
			"count":   len(validLines),
			"total":   len(scan.Subdomains),
		})
	}

	*lastSize = currentSize
}

func (s *scanService) updateArtifacts(scanID, scanDir string) {
	// Lock this specific scan to prevent race conditions with subdomain monitoring
	mu := s.getScanMutex(scanID)
	mu.Lock()
	defer mu.Unlock()

	scan, err := s.scanDao.GetScanByUUID(scanID)
	if err != nil {
		s.logger.Error("Failed to load scan for artifact update", logger.Fields{"error": err, "scan_id": scanID})
		return
	}

	if err := s.saveScreenShotPaths(scan, scanDir); err != nil {
		s.logger.Error("Failed to update screenshot paths", logger.Fields{"error": err, "scan_id": scanID})
	}

	if err := s.saveArtifactPaths(scan, scanDir); err != nil {
		s.logger.Error("Failed to update artifact paths", logger.Fields{"error": err, "scan_id": scanID})
	}

	if err := s.scanDao.UpdateScan(scan); err != nil {
		s.logger.Error("Failed to persist artifact update", logger.Fields{"error": err, "scan_id": scanID})
		return
	}

	s.logger.Info("Updated artifact paths", logger.Fields{"scan_id": scanID})
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

	for i := range scan.Subdomains {

		domainName := strings.TrimPrefix(scan.Subdomains[i].Domain, "https://")
		domainName = strings.TrimPrefix(domainName, "http://")

		for _, screenshotPath := range paths {
			filename := filepath.Base(screenshotPath)
			filenameWithoutExt := strings.TrimSuffix(filename, filepath.Ext(filename))

			if strings.Contains(filenameWithoutExt, domainName) || strings.Contains(domainName, filenameWithoutExt) {
				scan.Subdomains[i].Screenshot = screenshotPath
				s.logger.Debug("Mapped screenshot to subdomain", logger.Fields{
					"subdomain":  scan.Subdomains[i].Domain,
					"screenshot": screenshotPath,
				})
				break // Found a match, move to next subdomain
			}
		}
	}

	encoded, err := json.Marshal(paths)
	if err != nil {
		s.logger.Error("Failed to encode screenshot paths", logger.Fields{"error": err, "scan_dir": scanDir})
		return err
	}

	scan.ScreenshotsPath = string(encoded)
	return nil
}

func (s *scanService) saveArtifactPaths(scan *models.Scan, scanDir string) error {
	if scanDir == "" {
		s.logger.Warn("Scan directory not provided for artifact persistence", logger.Fields{"scan_id": scan.UUID})
		return nil
	}

	nmapPath := filepath.Join(scanDir, "nmap_output.xml")
	if _, err := os.Stat(nmapPath); err == nil {
		s.logger.Info("Found nmap output, parsing...", logger.Fields{"scan_id": scan.UUID, "file": nmapPath})

		nmapParser := parsers.NewNmapParser()
		result, err := nmapParser.Parse(nmapPath)
		if err != nil {
			s.logger.Error("Failed to parse nmap output", logger.Fields{"error": err, "file": nmapPath})
		} else {
			if hosts, ok := result["hosts"].([]map[string]any); ok {
				s.logger.Info("Processing nmap hosts", logger.Fields{"scan_id": scan.UUID, "host_count": len(hosts)})

				for _, host := range hosts {
					hostnames, hasHostnames := host["hostnames"].([]parsers.Hostname)
					if !hasHostnames || len(hostnames) == 0 {
						s.logger.Warn("Host has no hostnames, skipping", logger.Fields{"scan_id": scan.UUID})
						continue
					}

					for _, hostname := range hostnames {
						nmapDomain := fmt.Sprintf("https://%s", hostname.Name)

						s.logger.Debug("Looking for nmap hostname match", logger.Fields{
							"nmap_hostname": hostname.Name,
							"nmap_domain":   nmapDomain,
							"scan_id":       scan.UUID,
						})
						if hostname.Type != "user" {
							s.logger.Debug("Skipping non-user hostname", logger.Fields{
								"nmap_hostname": hostname.Name,
								"hostname_type": hostname.Type,
								"scan_id":       scan.UUID,
							})
							continue
						}
						for i := range scan.Subdomains {
							if scan.Subdomains[i].Domain == nmapDomain {
								if ports, ok := host["ports"].([]parsers.Port); ok {
									var openPorts []string
									for _, port := range ports {
										if port.State.State == "open" {
											portInfo := fmt.Sprintf("%s/%s (%s)", port.PortID, port.Protocol, port.Service.Name)
											openPorts = append(openPorts, portInfo)
										}
									}
									if len(openPorts) > 0 {
										scan.Subdomains[i].OpenPorts = openPorts
										s.logger.Info("Set nmap results for subdomain", logger.Fields{
											"subdomain": scan.Subdomains[i].Domain,
											"ports":     len(openPorts),
										})
									}
								}
								break
							}
						}
					}
				}
			}

		}
	}

	ffufMatches, err := filepath.Glob(filepath.Join(scanDir, "*_ffuf_output.json"))
	if err != nil {
		s.logger.Error("Failed to glob ffuf files", logger.Fields{"error": err, "scan_dir": scanDir})
	}

	for _, ffufPath := range ffufMatches {
		s.logger.Info("Found ffuf output, parsing...", logger.Fields{"scan_id": scan.UUID, "file": ffufPath})

		ffufParser := parsers.NewFuffParser()
		result, err := ffufParser.Parse(ffufPath)
		if err != nil {
			s.logger.Error("Failed to parse ffuf output", logger.Fields{"error": err, "file": ffufPath})
			continue
		}

		if results, ok := result["results"].([]parsers.FuffResult); ok {
			filename := filepath.Base(ffufPath)
			s.logger.Info("Successfully parsed ffuf output", logger.Fields{
				"file":          filename,
				"total_results": len(results),
			})

			var patternsFile string
			if scan.SensitivePatterns != "" {
				tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("patterns_%s.txt", scan.UUID))
				if err := os.WriteFile(tmpFile, []byte(scan.SensitivePatterns), 0644); err != nil {
					s.logger.WithError(err).Warn("Failed to write temp patterns file")
				} else {
					patternsFile = tmpFile
					defer os.Remove(tmpFile)
				}
			}

			for i := range scan.Subdomains {
				domainClean := strings.Replace(scan.Subdomains[i].Domain, "://", ".", -1)
				domainClean = strings.Replace(domainClean, "https.", "", -1)
				domainClean = strings.Replace(domainClean, "http.", "", -1)

				if strings.HasPrefix(filename, domainClean+"_") {
					addedCount := 0
					sensitiveCount := 0
					for _, r := range results {
						if r.Status >= 200 && r.Status < 400 {
							pathInfo := fmt.Sprintf("%s [%d]", r.URL, r.Status)

							found := false
							for _, existing := range scan.Subdomains[i].DirFuzzing {
								if existing == pathInfo {
									found = true
									break
								}
							}
							if !found {
								scan.Subdomains[i].DirFuzzing = append(scan.Subdomains[i].DirFuzzing, pathInfo)
								addedCount++

								if sensitivePattern, found := parsers.DetectSensitivePattern(r.URL, patternsFile); found {
									sensitiveCount++
									s.logger.Warn("Sensitive endpoint detected!", logger.Fields{
										"url":         r.URL,
										"status":      r.Status,
										"severity":    sensitivePattern.Severity,
										"description": sensitivePattern.Description,
										"category":    sensitivePattern.Category,
									})

									if s.notificationClient != nil {
										emoji := parsers.GetSeverityEmoji(sensitivePattern.Severity)
										msg := notification.Message{
											Title:       fmt.Sprintf("%s Sensitive Endpoint Found!", emoji),
											Description: fmt.Sprintf("**%s**\n`%s` [%d]", sensitivePattern.Description, r.URL, r.Status),
											Severity:    sensitivePattern.Severity,
											Fields: map[string]string{
												"Category": sensitivePattern.Category,
												"Pattern":  sensitivePattern.Pattern,
												"Domain":   scan.Subdomains[i].Domain,
												"Status":   fmt.Sprintf("%d", r.Status),
											},
										}
										if err := s.notificationClient.Send(msg); err != nil {
											s.logger.WithError(err).Error("Failed to send sensitive finding notification")
										}
									}
								}
							}
						}
					}
					s.logger.Info("Added ffuf results to subdomain", logger.Fields{
						"subdomain": scan.Subdomains[i].Domain,
						"added":     addedCount,
						"sensitive": sensitiveCount,
						"total":     len(scan.Subdomains[i].DirFuzzing),
					})
					break // Only match one subdomain per file
				}
			}
		}
	}

	return nil
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
