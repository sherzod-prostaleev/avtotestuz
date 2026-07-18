package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// TelegramSender delivers OTP codes via the Telegram Gateway API.
type TelegramSender struct {
	BaseURL string
	Token   string
	HC      *http.Client
}

func NewTelegramSender(baseURL, token string, hc *http.Client) *TelegramSender {
	return &TelegramSender{BaseURL: baseURL, Token: token, HC: hc}
}

func (t *TelegramSender) Send(ctx context.Context, phone, code string) error {
	body, err := json.Marshal(map[string]string{
		"phone_number": phone,
		"code":         code,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.BaseURL+"/sendVerificationMessage", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.Token)

	resp, err := t.HC.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram gateway: unexpected status %d", resp.StatusCode)
	}
	return nil
}

func (*TelegramSender) Channel() string { return "telegram" }
