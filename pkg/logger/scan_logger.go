package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type ScanLogger struct {
	*Logger
	scanID      string
	scanDir     string
	logFile     *os.File
	errorFile   *os.File
	mu          sync.Mutex
	multiWriter io.Writer
}

func NewScanLogger(scanID, scanDir string, level logrus.Level) (*ScanLogger, error) {
	baseLogger := NewLogger(level)

	logFilePath := filepath.Join(scanDir, "scan.log")
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to create scan log file: %w", err)
	}

	errorFilePath := filepath.Join(scanDir, "error.log")
	errorFile, err := os.OpenFile(errorFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		logFile.Close()
		return nil, fmt.Errorf("failed to create error log file: %w", err)
	}

	header := fmt.Sprintf("\n=== Scan Log Started: %s ===\n", time.Now().Format(time.RFC3339))
	header += fmt.Sprintf("Scan ID: %s\n", scanID)
	header += fmt.Sprintf("Scan Directory: %s\n", scanDir)
	header += "==========================================\n\n"
	logFile.WriteString(header)

	multiWriter := io.MultiWriter(os.Stdout, logFile)
	baseLogger.Logger.SetOutput(multiWriter)

	scanLogger := &ScanLogger{
		Logger:      baseLogger,
		scanID:      scanID,
		scanDir:     scanDir,
		logFile:     logFile,
		errorFile:   errorFile,
		multiWriter: multiWriter,
	}

	return scanLogger, nil
}

func (sl *ScanLogger) LogError(component string, err error, fields Fields) {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	if fields == nil {
		fields = Fields{}
	}
	fields["component"] = component
	fields["scan_id"] = sl.scanID

	sl.WithFields(fields).WithError(err).Error("Error occurred")

	errorMsg := fmt.Sprintf("[%s] [%s] Error in %s: %v\n",
		time.Now().Format(time.RFC3339),
		sl.scanID,
		component,
		err,
	)
	if len(fields) > 0 {
		errorMsg += fmt.Sprintf("  Fields: %+v\n", fields)
	}
	sl.errorFile.WriteString(errorMsg)
}

func (sl *ScanLogger) LogToolOutput(toolName, outputType string, output string) {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	timestamp := time.Now().Format(time.RFC3339)
	header := fmt.Sprintf("\n--- [%s] Tool: %s (%s) ---\n", timestamp, toolName, outputType)
	footer := fmt.Sprintf("--- End %s ---\n\n", toolName)

	message := header + output + "\n" + footer

	sl.logFile.WriteString(message)

	sl.WithFields(Fields{
		"tool":        toolName,
		"output_type": outputType,
		"scan_id":     sl.scanID,
	}).Debug("Tool output captured")
}

func (sl *ScanLogger) LogScanFailure(reason string, err error, additionalInfo map[string]interface{}) {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	timestamp := time.Now().Format(time.RFC3339)
	failureMsg := fmt.Sprintf("\n=== SCAN FAILED: %s ===\n", timestamp)
	failureMsg += fmt.Sprintf("Scan ID: %s\n", sl.scanID)
	failureMsg += fmt.Sprintf("Reason: %s\n", reason)
	if err != nil {
		failureMsg += fmt.Sprintf("Error: %v\n", err)
	}
	if len(additionalInfo) > 0 {
		failureMsg += fmt.Sprintf("Additional Info: %+v\n", additionalInfo)
	}
	failureMsg += "=====================================\n\n"

	sl.logFile.WriteString(failureMsg)
	sl.errorFile.WriteString(failureMsg)

	fields := Fields{
		"scan_id": sl.scanID,
		"reason":  reason,
	}
	for k, v := range additionalInfo {
		fields[k] = v
	}
	if err != nil {
		sl.WithFields(fields).WithError(err).Error("Scan failed")
	} else {
		sl.WithFields(fields).Error("Scan failed")
	}
}

func (sl *ScanLogger) LogScanSuccess() {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	timestamp := time.Now().Format(time.RFC3339)
	successMsg := fmt.Sprintf("\n=== SCAN COMPLETED SUCCESSFULLY: %s ===\n", timestamp)
	successMsg += fmt.Sprintf("Scan ID: %s\n", sl.scanID)
	successMsg += "=========================================\n\n"

	sl.logFile.WriteString(successMsg)

	sl.WithFields(Fields{
		"scan_id": sl.scanID,
	}).Info("Scan completed successfully")
}

func (sl *ScanLogger) LogScanPartialSuccess(failedTools []interface{}) {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	timestamp := time.Now().Format(time.RFC3339)
	warningMsg := fmt.Sprintf("\n=== SCAN COMPLETED WITH WARNINGS: %s ===\n", timestamp)
	warningMsg += fmt.Sprintf("Scan ID: %s\n", sl.scanID)
	warningMsg += fmt.Sprintf("Failed Tools (%d):\n", len(failedTools))
	for _, toolErr := range failedTools {
		warningMsg += fmt.Sprintf("  - %v\n", toolErr)
	}
	warningMsg += "Most tools completed successfully.\n"
	warningMsg += "Check individual tool logs for more details.\n"
	warningMsg += "==============================================\n\n"

	sl.logFile.WriteString(warningMsg)
	sl.errorFile.WriteString(warningMsg)

	sl.WithFields(Fields{
		"scan_id":      sl.scanID,
		"failed_count": len(failedTools),
	}).Warn("Scan completed with some tool failures")
}

func (sl *ScanLogger) Close() error {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	var errs []error

	if sl.logFile != nil {
		footer := fmt.Sprintf("\n=== Scan Log Ended: %s ===\n", time.Now().Format(time.RFC3339))
		sl.logFile.WriteString(footer)

		if err := sl.logFile.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close log file: %w", err))
		}
	}

	if sl.errorFile != nil {
		if err := sl.errorFile.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close error file: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing scan logger: %v", errs)
	}

	return nil
}

func (sl *ScanLogger) GetLogFilePath() string {
	return filepath.Join(sl.scanDir, "scan.log")
}

func (sl *ScanLogger) GetErrorLogFilePath() string {
	return filepath.Join(sl.scanDir, "error.log")
}
