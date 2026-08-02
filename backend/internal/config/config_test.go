package config

import (
	"strings"
	"testing"
)

var configEnvKeys = []string{
	"ENV",
	"PORT",
	"DATABASE_URL",
	"REDIS_URL",
	"DB_POOL_MAX_CONNS",
	"DB_POOL_MIN_CONNS",
	"DB_POOL_MAX_LIFETIME",
	"DB_POOL_MAX_IDLE_TIME",
	"DB_POOL_HEALTH_CHECK_PERIOD",
	"MEDIA_BASE_URL",
	"PUBLIC_BASE_URL",
	"CORS_ALLOWED_ORIGINS",
	"JWT_SECRET",
	"DATA_ENCRYPTION_KEY",
	"OTP_CHANNEL",
	"TELEGRAM_GATEWAY_TOKEN",
	"TELEGRAM_GATEWAY_URL",
	"CLIENT_IP_ASSERTION_SECRET",
	"TELEGRAM_BOT_TOKEN",
	"TELEGRAM_BOT_API_BASE_URL",
	"TELEGRAM_BOT_USERNAME",
	"TELEGRAM_BOT_MODE",
	"TELEGRAM_WEBHOOK_SECRET",
	"TELEGRAM_QUIZ_WINNER_STICKER",
	"OPS_ADMIN_TOKEN",
}

func isolateConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range configEnvKeys {
		t.Setenv(key, "")
	}
}

func TestLoadDefaults(t *testing.T) {
	isolateConfigEnv(t)
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
	if cfg.DBPoolMaxConns != 20 || cfg.DBPoolMinConns != 2 {
		t.Errorf("unexpected DB pool defaults: min=%d max=%d", cfg.DBPoolMinConns, cfg.DBPoolMaxConns)
	}
	if len(cfg.CORSOrigins) != 1 || cfg.CORSOrigins[0] != "http://localhost:3000" {
		t.Errorf("unexpected CORS defaults: %v", cfg.CORSOrigins)
	}
	if cfg.JWTSecret != "dev-secret-change-me" || cfg.OTPChannel != "sandbox" {
		t.Errorf("unexpected auth defaults: %+v", cfg)
	}
	if cfg.TelegramGatewayToken != "" || cfg.TelegramGatewayURL != "https://gatewayapi.telegram.org" {
		t.Errorf("unexpected telegram defaults: %+v", cfg)
	}
	if cfg.TelegramBotMode != "off" || cfg.TelegramBotAPIBaseURL != "https://api.telegram.org" {
		t.Errorf("unexpected telegram bot defaults: %+v", cfg)
	}
	if cfg.TelegramBotToken != "" || cfg.TelegramBotUsername != "" || cfg.TelegramWebhookSecret != "" {
		t.Errorf("unexpected telegram bot secrets: %+v", cfg)
	}
	if cfg.TelegramQuizWinnerSticker != "" {
		t.Errorf("TelegramQuizWinnerSticker default = %q, want empty (no default file_id — an unverified one would fail every finished game)", cfg.TelegramQuizWinnerSticker)
	}
}

// The winner sticker is decoration only, so unlike the other TELEGRAM_BOT_*
// settings it must never gate startup — it just needs to reach the config
// struct when an operator does set it.
func TestLoadTelegramQuizWinnerStickerOverride(t *testing.T) {
	isolateConfigEnv(t)
	t.Setenv("TELEGRAM_QUIZ_WINNER_STICKER", "CAACAgIAAxkBAA_sticker_file_id")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.TelegramQuizWinnerSticker != "CAACAgIAAxkBAA_sticker_file_id" {
		t.Errorf("TelegramQuizWinnerSticker = %q, want the configured file_id", cfg.TelegramQuizWinnerSticker)
	}
}

func TestLoadAuthOverrides(t *testing.T) {
	isolateConfigEnv(t)
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

// TestDataKeyFallsBackToJWTSecret pins the compatibility contract that lets
// DATA_ENCRYPTION_KEY ship without a migration: unset, it must resolve to
// exactly JWT_SECRET, because that is what sealed every admin TOTP secret,
// stored PAN and Telegram credential already in the database.
func TestDataKeyFallsBackToJWTSecret(t *testing.T) {
	isolateConfigEnv(t)
	t.Setenv("JWT_SECRET", "jwt-signing-secret-at-least-32-bytes")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DataEncryptionKey != "" {
		t.Errorf("DataEncryptionKey = %q, want empty by default", cfg.DataEncryptionKey)
	}
	if cfg.DataKey() != cfg.JWTSecret {
		t.Errorf("DataKey() = %q, want the JWT secret %q", cfg.DataKey(), cfg.JWTSecret)
	}

	// Set, it takes over — and JWT_SECRET is then free to rotate without
	// touching anything encrypted at rest.
	t.Setenv("DATA_ENCRYPTION_KEY", "dedicated-data-encryption-key-32-bytes")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DataKey() != "dedicated-data-encryption-key-32-bytes" {
		t.Errorf("DataKey() = %q, want the dedicated key", cfg.DataKey())
	}
	if cfg.DataKey() == cfg.JWTSecret {
		t.Error("DataKey() must not follow JWT_SECRET once a dedicated key is set")
	}

	// Whitespace is not a key: it must not silently displace the fallback and
	// re-key everything to a blank-ish value.
	if got := ResolveDataKey("   ", "jwt-secret"); got != "jwt-secret" {
		t.Errorf("ResolveDataKey(blank) = %q, want the JWT secret", got)
	}
}

func TestLoadOverridesAndInvalidPort(t *testing.T) {
	isolateConfigEnv(t)
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

func TestLoadValidation(t *testing.T) {
	validSecret := strings.Repeat("s", minJWTSecretLen)
	validAssertionSecret := strings.Repeat("i", minClientIPAssertionSecretLen)
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{
			name: "development defaults remain valid",
		},
		{
			name: "development permits a short custom JWT secret",
			env: map[string]string{
				"JWT_SECRET": "local-only",
			},
		},
		{
			name: "production rejects legacy shared ops token",
			env: map[string]string{
				"ENV":             "prod",
				"OPS_ADMIN_TOKEN": "legacy-shared-secret",
			},
			wantErr: "OPS_ADMIN_TOKEN is disabled when ENV=prod",
		},
		{
			name: "invalid environment",
			env: map[string]string{
				"ENV": "qa",
			},
			wantErr: `invalid ENV "qa"`,
		},
		{
			name: "unknown OTP channel",
			env: map[string]string{
				"OTP_CHANNEL": "email",
			},
			wantErr: `invalid OTP_CHANNEL "email"`,
		},
		{
			name: "unimplemented SMS channel",
			env: map[string]string{
				"OTP_CHANNEL": "sms",
			},
			wantErr: `invalid OTP_CHANNEL "sms": no sender implementation`,
		},
		{
			name: "telegram requires its sender token",
			env: map[string]string{
				"OTP_CHANNEL": "telegram",
			},
			wantErr: "requires TELEGRAM_GATEWAY_TOKEN",
		},
		{
			name: "telegram rejects a whitespace token",
			env: map[string]string{
				"OTP_CHANNEL":            "telegram",
				"TELEGRAM_GATEWAY_TOKEN": "   ",
			},
			wantErr: "requires TELEGRAM_GATEWAY_TOKEN",
		},
		{
			name: "development telegram config is valid with a token",
			env: map[string]string{
				"OTP_CHANNEL":            "telegram",
				"TELEGRAM_GATEWAY_TOKEN": "token",
			},
		},
		{
			// Rejected at every ENV: a weak at-rest key cannot be strengthened
			// later without losing every TOTP secret and stored PAN it sealed.
			name: "a short data encryption key is rejected in development",
			env: map[string]string{
				"DATA_ENCRYPTION_KEY": "too-short",
			},
			wantErr: "DATA_ENCRYPTION_KEY must be at least 32 bytes",
		},
		{
			name: "a strong data encryption key is accepted",
			env: map[string]string{
				"DATA_ENCRYPTION_KEY": validSecret,
			},
		},
		{
			name: "production rejects a short data encryption key",
			env: map[string]string{
				"ENV":                        "prod",
				"JWT_SECRET":                 validSecret,
				"DATA_ENCRYPTION_KEY":        "too-short",
				"OTP_CHANNEL":                "telegram",
				"TELEGRAM_GATEWAY_TOKEN":     "token",
				"CLIENT_IP_ASSERTION_SECRET": validAssertionSecret,
				"PUBLIC_BASE_URL":            "https://avtotest.uz",
			},
			wantErr: "DATA_ENCRYPTION_KEY must be at least 32 bytes",
		},
		{
			name: "development rejects a short client IP assertion secret",
			env: map[string]string{
				"CLIENT_IP_ASSERTION_SECRET": "too-short",
			},
			wantErr: "CLIENT_IP_ASSERTION_SECRET must be at least 32 bytes",
		},
		{
			name: "unknown telegram bot mode",
			env: map[string]string{
				"TELEGRAM_BOT_MODE": "polling-ish",
			},
			wantErr: `invalid TELEGRAM_BOT_MODE "polling-ish"`,
		},
		{
			name: "webhook mode requires a bot token",
			env: map[string]string{
				"TELEGRAM_BOT_MODE": "webhook",
			},
			wantErr: "TELEGRAM_BOT_MODE webhook requires TELEGRAM_BOT_TOKEN",
		},
		{
			name: "webhook mode requires a webhook secret",
			env: map[string]string{
				"TELEGRAM_BOT_MODE":     "webhook",
				"TELEGRAM_BOT_TOKEN":    "tok",
				"TELEGRAM_BOT_USERNAME": "AvtoTestBot",
			},
			wantErr: "TELEGRAM_BOT_MODE webhook requires TELEGRAM_WEBHOOK_SECRET",
		},
		{
			name: "webhook mode requires a bot username",
			env: map[string]string{
				"TELEGRAM_BOT_MODE":       "webhook",
				"TELEGRAM_BOT_TOKEN":      "tok",
				"TELEGRAM_WEBHOOK_SECRET": "s3cr3t",
			},
			wantErr: "TELEGRAM_BOT_MODE webhook requires TELEGRAM_BOT_USERNAME",
		},
		{
			name: "webhook mode with token, username and secret is valid",
			env: map[string]string{
				"TELEGRAM_BOT_MODE":       "webhook",
				"TELEGRAM_BOT_TOKEN":      "tok",
				"TELEGRAM_BOT_USERNAME":   "AvtoTestBot",
				"TELEGRAM_WEBHOOK_SECRET": "s3cr3t",
			},
		},
		{
			name: "longpoll mode requires a bot token",
			env: map[string]string{
				"TELEGRAM_BOT_MODE": "longpoll",
			},
			wantErr: "TELEGRAM_BOT_MODE longpoll requires TELEGRAM_BOT_TOKEN",
		},
		{
			name: "longpoll mode requires a bot username",
			env: map[string]string{
				"TELEGRAM_BOT_MODE":  "longpoll",
				"TELEGRAM_BOT_TOKEN": "tok",
			},
			wantErr: "TELEGRAM_BOT_MODE longpoll requires TELEGRAM_BOT_USERNAME",
		},
		{
			name: "longpoll mode is valid in dev",
			env: map[string]string{
				"TELEGRAM_BOT_MODE":     "longpoll",
				"TELEGRAM_BOT_TOKEN":    "tok",
				"TELEGRAM_BOT_USERNAME": "AvtoTestBot",
			},
		},
		{
			name: "longpoll mode is rejected in prod",
			env: map[string]string{
				"ENV":                        "prod",
				"JWT_SECRET":                 validSecret,
				"OTP_CHANNEL":                "telegram",
				"TELEGRAM_GATEWAY_TOKEN":     "token",
				"CLIENT_IP_ASSERTION_SECRET": validAssertionSecret,
				"PUBLIC_BASE_URL":            "https://avtotest.uz",
				"TELEGRAM_BOT_MODE":          "longpoll",
				"TELEGRAM_BOT_TOKEN":         "tok",
				"TELEGRAM_BOT_USERNAME":      "AvtoTestBot",
			},
			wantErr: "TELEGRAM_BOT_MODE longpoll is not allowed when ENV=prod",
		},
		{
			name: "webhook mode is valid in prod",
			env: map[string]string{
				"ENV":                        "prod",
				"JWT_SECRET":                 validSecret,
				"OTP_CHANNEL":                "telegram",
				"TELEGRAM_GATEWAY_TOKEN":     "token",
				"CLIENT_IP_ASSERTION_SECRET": validAssertionSecret,
				"PUBLIC_BASE_URL":            "https://avtotest.uz",
				"TELEGRAM_BOT_MODE":          "webhook",
				"TELEGRAM_BOT_TOKEN":         "tok",
				"TELEGRAM_BOT_USERNAME":      "AvtoTestBot",
				"TELEGRAM_WEBHOOK_SECRET":    "s3cr3t",
			},
		},
		{
			name: "staging rejects sandbox OTP",
			env: map[string]string{
				"ENV":        "staging",
				"JWT_SECRET": validSecret,
			},
			wantErr: "OTP_CHANNEL sandbox is not allowed when ENV=staging",
		},
		{
			name: "production rejects sandbox OTP",
			env: map[string]string{
				"ENV":        "prod",
				"JWT_SECRET": validSecret,
			},
			wantErr: "OTP_CHANNEL sandbox is not allowed when ENV=prod",
		},
		{
			name: "staging rejects the development JWT secret",
			env: map[string]string{
				"ENV":                    "staging",
				"OTP_CHANNEL":            "telegram",
				"TELEGRAM_GATEWAY_TOKEN": "token",
			},
			wantErr: "JWT_SECRET must not use the development default",
		},
		{
			name: "production rejects a short JWT secret",
			env: map[string]string{
				"ENV":                    "prod",
				"JWT_SECRET":             "too-short",
				"OTP_CHANNEL":            "telegram",
				"TELEGRAM_GATEWAY_TOKEN": "token",
			},
			wantErr: "JWT_SECRET must be at least 32 bytes",
		},
		{
			name: "staging accepts complete real sender config",
			env: map[string]string{
				"ENV":                        "staging",
				"JWT_SECRET":                 validSecret,
				"OTP_CHANNEL":                "telegram",
				"TELEGRAM_GATEWAY_TOKEN":     "token",
				"CLIENT_IP_ASSERTION_SECRET": validAssertionSecret,
				"PUBLIC_BASE_URL":            "https://staging.avtotest.uz",
			},
		},
		{
			name: "staging requires a client IP assertion secret",
			env: map[string]string{
				"ENV":                    "staging",
				"JWT_SECRET":             validSecret,
				"OTP_CHANNEL":            "telegram",
				"TELEGRAM_GATEWAY_TOKEN": "token",
			},
			wantErr: "CLIENT_IP_ASSERTION_SECRET is required when ENV=staging",
		},
		{
			name: "staging rejects localhost PUBLIC_BASE_URL",
			env: map[string]string{
				"ENV":                        "staging",
				"JWT_SECRET":                 validSecret,
				"OTP_CHANNEL":                "telegram",
				"TELEGRAM_GATEWAY_TOKEN":     "token",
				"CLIENT_IP_ASSERTION_SECRET": validAssertionSecret,
			},
			wantErr: "PUBLIC_BASE_URL must be a real public origin",
		},
		{
			name: "production accepts complete real sender config",
			env: map[string]string{
				"ENV":                        "prod",
				"JWT_SECRET":                 validSecret,
				"OTP_CHANNEL":                "telegram",
				"TELEGRAM_GATEWAY_TOKEN":     "token",
				"CLIENT_IP_ASSERTION_SECRET": validAssertionSecret,
				"PUBLIC_BASE_URL":            "https://avtotest.uz",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateConfigEnv(t)
			for key, value := range tt.env {
				t.Setenv(key, value)
			}

			_, err := Load()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Load: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Load error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}
