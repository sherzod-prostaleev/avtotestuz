// Package config loads service configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	defaultJWTSecret              = "dev-secret-change-me"
	minJWTSecretLen               = 32
	minClientIPAssertionSecretLen = 32
)

type Config struct {
	Env          string // dev | staging | prod
	Port         int
	DatabaseURL  string
	RedisURL     string
	MediaBaseURL string // public base for image storage keys

	// PublicBaseURL is the origin users actually browse — the frontend, not
	// this API. Used to build shareable links (referral invite URLs). Without
	// it those were hardcoded to the production domain, so every invite link
	// generated in dev or staging pointed at a host that wasn't being served,
	// making the referral flow impossible to test end-to-end.
	PublicBaseURL string

	JWTSecret               string
	OTPChannel              string // sandbox | telegram
	TelegramGatewayToken    string
	TelegramGatewayURL      string
	ClientIPAssertionSecret string

	// Telegram bot (M4-06) — a different Telegram product from the OTP
	// Gateway above: the Gateway API only delivers verification codes, it
	// cannot receive updates or send arbitrary messages. See
	// docs/superpowers/specs/2026-07-25-m4-06-telegram-bot-design.md §5.2.
	TelegramBotToken      string
	TelegramBotAPIBaseURL string
	TelegramBotUsername   string // no leading '@'
	TelegramBotMode       string // off | webhook | longpoll
	TelegramWebhookSecret string

	PaymeMerchantID string // cashbox id for the checkout URL
	PaymeEnv        string // test | prod (selects which key)
	PaymeKeyProd    string // production cashbox KEY (Basic-auth password)
	PaymeTestKey    string // sandbox TEST_KEY

	ClickServiceID  string
	ClickMerchantID string
	ClickSecretKey  string

	// OpsAdminToken gates thin ops endpoints (payment provider kill-switches)
	// until the full M3 admin control center ships. Empty disables those routes.
	OpsAdminToken string
}

// PaymeKey returns the Basic-auth password (cashbox KEY) for the current
// PaymeEnv. Empty means Payme is not configured — the webhook rejects all
// calls with -32504.
func (c Config) PaymeKey() string {
	if c.PaymeEnv == "prod" {
		return c.PaymeKeyProd
	}
	return c.PaymeTestKey
}

// PaymeCheckoutHost is the base checkout host (same for test and prod; the
// sandbox tester at test.paycom.uz drives the webhook, not this URL).
func (c Config) PaymeCheckoutHost() string {
	return "https://checkout.paycom.uz"
}

func Load() (Config, error) {
	portStr := getenv("PORT", "8080")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return Config{}, fmt.Errorf("invalid PORT %q: %w", portStr, err)
	}
	cfg := Config{
		Env:           getenv("ENV", "dev"),
		Port:          port,
		DatabaseURL:   getenv("DATABASE_URL", "postgres://avtotest:avtotest@localhost:5432/avtotest?sslmode=disable"),
		RedisURL:      getenv("REDIS_URL", "redis://localhost:6379/0"),
		MediaBaseURL:  getenv("MEDIA_BASE_URL", "http://localhost:9000/media"),
		PublicBaseURL: strings.TrimRight(getenv("PUBLIC_BASE_URL", "http://localhost:3000"), "/"),

		JWTSecret:               getenv("JWT_SECRET", defaultJWTSecret),
		OTPChannel:              getenv("OTP_CHANNEL", "sandbox"),
		TelegramGatewayToken:    getenv("TELEGRAM_GATEWAY_TOKEN", ""),
		TelegramGatewayURL:      getenv("TELEGRAM_GATEWAY_URL", "https://gatewayapi.telegram.org"),
		ClientIPAssertionSecret: getenv("CLIENT_IP_ASSERTION_SECRET", ""),

		TelegramBotToken:      getenv("TELEGRAM_BOT_TOKEN", ""),
		TelegramBotAPIBaseURL: getenv("TELEGRAM_BOT_API_BASE_URL", "https://api.telegram.org"),
		TelegramBotUsername:   getenv("TELEGRAM_BOT_USERNAME", ""),
		TelegramBotMode:       getenv("TELEGRAM_BOT_MODE", "off"),
		TelegramWebhookSecret: getenv("TELEGRAM_WEBHOOK_SECRET", ""),

		PaymeMerchantID: getenv("PAYME_MERCHANT_ID", ""),
		PaymeEnv:        getenv("PAYME_ENV", "test"),
		PaymeKeyProd:    getenv("PAYME_KEY", ""),
		PaymeTestKey:    getenv("PAYME_TEST_KEY", ""),

		ClickServiceID:  getenv("CLICK_SERVICE_ID", ""),
		ClickMerchantID: getenv("CLICK_MERCHANT_ID", ""),
		ClickSecretKey:  getenv("CLICK_SECRET_KEY", ""),

		OpsAdminToken: getenv("OPS_ADMIN_TOKEN", ""),
	}
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) validate() error {
	switch c.Env {
	case "dev", "staging", "prod":
	default:
		return fmt.Errorf("invalid ENV %q: must be dev, staging, or prod", c.Env)
	}

	switch c.OTPChannel {
	case "sandbox":
	case "telegram":
		if strings.TrimSpace(c.TelegramGatewayToken) == "" {
			return fmt.Errorf("OTP_CHANNEL telegram requires TELEGRAM_GATEWAY_TOKEN")
		}
	case "sms":
		return fmt.Errorf("invalid OTP_CHANNEL %q: no sender implementation is configured", c.OTPChannel)
	default:
		return fmt.Errorf("invalid OTP_CHANNEL %q: must be sandbox or telegram", c.OTPChannel)
	}

	if c.Env == "staging" || c.Env == "prod" {
		if c.OTPChannel == "sandbox" {
			return fmt.Errorf("OTP_CHANNEL sandbox is not allowed when ENV=%s", c.Env)
		}
		if c.JWTSecret == defaultJWTSecret {
			return fmt.Errorf("JWT_SECRET must not use the development default when ENV=%s", c.Env)
		}
		if len(strings.TrimSpace(c.JWTSecret)) < minJWTSecretLen {
			return fmt.Errorf("JWT_SECRET must be at least %d bytes when ENV=%s", minJWTSecretLen, c.Env)
		}
		if strings.TrimSpace(c.ClientIPAssertionSecret) == "" {
			return fmt.Errorf("CLIENT_IP_ASSERTION_SECRET is required when ENV=%s", c.Env)
		}
		// Payme/Click redirect here after payment; localhost would strand payers.
		if strings.TrimSpace(c.PublicBaseURL) == "" || strings.HasPrefix(c.PublicBaseURL, "http://localhost") {
			return fmt.Errorf("PUBLIC_BASE_URL must be a real public origin when ENV=%s", c.Env)
		}
	}
	if secret := strings.TrimSpace(c.ClientIPAssertionSecret); secret != "" && len([]byte(secret)) < minClientIPAssertionSecretLen {
		return fmt.Errorf("CLIENT_IP_ASSERTION_SECRET must be at least %d bytes", minClientIPAssertionSecretLen)
	}

	switch c.TelegramBotMode {
	case "off":
	case "webhook":
		if strings.TrimSpace(c.TelegramBotToken) == "" {
			return fmt.Errorf("TELEGRAM_BOT_MODE webhook requires TELEGRAM_BOT_TOKEN")
		}
		if strings.TrimSpace(c.TelegramBotUsername) == "" {
			return fmt.Errorf("TELEGRAM_BOT_MODE webhook requires TELEGRAM_BOT_USERNAME")
		}
		if strings.TrimSpace(c.TelegramWebhookSecret) == "" {
			return fmt.Errorf("TELEGRAM_BOT_MODE webhook requires TELEGRAM_WEBHOOK_SECRET")
		}
	case "longpoll":
		if strings.TrimSpace(c.TelegramBotToken) == "" {
			return fmt.Errorf("TELEGRAM_BOT_MODE longpoll requires TELEGRAM_BOT_TOKEN")
		}
		if strings.TrimSpace(c.TelegramBotUsername) == "" {
			return fmt.Errorf("TELEGRAM_BOT_MODE longpoll requires TELEGRAM_BOT_USERNAME")
		}
		// Long-poll is a single-consumer model: Telegram hands updates to
		// whichever getUpdates call is currently open. Running it from more
		// than one prod instance means the others silently starve, and
		// Telegram's own docs warn against mixing it with a webhook on the
		// same bot — so it's dev-only here, not just dev-recommended.
		if c.Env == "prod" {
			return fmt.Errorf("TELEGRAM_BOT_MODE longpoll is not allowed when ENV=prod (use webhook)")
		}
	default:
		return fmt.Errorf("invalid TELEGRAM_BOT_MODE %q: must be off, webhook, or longpoll", c.TelegramBotMode)
	}

	return nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
