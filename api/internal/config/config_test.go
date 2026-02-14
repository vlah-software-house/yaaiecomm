package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadDev_ReturnsSensibleDefaults(t *testing.T) {
	// Unset SESSION_SECRET to force LoadDev to use fallback defaults.
	origVal := os.Getenv("SESSION_SECRET")
	os.Unsetenv("SESSION_SECRET")
	defer func() {
		if origVal != "" {
			os.Setenv("SESSION_SECRET", origVal)
		}
	}()

	cfg := LoadDev()
	if cfg == nil {
		t.Fatal("LoadDev returned nil")
	}

	if cfg.Port != 8080 {
		t.Errorf("Port: want 8080, got %d", cfg.Port)
	}
	if cfg.AdminPort != 8081 {
		t.Errorf("AdminPort: want 8081, got %d", cfg.AdminPort)
	}
	if cfg.BaseURL != "http://localhost:3000" {
		t.Errorf("BaseURL: want 'http://localhost:3000', got %q", cfg.BaseURL)
	}
	if cfg.AdminURL != "http://localhost:8081" {
		t.Errorf("AdminURL: want 'http://localhost:8081', got %q", cfg.AdminURL)
	}
	if cfg.TOTPIssuer != "ForgeCommerce" {
		t.Errorf("TOTPIssuer: want 'ForgeCommerce', got %q", cfg.TOTPIssuer)
	}
	if cfg.MediaStorage != "local" {
		t.Errorf("MediaStorage: want 'local', got %q", cfg.MediaStorage)
	}
	if cfg.MediaPath != "./media" {
		t.Errorf("MediaPath: want './media', got %q", cfg.MediaPath)
	}
	if cfg.SMTPPort != 1025 {
		t.Errorf("SMTPPort: want 1025, got %d", cfg.SMTPPort)
	}
}

func TestLoadDev_VATDefaults(t *testing.T) {
	// Unset SESSION_SECRET to force LoadDev to use fallback defaults.
	origVal := os.Getenv("SESSION_SECRET")
	os.Unsetenv("SESSION_SECRET")
	defer func() {
		if origVal != "" {
			os.Setenv("SESSION_SECRET", origVal)
		}
	}()

	cfg := LoadDev()
	if cfg == nil {
		t.Fatal("LoadDev returned nil")
	}

	vat := cfg.VAT

	if !vat.SyncEnabled {
		t.Error("VAT SyncEnabled should default to true")
	}
	if vat.SyncCron != "0 0 * * *" {
		t.Errorf("VAT SyncCron: want '0 0 * * *', got %q", vat.SyncCron)
	}
	if vat.TEDBTimeout != 30*time.Second {
		t.Errorf("VAT TEDBTimeout: want 30s, got %v", vat.TEDBTimeout)
	}
	if vat.FallbackURL != "https://euvatrates.com/rates.json" {
		t.Errorf("VAT FallbackURL: want 'https://euvatrates.com/rates.json', got %q", vat.FallbackURL)
	}
	if vat.VIESTimeout != 10*time.Second {
		t.Errorf("VAT VIESTimeout: want 10s, got %v", vat.VIESTimeout)
	}
	if vat.VIESCacheTTL != 24*time.Hour {
		t.Errorf("VAT VIESCacheTTL: want 24h, got %v", vat.VIESCacheTTL)
	}
}

func TestLoad_MissingSessionSecret(t *testing.T) {
	origVal := os.Getenv("SESSION_SECRET")
	os.Unsetenv("SESSION_SECRET")
	defer func() {
		if origVal != "" {
			os.Setenv("SESSION_SECRET", origVal)
		}
	}()

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing SESSION_SECRET, got nil")
	}
}

func TestLoad_WithSessionSecret(t *testing.T) {
	origVal := os.Getenv("SESSION_SECRET")
	os.Setenv("SESSION_SECRET", "test-session-secret")
	defer func() {
		if origVal != "" {
			os.Setenv("SESSION_SECRET", origVal)
		} else {
			os.Unsetenv("SESSION_SECRET")
		}
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SessionSecret != "test-session-secret" {
		t.Errorf("SessionSecret: want 'test-session-secret', got %q", cfg.SessionSecret)
	}
}

func TestGetEnv(t *testing.T) {
	key := "FORGECOMMERCE_TEST_ENV_VAR"
	os.Unsetenv(key)

	// Fallback when env var is not set.
	got := getEnv(key, "fallback-value")
	if got != "fallback-value" {
		t.Errorf("expected fallback, got %q", got)
	}

	// Uses env var when set.
	os.Setenv(key, "actual-value")
	defer os.Unsetenv(key)

	got = getEnv(key, "fallback-value")
	if got != "actual-value" {
		t.Errorf("expected 'actual-value', got %q", got)
	}
}

func TestGetEnvInt(t *testing.T) {
	key := "FORGECOMMERCE_TEST_INT_VAR"
	os.Unsetenv(key)

	// Fallback.
	got := getEnvInt(key, 42)
	if got != 42 {
		t.Errorf("expected fallback 42, got %d", got)
	}

	// Valid integer.
	os.Setenv(key, "100")
	defer os.Unsetenv(key)
	got = getEnvInt(key, 42)
	if got != 100 {
		t.Errorf("expected 100, got %d", got)
	}

	// Invalid integer uses fallback.
	os.Setenv(key, "not-a-number")
	got = getEnvInt(key, 42)
	if got != 42 {
		t.Errorf("expected fallback 42 for invalid int, got %d", got)
	}
}

func TestGetEnvBool(t *testing.T) {
	key := "FORGECOMMERCE_TEST_BOOL_VAR"
	os.Unsetenv(key)

	// Fallback.
	got := getEnvBool(key, true)
	if !got {
		t.Error("expected fallback true")
	}

	// Valid true.
	os.Setenv(key, "true")
	defer os.Unsetenv(key)
	got = getEnvBool(key, false)
	if !got {
		t.Error("expected true")
	}

	// Valid false.
	os.Setenv(key, "false")
	got = getEnvBool(key, true)
	if got {
		t.Error("expected false")
	}

	// Invalid uses fallback.
	os.Setenv(key, "maybe")
	got = getEnvBool(key, true)
	if !got {
		t.Error("expected fallback true for invalid bool")
	}
}

func TestGetEnvDuration(t *testing.T) {
	key := "FORGECOMMERCE_TEST_DUR_VAR"
	os.Unsetenv(key)

	// Fallback.
	got := getEnvDuration(key, 5*time.Second)
	if got != 5*time.Second {
		t.Errorf("expected fallback 5s, got %v", got)
	}

	// Valid duration.
	os.Setenv(key, "30s")
	defer os.Unsetenv(key)
	got = getEnvDuration(key, 5*time.Second)
	if got != 30*time.Second {
		t.Errorf("expected 30s, got %v", got)
	}

	// Invalid uses fallback.
	os.Setenv(key, "not-a-duration")
	got = getEnvDuration(key, 5*time.Second)
	if got != 5*time.Second {
		t.Errorf("expected fallback 5s for invalid duration, got %v", got)
	}
}
