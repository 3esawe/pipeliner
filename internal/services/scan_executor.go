package services

import (
	"context"
	"fmt"
	"pipeliner/internal/models"
	"pipeliner/pkg/engine"
	"pipeliner/pkg/logger"
	"pipeliner/pkg/tools"

	"github.com/sirupsen/logrus"
)

type ScanExecutor struct {
	scanService *scanService
}

func newScanExecutor(s *scanService) *ScanExecutor {
	return &ScanExecutor{scanService: s}
}

func (e *ScanExecutor) Execute(scanID, scanType, domain string) {
	var scanLogger *logger.ScanLogger
	var scanDir string

	defer func() {
		if r := recover(); r != nil {
			panicMsg := fmt.Sprintf("panic in background scan: %v", r)
			e.scanService.logger.Error(panicMsg, logger.Fields{"scan_id": scanID, "panic": r})

			if scanLogger != nil {
				scanLogger.LogScanFailure("panic during scan execution",
					fmt.Errorf("%v", r),
					map[string]interface{}{"panic_value": r})
				scanLogger.Close()
			}

			e.scanService.statusManager.MarkFailedWithReason(scanID, panicMsg)
		}
	}()

	queue := engine.GetGlobalQueue()
	err := queue.ExecuteWithQueue(func() error {
		if err := e.scanService.statusManager.UpdateStatus(scanID, "running"); err != nil {
			e.scanService.logger.Error("Failed to update scan to running", logger.Fields{"scan_id": scanID, "error": err})
		}

		e.scanService.logger.Info("Starting scan execution", logger.Fields{"scan_id": scanID, "scan_type": scanType, "domain": domain})

		eng, err := engine.NewPiplinerEngine()
		if err != nil {
			e.scanService.logger.Error("Failed to create engine", logger.Fields{"error": err, "scan_id": scanID})
			return err
		}

		if err := eng.PrepareScan(&tools.Options{
			ScanType: scanType,
			Domain:   domain,
		}); err != nil {
			e.scanService.logger.Error("PrepareScan failed", logger.Fields{"error": err, "scan_id": scanID})
			return err
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		scanDir = eng.ScanDirectory()

		if scanDir != "" {
			var logErr error
			scanLogger, logErr = logger.NewScanLogger(scanID, scanDir, logrus.InfoLevel)
			if logErr != nil {
				e.scanService.logger.Error("Failed to create scan logger", logger.Fields{"error": logErr, "scan_id": scanID})
			} else {
				scanLogger.WithFields(logger.Fields{
					"scan_id":   scanID,
					"scan_type": scanType,
					"domain":    domain,
				}).Info("Scan logger initialized")
			}
		}

		var monitoringDone chan struct{}
		if scanDir != "" {
			monitoringDone = make(chan struct{})
			go e.scanService.monitor.MonitorScanProgress(scanID, scanType, scanDir, ctx, monitoringDone)
		} else {
			e.scanService.logger.Warn("Scan directory not available for monitoring", logger.Fields{"scan_id": scanID})
		}

		runErr := eng.RunHTTP(scanType, domain)

		cancel()

		if monitoringDone != nil {
			e.scanService.logger.Info("Waiting for monitors to complete final processing", logger.Fields{"scan_id": scanID})
			<-monitoringDone
			e.scanService.logger.Info("Monitors completed, finalizing scan status", logger.Fields{"scan_id": scanID})
		}

		if runErr != nil {
			if partialErr, ok := runErr.(*tools.PartialExecutionError); ok {
				e.scanService.logger.Warn("Scan completed with some tool failures", logger.Fields{
					"scan_id":      scanID,
					"failed_count": len(partialErr.FailedTools),
				})

				if scanLogger != nil {
					failedToolsInterface := make([]interface{}, len(partialErr.FailedTools))
					for i, t := range partialErr.FailedTools {
						failedToolsInterface[i] = fmt.Sprintf("%s: %v", t.Tool, t.Err)
					}
					scanLogger.LogScanPartialSuccess(failedToolsInterface)
					scanLogger.Close()
				}

				if err := e.scanService.statusManager.MarkCompletedWithWarnings(scanID, partialErr.FailedTools); err != nil {
					e.scanService.logger.Error("Failed to mark scan as completed with warnings", logger.Fields{"scan_id": scanID, "error": err})
				}
				return nil
			}
			return runErr
		}

		return runErr
	})

	if err != nil {
		e.scanService.logger.Error("Scan execution failed", logger.Fields{"scan_id": scanID, "error": err})

		if scanLogger != nil {
			scanLogger.LogScanFailure("scan execution error", err, map[string]interface{}{
				"scan_type": scanType,
				"domain":    domain,
			})
			scanLogger.Close()
		}

		e.scanService.statusManager.MarkFailedWithReason(scanID, fmt.Sprintf("Execution failed: %v", err))
		return
	}

	if scanLogger != nil {
		scanLogger.LogScanSuccess()
		scanLogger.Close()
	}

	e.scanService.logger.Info("Scan completed successfully", logger.Fields{"scan_id": scanID})
	if err := e.scanService.statusManager.MarkCompleted(scanID); err != nil {
		e.scanService.logger.Error("Failed to finalize scan", logger.Fields{"scan_id": scanID, "error": err})
	}
}

func (s *scanService) startScanExecution(scan *models.Scan) {
	s.executor.Execute(scan.UUID, scan.ScanType, scan.Domain)
}
