package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"pipeliner/internal/dao"
	"pipeliner/internal/models"
	"pipeliner/internal/notification"
	"pipeliner/pkg/logger"
	"pipeliner/pkg/parsers"
	"sort"
	"strings"
	"sync"
)

type ArtifactProcessor struct {
	scanDao            dao.ScanDAO
	logger             *logger.Logger
	scanMutexes        *sync.Map
	notificationClient *notification.NotificationClient
}

func newArtifactProcessor(scanDao dao.ScanDAO, logger *logger.Logger, scanMutexes *sync.Map, notifClient *notification.NotificationClient) *ArtifactProcessor {
	return &ArtifactProcessor{
		scanDao:            scanDao,
		logger:             logger,
		scanMutexes:        scanMutexes,
		notificationClient: notifClient,
	}
}

func (a *ArtifactProcessor) getScanMutex(scanID string) *sync.Mutex {
	value, _ := a.scanMutexes.LoadOrStore(scanID, &sync.Mutex{})
	return value.(*sync.Mutex)
}

func (a *ArtifactProcessor) UpdateArtifacts(scanID, scanDir string) {
	mu := a.getScanMutex(scanID)
	mu.Lock()
	defer mu.Unlock()

	scan, err := a.scanDao.GetScanByUUID(scanID)
	if err != nil {
		a.logger.Error("Failed to load scan for artifact update", logger.Fields{"error": err, "scan_id": scanID})
		return
	}

	if err := a.saveScreenShotPaths(scan, scanDir); err != nil {
		a.logger.Error("Failed to update screenshot paths", logger.Fields{"error": err, "scan_id": scanID})
	}

	if err := a.saveArtifactPaths(scan, scanDir); err != nil {
		a.logger.Error("Failed to update artifact paths", logger.Fields{"error": err, "scan_id": scanID})
	}

	if err := a.scanDao.UpdateScan(scan); err != nil {
		a.logger.Error("Failed to persist artifact update", logger.Fields{"error": err, "scan_id": scanID})
		return
	}

	a.logger.Info("Updated artifact paths", logger.Fields{"scan_id": scanID})
}

func (a *ArtifactProcessor) saveScreenShotPaths(scan *models.Scan, scanDir string) error {
	if scanDir == "" {
		a.logger.Warn("Scan directory not provided for screenshot persistence", logger.Fields{"scan_id": scan.UUID})
		return nil
	}

	patterns := []string{"*.jpeg", "*.jpg", "*.png"}
	seen := make(map[string]struct{})
	var paths []string
	scanDirName := filepath.Base(scanDir)

	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(scanDir, pattern))
		if err != nil {
			a.logger.Error("Failed to glob screenshot files", logger.Fields{"error": err, "pattern": pattern, "scan_dir": scanDir})
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
				a.logger.Debug("Mapped screenshot to subdomain", logger.Fields{
					"subdomain":  scan.Subdomains[i].Domain,
					"screenshot": screenshotPath,
				})
				break
			}
		}
	}

	encoded, err := json.Marshal(paths)
	if err != nil {
		a.logger.Error("Failed to encode screenshot paths", logger.Fields{"error": err, "scan_dir": scanDir})
		return err
	}

	scan.ScreenshotsPath = string(encoded)
	return nil
}

func (a *ArtifactProcessor) saveArtifactPaths(scan *models.Scan, scanDir string) error {
	if scanDir == "" {
		a.logger.Warn("Scan directory not provided for artifact persistence", logger.Fields{"scan_id": scan.UUID})
		return nil
	}

	a.processNmapOutput(scan, scanDir)
	a.processFfufOutput(scan, scanDir)
	a.processNucleiOutput(scan, scanDir)

	return nil
}

func (a *ArtifactProcessor) processNmapOutput(scan *models.Scan, scanDir string) {
	nmapPath := filepath.Join(scanDir, "nmap_output.xml")
	if _, err := os.Stat(nmapPath); err != nil {
		return
	}

	a.logger.Info("Found nmap output, parsing...", logger.Fields{"scan_id": scan.UUID, "file": nmapPath})

	nmapParser := parsers.NewNmapParser()
	result, err := nmapParser.Parse(nmapPath)
	if err != nil {
		a.logger.Error("Failed to parse nmap output", logger.Fields{"error": err, "file": nmapPath})
		return
	}

	hosts, ok := result["hosts"].([]map[string]any)
	if !ok {
		return
	}

	a.logger.Info("Processing nmap hosts", logger.Fields{"scan_id": scan.UUID, "host_count": len(hosts)})

	for _, host := range hosts {
		hostnames, hasHostnames := host["hostnames"].([]parsers.Hostname)
		if !hasHostnames || len(hostnames) == 0 {
			a.logger.Warn("Host has no hostnames, skipping", logger.Fields{"scan_id": scan.UUID})
			continue
		}

		for _, hostname := range hostnames {
			nmapDomain := fmt.Sprintf("https://%s", hostname.Name)

			a.logger.Debug("Looking for nmap hostname match", logger.Fields{
				"nmap_hostname": hostname.Name,
				"nmap_domain":   nmapDomain,
				"scan_id":       scan.UUID,
			})

			if hostname.Type != "user" {
				a.logger.Debug("Skipping non-user hostname", logger.Fields{
					"nmap_hostname": hostname.Name,
					"hostname_type": hostname.Type,
					"scan_id":       scan.UUID,
				})
				continue
			}

			isLikelyFalsePositive, _ := host["likely_false_positive"].(bool)

			for i := range scan.Subdomains {
				if scan.Subdomains[i].Domain == nmapDomain {
					if ports, ok := host["ports"].([]parsers.Port); ok {
						var openPorts []string
						var suspiciousPorts []string

						for _, port := range ports {
							if port.State.State == "open" {
								portInfo := fmt.Sprintf("%s/%s (%s)", port.PortID, port.Protocol, port.Service.Name)
								if isLikelyFalsePositive {
									suspiciousPorts = append(suspiciousPorts, portInfo)
								} else {
									openPorts = append(openPorts, portInfo)
								}
							}
						}

						if len(openPorts) > 0 {
							scan.Subdomains[i].OpenPorts = openPorts
							a.logger.Info("Set nmap results for subdomain", logger.Fields{
								"subdomain": scan.Subdomains[i].Domain,
								"ports":     len(openPorts),
							})
						}

						if len(suspiciousPorts) > 0 {
							scan.Subdomains[i].PotentialFalsePorts = suspiciousPorts
							a.logger.Warn("Potential false positive ports detected (CDN/WAF)", logger.Fields{
								"subdomain":        scan.Subdomains[i].Domain,
								"suspicious_ports": len(suspiciousPorts),
							})
						}
					}
					break
				}
			}
		}
	}
}

func (a *ArtifactProcessor) processFfufOutput(scan *models.Scan, scanDir string) {
	ffufMatches, err := filepath.Glob(filepath.Join(scanDir, "*_ffuf_output.json"))
	if err != nil {
		a.logger.Error("Failed to glob ffuf files", logger.Fields{"error": err, "scan_dir": scanDir})
		return
	}

	for _, ffufPath := range ffufMatches {
		a.logger.Info("Found ffuf output, parsing...", logger.Fields{"scan_id": scan.UUID, "file": ffufPath})

		ffufParser := parsers.NewFuffParser()
		result, err := ffufParser.Parse(ffufPath)
		if err != nil {
			a.logger.Error("Failed to parse ffuf output", logger.Fields{"error": err, "file": ffufPath})
			continue
		}

		results, ok := result["results"].([]parsers.FuffResult)
		if !ok {
			continue
		}

		filename := filepath.Base(ffufPath)
		a.logger.Info("Successfully parsed ffuf output", logger.Fields{
			"file":          filename,
			"total_results": len(results),
		})

		var patternsFile string
		if scan.SensitivePatterns != "" {
			tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("patterns_%s.txt", scan.UUID))
			if err := os.WriteFile(tmpFile, []byte(scan.SensitivePatterns), 0644); err != nil {
				a.logger.WithError(err).Warn("Failed to write temp patterns file")
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
								a.logger.Warn("Sensitive endpoint detected!", logger.Fields{
									"url":         r.URL,
									"status":      r.Status,
									"severity":    sensitivePattern.Severity,
									"description": sensitivePattern.Description,
									"category":    sensitivePattern.Category,
								})

								if a.notificationClient != nil {
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
									if err := a.notificationClient.Send(msg); err != nil {
										a.logger.WithError(err).Error("Failed to send sensitive finding notification")
									}
								}
							}
						}
					}
				}
				a.logger.Info("Added ffuf results to subdomain", logger.Fields{
					"subdomain": scan.Subdomains[i].Domain,
					"added":     addedCount,
					"sensitive": sensitiveCount,
					"total":     len(scan.Subdomains[i].DirFuzzing),
				})
				break
			}
		}
	}
}

func (a *ArtifactProcessor) processNucleiOutput(scan *models.Scan, scanDir string) {
	nucleiPath := filepath.Join(scanDir, "nuclei_output.json")
	if _, err := os.Stat(nucleiPath); err != nil {
		return
	}

	a.logger.Info("Found nuclei output, parsing...", logger.Fields{"scan_id": scan.UUID, "file": nucleiPath})

	nucleiParser := parsers.NewNucleiParser()
	result, err := nucleiParser.Parse(nucleiPath)
	if err != nil {
		a.logger.Error("Failed to parse nuclei output", logger.Fields{"error": err, "file": nucleiPath})
		return
	}

	results, ok := result["results"].([]parsers.NucleiResult)
	if !ok || len(results) == 0 {
		a.logger.Info("No nuclei results found", logger.Fields{"scan_id": scan.UUID})
		return
	}

	a.logger.Info("Processing nuclei results", logger.Fields{"scan_id": scan.UUID, "result_count": len(results)})

	for _, nucleiResult := range results {
		host := nucleiResult.Host
		if host == "" {
			host = nucleiResult.URL
		}

		severity := a.getNucleiSeverity(nucleiResult.Info)
		templateName := a.getNucleiTemplateName(nucleiResult.Info)

		for i := range scan.Subdomains {
			subdomainHost := strings.TrimPrefix(scan.Subdomains[i].Domain, "https://")
			subdomainHost = strings.TrimPrefix(subdomainHost, "http://")

			if strings.Contains(host, subdomainHost) || strings.Contains(nucleiResult.URL, subdomainHost) {
				vulnEntry := fmt.Sprintf("[%s] %s - %s", strings.ToUpper(severity), templateName, nucleiResult.MatchedAt)

				found := false
				for _, existing := range scan.Subdomains[i].Vulns {
					if existing == vulnEntry {
						found = true
						break
					}
				}

				if !found {
					scan.Subdomains[i].Vulns = append(scan.Subdomains[i].Vulns, vulnEntry)
				}
				break
			}
		}
	}

	a.logger.Info("Processed nuclei results", logger.Fields{
		"scan_id":     scan.UUID,
		"total_vulns": len(results),
	})
}

func (a *ArtifactProcessor) getNucleiSeverity(info map[string]interface{}) string {
	if severity, ok := info["severity"].(string); ok {
		return strings.ToLower(severity)
	}
	return "info"
}

func (a *ArtifactProcessor) getNucleiTemplateName(info map[string]interface{}) string {
	if name, ok := info["name"].(string); ok {
		return name
	}
	return "Unknown Template"
}
