package services

import (
	"errors"
	"pipeliner/internal/dao"
	"pipeliner/internal/models"
	"pipeliner/internal/notification"
	"pipeliner/pkg/logger"
	"sync"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type ScanServiceMethods interface {
	StartScan(scan *models.Scan) (string, error)
	GetScanByUUID(id string) (*models.Scan, error)
	ListScans() ([]models.Scan, error)
	ListScansWithPagination(page, limit int) ([]models.Scan, int64, error)
	DeleteScan(id string) error
}

type scanService struct {
	scanDao            dao.ScanDAO
	logger             *logger.Logger
	scanMutexes        *sync.Map
	notificationClient *notification.NotificationClient

	executor      *ScanExecutor
	monitor       *ScanMonitor
	statusManager *ScanStatusManager
	artifacts     *ArtifactProcessor
}

var ErrScanNotFound = errors.New("scan not found")

func NewScanService(scanDao dao.ScanDAO) ScanServiceMethods {
	notifClient, err := notification.NewNotificationClient()
	if err != nil {
		logger.NewLogger(logrus.InfoLevel).WithError(err).Warn("Failed to initialize notification client - notifications disabled")
	}

	log := logger.NewLogger(logrus.InfoLevel)
	scanMutexes := &sync.Map{}

	svc := &scanService{
		scanDao:            scanDao,
		logger:             log,
		scanMutexes:        scanMutexes,
		notificationClient: notifClient,
	}

	svc.statusManager = newScanStatusManager(scanDao, log)
	svc.artifacts = newArtifactProcessor(scanDao, log, svc.scanMutexes, notifClient)
	svc.monitor = newScanMonitor(scanDao, log, svc.scanMutexes, svc.artifacts)
	svc.executor = newScanExecutor(svc)

	return svc
}

func (s *scanService) StartScan(scan *models.Scan) (string, error) {
	id := uuid.New().String()
	scan.UUID = id
	scan.Status = "queued"

	if err := s.scanDao.SaveScan(scan); err != nil {
		s.logger.Error("SaveScan failed", logger.Fields{"error": err})
		return "", err
	}

	go s.startScanExecution(scan)

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

func (s *scanService) ListScansWithPagination(page, limit int) ([]models.Scan, int64, error) {
	return s.scanDao.ListScansWithPagination(page, limit)
}

func (s *scanService) DeleteScan(id string) error {
	return s.scanDao.DeleteScan(id)
}
