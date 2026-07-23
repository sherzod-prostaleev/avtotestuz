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

	JWTSecret               string
	OTPChannel              string // sandbox | telegram
	TelegramGatewayToken    string
	TelegramGatewayURL      string
	ClientIPAssertionSecret string
}

func Load() (Config, error) {
	portStr := getenv("PORT", "8080")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return Config{}, fmt.Errorf("invalid PORT %q: %w", portStr, err)
	}
	cfg := Config{
		Env:          getenv("ENV", "dev"),
		Port:         port,
		DatabaseURL:  getenv("DATABASE_URL", "postgres://avtotest:avtotest@localhost:5432/avtotest?sslmode=disable"),
		RedisURL:     getenv("REDIS_URL", "redis://localhost:6379/0"),
		MediaBaseURL: getenv("MEDIA_BASE_URL", "http://localhost:9000/media"),

		JWTSecret:               getenv("JWT_SECRET", defaultJWTSecret),
		OTPChannel:              getenv("OTP_CHANNEL", "sandbox"),
		TelegramGatewayToken:    getenv("TELEGRAM_GATEWAY_TOKEN", ""),
		TelegramGatewayURL:      getenv("TELEGRAM_GATEWAY_URL", "https://gatewayapi.telegram.org"),
		ClientIPAssertionSecret: getenv("CLIENT_IP_ASSERTION_SECRET", ""),
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
	}
	if secret := strings.TrimSpace(c.ClientIPAssertionSecret); secret != "" && len([]byte(secret)) < minClientIPAssertionSecretLen {
		return fmt.Errorf("CLIENT_IP_ASSERTION_SECRET must be at least %d bytes", minClientIPAssertionSecretLen)
	}

	return nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
