package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseConfigEnvFields(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.go")
	src := `package config
type Config struct {
	JWTAlg        string ` + "`env:\"JWT_ALG\" env-default:\"HS256\"`" + `
	JWTPublicKey  string ` + "`env:\"JWT_PUBLIC_KEY\"`" + `
	DatabaseURL   string ` + "`env:\"DATABASE_URL\" env-required:\"true\"`" + `
}
`
	if err := os.WriteFile(cfgPath, []byte(src), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	fields, err := parseConfigEnvFields(cfgPath)
	if err != nil {
		t.Fatalf("parse fields: %v", err)
	}
	if len(fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(fields))
	}
	found := map[string]configEnvField{}
	for _, f := range fields {
		found[f.Key] = f
	}
	if !found["DATABASE_URL"].Required {
		t.Fatalf("DATABASE_URL should be required")
	}
	if found["JWT_ALG"].Default != "HS256" {
		t.Fatalf("JWT_ALG default mismatch: %q", found["JWT_ALG"].Default)
	}
}

func TestEvaluateConfig_JWTConditional(t *testing.T) {
	t.Parallel()

	fields := []configEnvField{
		{Key: "JWT_ALG", Default: "HS256"},
		{Key: "JWT_PUBLIC_KEY"},
		{Key: "JWT_PRIVATE_KEY", Default: "secret"},
		{Key: "EMAIL_PROVIDER", Default: "noop"},
		{Key: "SMTP_HOST"},
		{Key: "SMTP_FROM"},
		{Key: "SMTP_USER"},
		{Key: "DATABASE_URL", Required: true},
	}
	getenv := func(string) string { return "" }

	missingRS, _ := evaluateConfig(fields, map[string]string{
		"DATABASE_URL": "postgres://x",
		"JWT_ALG":      "RS256",
	}, map[string]struct{}{}, getenv)
	if len(missingRS) != 1 || missingRS[0] != "JWT_PUBLIC_KEY (required for RS256)" {
		t.Fatalf("unexpected RS256 missing: %#v", missingRS)
	}

	missingHS, _ := evaluateConfig(fields, map[string]string{
		"DATABASE_URL":    "postgres://x",
		"JWT_ALG":         "HS256",
		"JWT_PRIVATE_KEY": "secret",
	}, map[string]struct{}{}, getenv)
	if len(missingHS) != 0 {
		t.Fatalf("expected no missing for HS256 with private key, got %#v", missingHS)
	}
}

func TestEvaluateConfig_EmailProviderConditional(t *testing.T) {
	t.Parallel()

	fields := []configEnvField{
		{Key: "JWT_ALG", Default: "HS256"},
		{Key: "JWT_PRIVATE_KEY", Default: "secret"},
		{Key: "EMAIL_PROVIDER", Default: "noop"},
		{Key: "SMTP_HOST"},
		{Key: "SMTP_FROM"},
		{Key: "SMTP_USER"},
	}
	getenv := func(string) string { return "" }

	missingSMTP, _ := evaluateConfig(fields, map[string]string{
		"EMAIL_PROVIDER":  "smtp",
		"JWT_PRIVATE_KEY": "secret",
	}, map[string]struct{}{}, getenv)
	if len(missingSMTP) != 2 {
		t.Fatalf("expected 2 smtp missing entries, got %#v", missingSMTP)
	}

	missingNoop, _ := evaluateConfig(fields, map[string]string{
		"EMAIL_PROVIDER": "noop",
	}, map[string]struct{}{}, getenv)
	if len(missingNoop) != 0 {
		t.Fatalf("expected no missing for noop provider, got %#v", missingNoop)
	}
}

func TestEvaluateConfig_SESProviderConditional(t *testing.T) {
	t.Parallel()

	fields := []configEnvField{
		{Key: "JWT_ALG", Default: "HS256"},
		{Key: "JWT_PRIVATE_KEY", Default: "secret"},
		{Key: "EMAIL_PROVIDER", Default: "noop"},
		{Key: "SES_REGION"},
		{Key: "SES_ACCESS_KEY_ID"},
		{Key: "SES_SECRET_ACCESS_KEY"},
		{Key: "SES_FROM"},
		{Key: "SMTP_FROM"},
	}
	getenv := func(string) string { return "" }

	missingSES, _ := evaluateConfig(fields, map[string]string{
		"EMAIL_PROVIDER":  "ses",
		"JWT_PRIVATE_KEY": "secret",
	}, map[string]struct{}{}, getenv)
	if len(missingSES) != 4 {
		t.Fatalf("expected 4 ses missing entries, got %#v", missingSES)
	}

	okSES, _ := evaluateConfig(fields, map[string]string{
		"EMAIL_PROVIDER":         "ses",
		"JWT_PRIVATE_KEY":        "secret",
		"SES_REGION":             "eu-central-1",
		"SES_ACCESS_KEY_ID":      "abc",
		"SES_SECRET_ACCESS_KEY":  "def",
		"SES_FROM":               "noreply@example.com",
	}, map[string]struct{}{}, getenv)
	if len(okSES) != 0 {
		t.Fatalf("expected no missing for ses provider, got %#v", okSES)
	}
}
