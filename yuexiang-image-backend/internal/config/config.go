package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv                string
	HTTPAddr              string
	PublicBaseURL         string
	ImagePublicBaseURL    string
	CORSAllowedOrigins    string
	AllowedRefererDomains string
	BlockedRefererDomains string
	AllowEmptyReferer     bool
	AdminToken            string
	JWTSecret             string
	ImageSigningSecret    string

	DatabaseURL           string
	RedisAddr             string
	RedisDB               int
	QueueDriver           string
	QueueStream           string
	QueueDeadLetterStream string
	QueueGroup            string
	WorkerName            string
	WorkerBatchSize       int64
	WorkerPollInterval    time.Duration
	WorkerRetryLimit      int64
	WorkerClaimIdle       time.Duration

	S3Endpoint       string
	S3Region         string
	S3Bucket         string
	S3AccessKey      string
	S3SecretKey      string
	S3ForcePathStyle bool
	StorageDriver    string

	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string

	IFPayBaseURL          string
	IFPayPartnerAppID     string
	IFPayClientID         string
	IFPayClientSecret     string
	IFPayPrivateKeyPEM    string
	IFPayPublicKeyPEM     string
	IFPayWebhookPublicKey string
	IFPayRedirectURI      string

	ModerationBlockedHashes string
}

func Load() Config {
	queueStream := env("QUEUE_STREAM", "yuexiang:image:tasks")
	return Config{
		AppEnv:                  env("APP_ENV", "development"),
		HTTPAddr:                env("HTTP_ADDR", ":8080"),
		PublicBaseURL:           env("PUBLIC_BASE_URL", "http://localhost:8080"),
		ImagePublicBaseURL:      env("IMAGE_PUBLIC_BASE_URL", "http://localhost:8080"),
		CORSAllowedOrigins:      env("CORS_ALLOWED_ORIGINS", "http://localhost:5173,http://localhost:5174"),
		AllowedRefererDomains:   env("ALLOWED_REFERER_DOMAINS", "localhost,127.0.0.1,yuexiang.com"),
		BlockedRefererDomains:   env("BLOCKED_REFERER_DOMAINS", ""),
		AllowEmptyReferer:       envBool("ALLOW_EMPTY_REFERER", true),
		AdminToken:              env("ADMIN_TOKEN", "dev-admin-token"),
		JWTSecret:               env("JWT_SECRET", "dev-change-me"),
		ImageSigningSecret:      env("IMAGE_SIGNING_SECRET", "dev-image-signing-secret"),
		DatabaseURL:             env("DATABASE_URL", ""),
		RedisAddr:               env("REDIS_ADDR", "localhost:6379"),
		RedisDB:                 envInt("REDIS_DB", 0),
		QueueDriver:             env("QUEUE_DRIVER", "inline"),
		QueueStream:             queueStream,
		QueueDeadLetterStream:   env("QUEUE_DEAD_LETTER_STREAM", queueStream+":dead"),
		QueueGroup:              env("QUEUE_GROUP", "yuexiang-image-workers"),
		WorkerName:              env("WORKER_NAME", "worker-local"),
		WorkerBatchSize:         int64(envInt("WORKER_BATCH_SIZE", 5)),
		WorkerPollInterval:      envDuration("WORKER_POLL_INTERVAL", 5*time.Second),
		WorkerRetryLimit:        int64(envInt("WORKER_RETRY_LIMIT", 5)),
		WorkerClaimIdle:         envDuration("WORKER_CLAIM_IDLE", 2*time.Minute),
		S3Endpoint:              env("S3_ENDPOINT", ""),
		S3Region:                env("S3_REGION", "auto"),
		S3Bucket:                env("S3_BUCKET", ""),
		S3AccessKey:             env("S3_ACCESS_KEY", ""),
		S3SecretKey:             env("S3_SECRET_KEY", ""),
		S3ForcePathStyle:        envBool("S3_FORCE_PATH_STYLE", true),
		StorageDriver:           env("STORAGE_DRIVER", "memory"),
		SMTPHost:                env("SMTP_HOST", ""),
		SMTPPort:                envInt("SMTP_PORT", 587),
		SMTPUsername:            env("SMTP_USERNAME", ""),
		SMTPPassword:            env("SMTP_PASSWORD", ""),
		SMTPFrom:                env("SMTP_FROM", "Yuexiang Image <no-reply@yuexiang.local>"),
		IFPayBaseURL:            env("IFPAY_BASE_URL", ""),
		IFPayPartnerAppID:       env("IFPAY_PARTNER_APP_ID", "yuexiang-image"),
		IFPayClientID:           env("IFPAY_CLIENT_ID", ""),
		IFPayClientSecret:       env("IFPAY_CLIENT_SECRET", ""),
		IFPayPrivateKeyPEM:      env("IFPAY_PRIVATE_KEY_PEM", ""),
		IFPayPublicKeyPEM:       env("IFPAY_PUBLIC_KEY_PEM", ""),
		IFPayWebhookPublicKey:   env("IFPAY_WEBHOOK_PUBLIC_KEY_PEM", ""),
		IFPayRedirectURI:        env("IFPAY_REDIRECT_URI", "http://localhost:5173/oauth/ifpay/callback"),
		ModerationBlockedHashes: env("MODERATION_BLOCKED_HASHES", ""),
	}
}

func (c Config) ValidateProduction() error {
	if c.AppEnv != "production" {
		return nil
	}
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required in production")
	}
	if containsInsecureToken(c.DatabaseURL, "change-me", "postgres:postgres") {
		return fmt.Errorf("DATABASE_URL must not contain default credentials in production")
	}
	if !isHTTPSURL(c.PublicBaseURL) || !isHTTPSURL(c.ImagePublicBaseURL) {
		return fmt.Errorf("PUBLIC_BASE_URL and IMAGE_PUBLIC_BASE_URL must be HTTPS URLs in production")
	}
	if c.JWTSecret == "" || c.JWTSecret == "dev-change-me" || len(c.JWTSecret) < 32 || containsInsecureToken(c.JWTSecret, "replace", "change-me") {
		return fmt.Errorf("JWT_SECRET must be changed and at least 32 characters in production")
	}
	if c.ImageSigningSecret == "" || c.ImageSigningSecret == "dev-image-signing-secret" || len(c.ImageSigningSecret) < 32 || containsInsecureToken(c.ImageSigningSecret, "replace", "change-me") {
		return fmt.Errorf("IMAGE_SIGNING_SECRET must be changed and at least 32 characters in production")
	}
	if strings.TrimSpace(c.CORSAllowedOrigins) == "" || strings.Contains(c.CORSAllowedOrigins, "*") {
		return fmt.Errorf("CORS_ALLOWED_ORIGINS must be explicit in production")
	}
	if containsInsecureToken(c.CORSAllowedOrigins, "localhost", "127.0.0.1", "http://") {
		return fmt.Errorf("CORS_ALLOWED_ORIGINS must use production HTTPS origins")
	}
	if strings.TrimSpace(c.AllowedRefererDomains) == "" && !c.AllowEmptyReferer {
		return fmt.Errorf("ALLOWED_REFERER_DOMAINS is required when ALLOW_EMPTY_REFERER=false")
	}
	if containsInsecureToken(c.AllowedRefererDomains, "localhost", "127.0.0.1") {
		return fmt.Errorf("ALLOWED_REFERER_DOMAINS must not include local development hosts in production")
	}
	if c.StorageDriver != "s3" {
		return fmt.Errorf("STORAGE_DRIVER=s3 is required in production")
	}
	if c.QueueDriver != "redis" {
		return fmt.Errorf("QUEUE_DRIVER=redis is required in production")
	}
	if strings.TrimSpace(c.RedisAddr) == "" {
		return fmt.Errorf("REDIS_ADDR is required when QUEUE_DRIVER=redis")
	}
	if strings.TrimSpace(c.QueueDeadLetterStream) == "" {
		return fmt.Errorf("QUEUE_DEAD_LETTER_STREAM is required when QUEUE_DRIVER=redis")
	}
	if c.WorkerRetryLimit < 1 {
		return fmt.Errorf("WORKER_RETRY_LIMIT must be at least 1")
	}
	if strings.TrimSpace(c.S3Bucket) == "" || strings.TrimSpace(c.S3AccessKey) == "" || strings.TrimSpace(c.S3SecretKey) == "" {
		return fmt.Errorf("S3_BUCKET, S3_ACCESS_KEY and S3_SECRET_KEY are required in production")
	}
	if containsInsecureToken(c.S3AccessKey, "minioadmin", "change-me", "replace") || containsInsecureToken(c.S3SecretKey, "minioadmin", "change-me", "replace") {
		return fmt.Errorf("S3_ACCESS_KEY and S3_SECRET_KEY must not use default credentials in production")
	}
	if strings.TrimSpace(c.SMTPHost) == "" || strings.TrimSpace(c.SMTPUsername) == "" || strings.TrimSpace(c.SMTPPassword) == "" || strings.TrimSpace(c.SMTPFrom) == "" {
		return fmt.Errorf("SMTP_HOST, SMTP_USERNAME, SMTP_PASSWORD and SMTP_FROM are required in production")
	}
	if containsInsecureToken(c.SMTPPassword, "replace", "change-me") {
		return fmt.Errorf("SMTP_PASSWORD must not use placeholder credentials in production")
	}
	if strings.TrimSpace(c.IFPayBaseURL) == "" || strings.TrimSpace(c.IFPayClientID) == "" || strings.TrimSpace(c.IFPayClientSecret) == "" {
		return fmt.Errorf("IFPAY_BASE_URL, IFPAY_CLIENT_ID and IFPAY_CLIENT_SECRET are required in production")
	}
	if containsInsecureToken(c.IFPayClientSecret, "replace", "change-me") {
		return fmt.Errorf("IFPAY_CLIENT_SECRET must not use placeholder credentials in production")
	}
	if !isHTTPSURL(c.IFPayBaseURL) || !isHTTPSURL(c.IFPayRedirectURI) {
		return fmt.Errorf("IFPAY_BASE_URL and IFPAY_REDIRECT_URI must be HTTPS URLs in production")
	}
	if strings.TrimSpace(c.IFPayPrivateKeyPEM) == "" || strings.TrimSpace(c.IFPayWebhookPublicKey) == "" {
		return fmt.Errorf("IFPAY_PRIVATE_KEY_PEM and IFPAY_WEBHOOK_PUBLIC_KEY_PEM are required in production")
	}
	return nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func containsInsecureToken(value string, tokens ...string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	for _, token := range tokens {
		if token != "" && strings.Contains(normalized, strings.ToLower(token)) {
			return true
		}
	}
	return false
}

func isHTTPSURL(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(normalized, "https://")
}
