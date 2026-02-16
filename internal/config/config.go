package config

import (
	"fmt"
	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	AWSRegion     string `env:"AWS_REGION" env-default:"us-east-1"`
	DatabaseURL   string `env:"DATABASE_URL" env-default:"postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"`
	HTTPPort      string `env:"HTTP_PORT" env-default:"8080"`
	JWTAccessTTL  string `env:"JWT_ACCESS_TTL" env-default:"15m"`
	JWTAlg        string `env:"JWT_ALG" env-default:"HS256"`
	JWTAudience   string `env:"JWT_AUDIENCE" env-default:"ang-api"`
	JWTIssuer     string `env:"JWT_ISSUER" env-default:"ang"`
	JWTPrivateKey string `env:"JWT_PRIVATE_KEY" env-default:"secret-key-for-tests"`
	JWTPublicKey  string `env:"JWT_PUBLIC_KEY" env-required:"true"`
	JWTRefreshTTL string `env:"JWT_REFRESH_TTL" env-default:"168h"`
	MongoDatabase string `env:"MONGO_DATABASE" env-default:"app"`
	MongoURL      string `env:"MONGO_URL" env-default:"mongodb://localhost:27017"`
	NatsURL       string `env:"NATS_URL" env-default:"nats://localhost:4222"`
	RedisAddr     string `env:"REDIS_ADDR" env-default:"localhost:6379"`
	S3Bucket      string `env:"S3_BUCKET" env-required:"true"`
	S3Endpoint    string `env:"S3_ENDPOINT" env-required:"true"`
	SMTPFrom      string `env:"SMTP_FROM" env-required:"true"`
	SMTPHost      string `env:"SMTP_HOST" env-required:"true"`
	SMTPPass      string `env:"SMTP_PASS" env-required:"true"`
	SMTPPort      string `env:"SMTP_PORT" env-default:"587"`
	SMTPUser      string `env:"SMTP_USER" env-required:"true"`
}

func Load() (*Config, error) {
	var cfg Config

	// ReadConfig reads from ENV and optionally from a file if needed.
	// We use ReadEnv to focus strictly on Environment Variables as per current project architecture.
	err := cleanenv.ReadEnv(&cfg)
	if err != nil {
		return nil, fmt.Errorf("config error: %w", err)
	}

	return &cfg, nil
}
