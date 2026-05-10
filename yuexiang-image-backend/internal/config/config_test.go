package config

import (
	"strings"
	"testing"
	"time"
)

func TestValidateProductionRequiresReliableQueue(t *testing.T) {
	cfg := validProductionConfig()
	cfg.QueueDriver = "inline"

	err := cfg.ValidateProduction()
	if err == nil || !strings.Contains(err.Error(), "QUEUE_DRIVER=redis") {
		t.Fatalf("expected production queue validation error, got %v", err)
	}
}

func TestValidateProductionRequiresDeadLetterStream(t *testing.T) {
	cfg := validProductionConfig()
	cfg.QueueDeadLetterStream = ""

	err := cfg.ValidateProduction()
	if err == nil || !strings.Contains(err.Error(), "QUEUE_DEAD_LETTER_STREAM") {
		t.Fatalf("expected dead-letter validation error, got %v", err)
	}
}

func TestValidateProductionAcceptsCommercialStack(t *testing.T) {
	cfg := validProductionConfig()
	if err := cfg.ValidateProduction(); err != nil {
		t.Fatalf("expected valid production config, got %v", err)
	}
}

func TestValidateProductionRejectsDefaultInfrastructureSecrets(t *testing.T) {
	cfg := validProductionConfig()
	cfg.S3AccessKey = "minioadmin"
	cfg.S3SecretKey = "minioadmin"

	err := cfg.ValidateProduction()
	if err == nil || !strings.Contains(err.Error(), "S3_ACCESS_KEY") {
		t.Fatalf("expected S3 default credential validation error, got %v", err)
	}
}

func TestValidateProductionRejectsMissingSMTPAndIFPay(t *testing.T) {
	cfg := validProductionConfig()
	cfg.SMTPHost = ""

	err := cfg.ValidateProduction()
	if err == nil || !strings.Contains(err.Error(), "SMTP_HOST") {
		t.Fatalf("expected SMTP validation error, got %v", err)
	}

	cfg = validProductionConfig()
	cfg.IFPayPrivateKeyPEM = ""
	err = cfg.ValidateProduction()
	if err == nil || !strings.Contains(err.Error(), "IFPAY_PRIVATE_KEY_PEM") {
		t.Fatalf("expected IF-Pay validation error, got %v", err)
	}
}

func TestValidateProductionRejectsDevelopmentOrigins(t *testing.T) {
	cfg := validProductionConfig()
	cfg.CORSAllowedOrigins = "http://localhost:5173"

	err := cfg.ValidateProduction()
	if err == nil || !strings.Contains(err.Error(), "CORS_ALLOWED_ORIGINS") {
		t.Fatalf("expected CORS validation error, got %v", err)
	}
}

func TestLoadDefaultsDeadLetterStreamFromQueueStream(t *testing.T) {
	t.Setenv("QUEUE_STREAM", "custom:tasks")
	t.Setenv("QUEUE_DEAD_LETTER_STREAM", "")

	cfg := Load()
	if cfg.QueueDeadLetterStream != "custom:tasks:dead" {
		t.Fatalf("expected custom dead-letter stream, got %q", cfg.QueueDeadLetterStream)
	}
}

func validProductionConfig() Config {
	return Config{
		AppEnv:                "production",
		DatabaseURL:           "postgres://user:pass@db:5432/yuexiang?sslmode=require",
		PublicBaseURL:         "https://api.example.com",
		ImagePublicBaseURL:    "https://img.example.com",
		AdminToken:            strings.Repeat("a", 24),
		JWTSecret:             strings.Repeat("b", 32),
		ImageSigningSecret:    strings.Repeat("c", 32),
		CORSAllowedOrigins:    "https://app.example.com,https://admin.example.com",
		AllowedRefererDomains: "example.com",
		StorageDriver:         "s3",
		QueueDriver:           "redis",
		RedisAddr:             "redis:6379",
		QueueStream:           "yuexiang:image:tasks",
		QueueDeadLetterStream: "yuexiang:image:tasks:dead",
		WorkerRetryLimit:      5,
		WorkerClaimIdle:       2 * time.Minute,
		S3Bucket:              "yuexiang-images",
		S3AccessKey:           "access",
		S3SecretKey:           "secret",
		SMTPHost:              "smtp.example.com",
		SMTPUsername:          "mailer",
		SMTPPassword:          "mailer-secret",
		SMTPFrom:              "Yuexiang Image <no-reply@example.com>",
		IFPayBaseURL:          "https://ifpay.example.com",
		IFPayClientID:         "ifpay-client",
		IFPayClientSecret:     "ifpay-secret",
		IFPayPrivateKeyPEM:    "-----BEGIN PRIVATE KEY-----\nexample\n-----END PRIVATE KEY-----",
		IFPayWebhookPublicKey: "-----BEGIN PUBLIC KEY-----\nexample\n-----END PUBLIC KEY-----",
		IFPayRedirectURI:      "https://app.example.com/oauth/ifpay/callback",
	}
}
