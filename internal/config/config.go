package config

import (
	"os"
	"strconv"
)

type Config struct {
	DBHost             string
	DBPort             int
	DBUser             string
	DBPassword         string
	DBName             string
	MaxConcurrentScans int
}

// LoadConfig loads database config from environment variables with sensible defaults.
// Supported env vars: DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME, MAX_CONCURRENT_SCANS
func LoadConfig() *Config {
	host := getenvDefault("DB_HOST", "localhost")
	portStr := getenvDefault("DB_PORT", "5432")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		port = 5432
	}
	user := getenvDefault("DB_USER", "pipeliner")
	pass := getenvDefault("DB_PASSWORD", "pipeliner")
	name := getenvDefault("DB_NAME", "pipeliner")

	maxConcurrentStr := getenvDefault("MAX_CONCURRENT_SCANS", "1")
	maxConcurrent, err := strconv.Atoi(maxConcurrentStr)
	if err != nil || maxConcurrent < 1 {
		maxConcurrent = 1
	}

	return &Config{
		DBHost:             host,
		DBPort:             port,
		DBUser:             user,
		DBPassword:         pass,
		DBName:             name,
		MaxConcurrentScans: maxConcurrent,
	}
}

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
