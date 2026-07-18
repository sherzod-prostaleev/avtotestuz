package auth

import (
	"context"
	"fmt"
	"net/http"

	"go.uber.org/zap"

	"avtotest.uz/backend/internal/config"
)

// Sender delivers an OTP code to a phone over some channel.
type Sender interface {
	Send(ctx context.Context, phone, code string) error
	Channel() string
}

// SandboxSender logs the code instead of delivering it; used in dev/test.
type SandboxSender struct{ Log *zap.Logger }

func (s SandboxSender) Send(_ context.Context, phone, code string) error {
	s.Log.Info("sandbox otp", zap.String("phone", phone), zap.String("code", code))
	return nil
}

func (SandboxSender) Channel() string { return "sandbox" }

// SenderFor picks the OTP delivery channel from config.
func SenderFor(cfg config.Config, log *zap.Logger) (Sender, error) {
	switch cfg.OTPChannel {
	case "", "sandbox":
		return SandboxSender{Log: log}, nil
	case "telegram":
		if cfg.TelegramGatewayToken == "" {
			return nil, fmt.Errorf("telegram sender requires TELEGRAM_GATEWAY_TOKEN")
		}
		return NewTelegramSender(cfg.TelegramGatewayURL, cfg.TelegramGatewayToken, http.DefaultClient), nil
	case "sms":
		return nil, fmt.Errorf("sms sender not configured (Plan 05+)")
	default:
		return nil, fmt.Errorf("unknown OTP_CHANNEL %q", cfg.OTPChannel)
	}
}
