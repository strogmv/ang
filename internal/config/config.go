// VERSION 2
package config

import (
	"os"
)

type Config struct {
}

func Load() (*Config, error) {
	cfg := &Config{}

	return cfg, nil
}

func getEnvOrFile(key, filePath string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	data, err := os.ReadFile(filePath)
	if err == nil {
		return string(data)
	}
	return ""
}

func getEnvOrFileOrValue(key, filePath, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	data, err := os.ReadFile(filePath)
	if err == nil {
		return string(data)
	}
	return fallback
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
