// Package config loads service configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Env          string // dev | staging | prod
	Port         int
	DatabaseURL  string
	RedisURL     string
	MediaBaseURL string // public base for image storage keys

	JWTSecret            string
	OTPChannel           string // sandbox | telegram | sms
	TelegramGatewayToken string
	TelegramGatewayURL   string
}

func Load() (Config, error) {
	portStr := getenv("PORT", "8080")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return Config{}, fmt.Errorf("invalid PORT %q: %w", portStr, err)
	}
	return Config{
		Env:          getenv("ENV", "dev"),
		Port:         port,
		DatabaseURL:  getenv("DATABASE_URL", "postgres://avtotest:avtotest@localhost:5432/avtotest?sslmode=disable"),
		RedisURL:     getenv("REDIS_URL", "redis://localhost:6379/0"),
		MediaBaseURL: getenv("MEDIA_BASE_URL", "http://localhost:9000/media"),

		JWTSecret:            getenv("JWT_SECRET", "dev-secret-change-me"),
		OTPChannel:           getenv("OTP_CHANNEL", "sandbox"),
		TelegramGatewayToken: getenv("TELEGRAM_GATEWAY_TOKEN", ""),
		TelegramGatewayURL:   getenv("TELEGRAM_GATEWAY_URL", "https://gatewayapi.telegram.org"),
	}, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
