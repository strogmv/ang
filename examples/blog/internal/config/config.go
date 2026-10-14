package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	AWSRegion          string `env:"AWS_REGION" env-default:"us-east-1"`
	DatabaseURL        string `env:"DATABASE_URL" env-default:"postgres://app:app@localhost:5439/app?sslmode=disable"`
	DevMode            bool   `env:"DEV_MODE" env-default:"false"`
	EmailDryRun        bool   `env:"EMAIL_DRY_RUN" env-default:"false"`
	EmailProvider      string `env:"EMAIL_PROVIDER" env-default:"noop"`
	HTTPPort           string `env:"HTTP_PORT" env-default:"8080"`
	JWTAccessTTL       string `env:"JWT_ACCESS_TTL" env-default:"15m"`
	JWTAlg             string `env:"JWT_ALG" env-default:"HS256"`
	JWTAudience        string `env:"JWT_AUDIENCE" env-default:"ang-api"`
	JWTIssuer          string `env:"JWT_ISSUER" env-default:"ang"`
	JWTPrivateKey      string `env:"JWT_PRIVATE_KEY" env-default:"secret-key-for-tests"`
	JWTPublicKey       string `env:"JWT_PUBLIC_KEY"`
	JWTRefreshTTL      string `env:"JWT_REFRESH_TTL" env-default:"168h"`
	JWTRotation        bool   `env:"JWT_ROTATION" env-default:"true"`
	MongoDatabase      string `env:"MONGO_DATABASE" env-default:"app"`
	MongoURL           string `env:"MONGO_URL" env-default:"mongodb://localhost:27017"`
	NatsURL            string `env:"NATS_URL" env-default:"nats://localhost:4222"`
	OpenAIAPIKey       string `env:"OPENAI_API_KEY"`
	PGMaxConnIdleTime  string `env:"PG_MAX_CONN_IDLE_TIME" env-default:"30m"`
	PGMaxConnLifetime  string `env:"PG_MAX_CONN_LIFETIME" env-default:"1h"`
	PGMaxConns         int    `env:"PG_MAX_CONNS" env-default:"25"`
	PGMinConns         int    `env:"PG_MIN_CONNS" env-default:"5"`
	RedisAddr          string `env:"REDIS_ADDR" env-default:"localhost:6379"`
	S3Bucket           string `env:"S3_BUCKET" env-required:"true"`
	S3Endpoint         string `env:"S3_ENDPOINT" env-required:"true"`
	SESAccessKeyID     string `env:"SES_ACCESS_KEY_ID"`
	SESFrom            string `env:"SES_FROM"`
	SESRegion          string `env:"SES_REGION"`
	SESSecretAccessKey string `env:"SES_SECRET_ACCESS_KEY"`
	SMTPFrom           string `env:"SMTP_FROM"`
	SMTPHost           string `env:"SMTP_HOST"`
	SMTPPass           string `env:"SMTP_PASS"`
	SMTPPort           string `env:"SMTP_PORT" env-default:"587"`
	SMTPUser           string `env:"SMTP_USER"`
}

func Load() (*Config, error) {
	var cfg Config

	// Prefer a local .env file when present. When running nested entrypoints like
	// cmd/server/main.go, walk parent directories until repository root.
	envPath, err := findDotEnv()
	if err != nil {
		return nil, fmt.Errorf("config error: %w", err)
	}
	if envPath != "" {
		if err := cleanenv.ReadConfig(envPath, &cfg); err != nil {
			return nil, fmt.Errorf("config error: %w", err)
		}
	} else {
		if err := cleanenv.ReadEnv(&cfg); err != nil {
			return nil, fmt.Errorf("config error: %w", err)
		}
	}

	if err := validateJWTConfig(&cfg); err != nil {
		return nil, fmt.Errorf("config error: %w", err)
	}

	return &cfg, nil
}

func findDotEnv() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(wd, ".env")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		} else if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			return "", nil
		}
		wd = parent
	}
}

func validateJWTConfig(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("nil config")
	}
	alg := strings.ToUpper(strings.TrimSpace(cfg.JWTAlg))
	switch alg {
	case "", "HS256":
		if strings.TrimSpace(cfg.JWTPrivateKey) == "" {
			return fmt.Errorf("JWT_PRIVATE_KEY is required for HS256")
		}
	case "RS256", "ES256":
		if strings.TrimSpace(cfg.JWTPublicKey) == "" {
			return fmt.Errorf("JWT_PUBLIC_KEY is required for %s", alg)
		}
	}
	return nil
}
