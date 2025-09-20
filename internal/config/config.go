package config

import (
	"os"
	"strconv"
)

type Config struct {
	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string
}

// LoadConfig loads database config from environment variables with sensible defaults.
// Supported env vars: DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME
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

	return &Config{
		DBHost:     host,
		DBPort:     port,
		DBUser:     user,
		DBPassword: pass,
		DBName:     name,
	}
}

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
