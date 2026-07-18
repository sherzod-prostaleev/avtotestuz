package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want 8080", cfg.Port)
	}
	if cfg.DatabaseURL == "" || cfg.MediaBaseURL == "" || cfg.Env != "dev" {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
	if cfg.JWTSecret != "dev-secret-change-me" || cfg.OTPChannel != "sandbox" {
		t.Errorf("unexpected auth defaults: %+v", cfg)
	}
	if cfg.TelegramGatewayToken != "" || cfg.TelegramGatewayURL != "https://gatewayapi.telegram.org" {
		t.Errorf("unexpected telegram defaults: %+v", cfg)
	}
}

func TestLoadAuthOverrides(t *testing.T) {
	t.Setenv("JWT_SECRET", "s3cr3t")
	t.Setenv("OTP_CHANNEL", "telegram")
	t.Setenv("TELEGRAM_GATEWAY_TOKEN", "tok123")
	t.Setenv("TELEGRAM_GATEWAY_URL", "https://example.test")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.JWTSecret != "s3cr3t" || cfg.OTPChannel != "telegram" {
		t.Errorf("unexpected overrides: %+v", cfg)
	}
	if cfg.TelegramGatewayToken != "tok123" || cfg.TelegramGatewayURL != "https://example.test" {
		t.Errorf("unexpected telegram overrides: %+v", cfg)
	}
}

func TestLoadOverridesAndInvalidPort(t *testing.T) {
	t.Setenv("PORT", "9999")
	cfg, err := Load()
	if err != nil || cfg.Port != 9999 {
		t.Fatalf("Port = %d, err=%v, want 9999", cfg.Port, err)
	}
	t.Setenv("PORT", "abc")
	if _, err := Load(); err == nil {
		t.Fatal("want error for invalid PORT")
	}
}
