// VERSION 2
package config

import (
	"os"
)

type Config struct {
	JWTAccessTTL  string
	JWTAlg        string
	JWTAudience   string
	JWTIssuer     string
	JWTPrivateKey string
	JWTPublicKey  string
	JWTRefreshTTL string
	SMTPFrom      string
	SMTPHost      string
	SMTPPass      string
	SMTPPort      string
	SMTPUser      string
}

func Load() (*Config, error) {
	cfg := &Config{
		JWTAccessTTL:  getEnv("JWT_ACCESS_TTL", "15m"),
		JWTAlg:        getEnv("JWT_ALG", "HS256"),
		JWTAudience:   getEnv("JWT_AUDIENCE", "ang-api"),
		JWTIssuer:     getEnv("JWT_ISSUER", "ang"),
		JWTPrivateKey: getEnvOrFileOrValue("JWT_PRIVATE_KEY", "private.pem", "secret-key-for-tests"),
		JWTPublicKey:  getEnvOrFileOrValue("JWT_PUBLIC_KEY", "public.pem", ""),
		JWTRefreshTTL: getEnv("JWT_REFRESH_TTL", "168h"),
		SMTPFrom:      getEnv("SMTP_FROM", ""),
		SMTPHost:      getEnv("SMTP_HOST", ""),
		SMTPPass:      getEnv("SMTP_PASS", ""),
		SMTPPort:      getEnv("SMTP_PORT", "587"),
		SMTPUser:      getEnv("SMTP_USER", ""),
	}

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
