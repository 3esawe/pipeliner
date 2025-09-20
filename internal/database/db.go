package database

import (
	"fmt"
	"pipeliner/internal/config"
	"pipeliner/internal/models"

	"github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB(cfg *config.Config) {

	dns := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName)

	var err error
	DB, err = gorm.Open(postgres.Open(dns), &gorm.Config{})
	if err != nil {
		logrus.Fatalf("Failed to connect to database: %v", err)
	}

	if err := DB.AutoMigrate(&models.Scan{}); err != nil {
		logrus.Fatalf("Failed to auto-migrate database: %v", err)
	}

	logrus.Info("Database connection established and migrated")
}
