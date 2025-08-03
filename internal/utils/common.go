package utils

import (
	"fmt"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func NewViperConfig(scanType string) *viper.Viper {
	v := viper.New()
	v.SetConfigType("yaml")
	v.AddConfigPath("./config")
	v.SetConfigName(scanType)
	log.Infof("Config search path: ./config")
	log.Infof("Config name: %s", scanType)
	if err := v.ReadInConfig(); err != nil {
		log.Errorf("Error reading config file: %v", err)
		panic(err)
	}
	log.Infof("Loaded config file: %s", v.ConfigFileUsed())
	return v
}

func CreateAndChangeScanDirectory(scanType string, domainName string) (string, error) {
	// Create directory path using the scan type and domain name
	dir := "./scans/" + scanType + "_" + domainName + "_" + time.Now().Format("2006-01-02_15-04-05")
	// Create the directory with permissions
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Errorf("Error creating scan directory: %v", err)
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Change to the new directory
	if err := os.Chdir(dir); err != nil {
		log.Errorf("Error changing to scan directory: %v", err)
		return dir, fmt.Errorf("failed to change directory: %w", err)
	}

	log.Infof("Created and changed to scan directory: %s", dir)
	return dir, nil
}
