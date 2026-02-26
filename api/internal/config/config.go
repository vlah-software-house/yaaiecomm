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
	AI  AIConfig
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

// AIProviderConfig holds settings for a single AI provider.
type AIProviderConfig struct {
	APIKey       string
	Model        string // default model
	ModelLight   string // fast/cheap model
	ModelContent string // content generation model
	ModelImage   string // image analysis model
	ModelTemplate string // template/structured output model
}

// AIConfig holds settings for all AI providers.
type AIConfig struct {
	OpenAI    AIProviderConfig
	Gemini    AIProviderConfig
	Mistral   AIProviderConfig
	Anthropic AIProviderConfig
}

// HasProviders returns true if at least one AI provider is configured.
func (c AIConfig) HasProviders() bool {
	return c.OpenAI.APIKey != "" || c.Gemini.APIKey != "" || c.Mistral.APIKey != "" || c.Anthropic.APIKey != ""
}

// AvailableProviders returns the names of configured providers.
func (c AIConfig) AvailableProviders() []string {
	var providers []string
	if c.OpenAI.APIKey != "" {
		providers = append(providers, "openai")
	}
	if c.Gemini.APIKey != "" {
		providers = append(providers, "gemini")
	}
	if c.Mistral.APIKey != "" {
		providers = append(providers, "mistral")
	}
	if c.Anthropic.APIKey != "" {
		providers = append(providers, "anthropic")
	}
	return providers
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

		AI: loadAIConfig(),
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

			AI: loadAIConfig(),
		}
	}
	return cfg
}

func loadAIConfig() AIConfig {
	return AIConfig{
		OpenAI: AIProviderConfig{
			APIKey:        getEnv("OPENAI_API_KEY", ""),
			Model:         getEnv("OPENAI_MODEL", "gpt-4o"),
			ModelLight:    getEnv("OPENAI_MODEL_LIGHT", "gpt-4o-mini"),
			ModelContent:  getEnv("OPENAI_MODEL_CONTENT", "gpt-4o"),
			ModelImage:    getEnv("OPENAI_MODEL_IMAGE", "gpt-4o"),
			ModelTemplate: getEnv("OPENAI_MODEL_TEMPLATE", "gpt-4o"),
		},
		Gemini: AIProviderConfig{
			APIKey:     getEnv("GEMINI_API_KEY", ""),
			Model:      getEnv("GEMINI_MODEL", "gemini-2.0-flash"),
			ModelLight: getEnv("GEMINI_MODEL_LIGHT", "gemini-2.0-flash-lite"),
			ModelImage: getEnv("GEMINI_MODEL_IMAGE", "gemini-2.0-flash"),
		},
		Mistral: AIProviderConfig{
			APIKey:     getEnv("MISTRAL_API_KEY", ""),
			Model:      getEnv("MISTRAL_MODEL", "mistral-large-latest"),
			ModelLight: getEnv("MISTRAL_MODEL_LIGHT", "mistral-small-latest"),
		},
		Anthropic: AIProviderConfig{
			APIKey:       getEnv("ANTHROPIC_API_KEY", ""),
			Model:        getEnv("ANTHROPIC_MODEL", "claude-sonnet-4-6"),
			ModelLight:   getEnv("ANTHROPIC_MODEL_LIGHT", "claude-haiku-4-5-20251001"),
			ModelContent: getEnv("ANTHROPIC_MODEL_CONTENT", "claude-sonnet-4-6"),
		},
	}
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
