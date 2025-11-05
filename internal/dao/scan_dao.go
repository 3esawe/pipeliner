package dao

import (
	"pipeliner/internal/models"

	"gorm.io/gorm"
)

type ScanDAO interface {
	SaveScan(scan *models.Scan) error
	GetScanByUUID(uuid string) (*models.Scan, error)
	ListScans() ([]models.Scan, error)
	UpdateScan(scan *models.Scan) error
	DeleteScan(uuid string) error
}

type scanDAO struct {
	db *gorm.DB
}

func NewScanDAO(db *gorm.DB) ScanDAO {
	return &scanDAO{db: db}
}

func (dao *scanDAO) SaveScan(scan *models.Scan) error {
	return dao.db.Create(scan).Error
}

func (dao *scanDAO) UpdateScan(scan *models.Scan) error {
	return dao.db.Save(scan).Error
}

func (dao *scanDAO) GetScanByUUID(uuid string) (*models.Scan, error) {
	var scan models.Scan
	if err := dao.db.Where("uuid = ?", uuid).First(&scan).Error; err != nil {
		return nil, err
	}
	return &scan, nil
}

func (dao *scanDAO) ListScans() ([]models.Scan, error) {
	var scans []models.Scan
	if err := dao.db.Order("created_at desc").Limit(50).Find(&scans).Error; err != nil {
		return nil, err
	}
	return scans, nil
}

func (dao *scanDAO) DeleteScan(uuid string) error {
	result := dao.db.Where("uuid = ?", uuid).Delete(&models.Scan{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
