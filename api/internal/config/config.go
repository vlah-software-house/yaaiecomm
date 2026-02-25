package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port      int
	AdminPort int
	BaseURL   string
	AdminURL  string

	DatabaseURL string

	JWTSecret     string
	SessionSecret string
	TOTPIssuer    string

	StripeSecretKey    string
	StripeWebhookKey   string
	StripePublicKey    string

	MediaStorage string // "local" or "s3"
	MediaPath    string // local-only: filesystem path

	S3 S3Config

	SMTPHost string
	SMTPPort int
	SMTPFrom string

	VAT VATConfig
}

// S3Config holds settings for S3-compatible object storage (CEPH, MinIO, AWS).
type S3Config struct {
	Endpoint       string
	Region         string
	AccessKey      string
	SecretKey      string
	ForcePathStyle bool
	PublicBucket   string // product images, storefront assets
	PublicBucketURL string // base URL where public bucket is reachable
	PrivateBucket  string // exports, invoices, internal files
}

type VATConfig struct {
	SyncEnabled       bool
	SyncCron          string
	TEDBTimeout       time.Duration
	FallbackURL       string
	VIESTimeout       time.Duration
	VIESCacheTTL      time.Duration
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:      getEnvInt("PORT", 8080),
		AdminPort: getEnvInt("ADMIN_PORT", 8081),
		BaseURL:   getEnv("BASE_URL", "http://localhost:3000"),
		AdminURL:  getEnv("ADMIN_URL", "http://localhost:8081"),

		DatabaseURL: getEnv("DATABASE_URL", "postgres://forge:forgedev@localhost:5432/forgecommerce?sslmode=disable"),

		JWTSecret:     getEnv("JWT_SECRET", ""),
		SessionSecret: getEnv("SESSION_SECRET", ""),
		TOTPIssuer:    getEnv("TOTP_ISSUER", "ForgeCommerce"),

		StripeSecretKey:  getEnv("STRIPE_SECRET_KEY", ""),
		StripeWebhookKey: getEnv("STRIPE_WEBHOOK_SECRET", ""),
		StripePublicKey:  getEnv("STRIPE_PUBLIC_KEY", ""),

		MediaStorage: getEnv("MEDIA_STORAGE", "local"),
		MediaPath:    getEnv("MEDIA_PATH", "./media"),

		S3: S3Config{
			Endpoint:        getEnv("S3_ENDPOINT", ""),
			Region:          getEnv("S3_REGION", "us-east-1"),
			AccessKey:       getEnv("S3_ACCESS_KEY_ID", ""),
			SecretKey:       getEnv("S3_SECRET_ACCESS_KEY", ""),
			ForcePathStyle:  getEnvBool("S3_FORCE_PATH_STYLE", true),
			PublicBucket:    getEnv("S3_PUBLIC_BUCKET", ""),
			PublicBucketURL: getEnv("S3_PUBLIC_BUCKET_URL", ""),
			PrivateBucket:   getEnv("S3_PRIVATE_BUCKET", ""),
		},

		SMTPHost: getEnv("SMTP_HOST", "localhost"),
		SMTPPort: getEnvInt("SMTP_PORT", 1025),
		SMTPFrom: getEnv("SMTP_FROM", "store@forgecommerce.local"),

		VAT: VATConfig{
			SyncEnabled:  getEnvBool("VAT_SYNC_ENABLED", true),
			SyncCron:     getEnv("VAT_SYNC_CRON", "0 0 * * *"),
			TEDBTimeout:  getEnvDuration("VAT_TEDB_TIMEOUT", 30*time.Second),
			FallbackURL:  getEnv("VAT_EUVATRATES_FALLBACK_URL", "https://euvatrates.com/rates.json"),
			VIESTimeout:  getEnvDuration("VIES_TIMEOUT", 10*time.Second),
			VIESCacheTTL: getEnvDuration("VIES_CACHE_TTL", 24*time.Hour),
		},
	}

	if cfg.SessionSecret == "" {
		return nil, fmt.Errorf("SESSION_SECRET is required")
	}

	return cfg, nil
}

// LoadDev loads config with development defaults (no required fields).
func LoadDev() *Config {
	cfg, err := Load()
	if err != nil {
		// In dev mode, use sensible defaults for missing required fields
		return &Config{
			Port:      getEnvInt("PORT", 8080),
			AdminPort: getEnvInt("ADMIN_PORT", 8081),
			BaseURL:   getEnv("BASE_URL", "http://localhost:3000"),
			AdminURL:  getEnv("ADMIN_URL", "http://localhost:8081"),

			DatabaseURL: getEnv("DATABASE_URL", "postgres://forge:forgedev@localhost:5432/forgecommerce?sslmode=disable"),

			JWTSecret:     getEnv("JWT_SECRET", "dev-jwt-secret-do-not-use-in-production"),
			SessionSecret: getEnv("SESSION_SECRET", "dev-session-secret-do-not-use-in-production"),
			TOTPIssuer:    getEnv("TOTP_ISSUER", "ForgeCommerce"),

			StripeSecretKey:  getEnv("STRIPE_SECRET_KEY", "sk_test_fake"),
			StripeWebhookKey: getEnv("STRIPE_WEBHOOK_SECRET", "whsec_fake"),
			StripePublicKey:  getEnv("STRIPE_PUBLIC_KEY", "pk_test_fake"),

			MediaStorage: getEnv("MEDIA_STORAGE", "local"),
			MediaPath:    getEnv("MEDIA_PATH", "./media"),

			S3: S3Config{
				Endpoint:        getEnv("S3_ENDPOINT", ""),
				Region:          getEnv("S3_REGION", "us-east-1"),
				AccessKey:       getEnv("S3_ACCESS_KEY_ID", ""),
				SecretKey:       getEnv("S3_SECRET_ACCESS_KEY", ""),
				ForcePathStyle:  getEnvBool("S3_FORCE_PATH_STYLE", true),
				PublicBucket:    getEnv("S3_PUBLIC_BUCKET", ""),
				PublicBucketURL: getEnv("S3_PUBLIC_BUCKET_URL", ""),
				PrivateBucket:   getEnv("S3_PRIVATE_BUCKET", ""),
			},

			SMTPHost: getEnv("SMTP_HOST", "localhost"),
			SMTPPort: getEnvInt("SMTP_PORT", 1025),
			SMTPFrom: getEnv("SMTP_FROM", "store@forgecommerce.local"),

			VAT: VATConfig{
				SyncEnabled:  getEnvBool("VAT_SYNC_ENABLED", true),
				SyncCron:     getEnv("VAT_SYNC_CRON", "0 0 * * *"),
				TEDBTimeout:  getEnvDuration("VAT_TEDB_TIMEOUT", 30*time.Second),
				FallbackURL:  getEnv("VAT_EUVATRATES_FALLBACK_URL", "https://euvatrates.com/rates.json"),
				VIESTimeout:  getEnvDuration("VIES_TIMEOUT", 10*time.Second),
				VIESCacheTTL: getEnvDuration("VIES_CACHE_TTL", 24*time.Hour),
			},
		}
	}
	return cfg
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
