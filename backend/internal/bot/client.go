package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Client is a minimal Telegram Bot API client — only the handful of methods
// this package needs, deliberately not a general-purpose SDK. Same shape as
// auth.TelegramSender: a plain REST wrapper, no external bot library
// dependency.
type Client struct {
	BaseURL string // e.g. https://api.telegram.org
	Token   string
	HC      *http.Client
}

func NewClient(baseURL, token string, hc *http.Client) *Client {
	if hc == nil {
		hc = http.DefaultClient
	}
	return &Client{BaseURL: baseURL, Token: token, HC: hc}
}

func (c *Client) call(ctx context.Context, method string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/bot%s/%s", c.BaseURL, c.Token, method)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HC.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	var envelope struct {
		OK          bool            `json:"ok"`
		Description string          `json:"description"`
		Result      json.RawMessage `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("telegram %s: decode response: %w", method, err)
	}
	if !envelope.OK {
		return fmt.Errorf("telegram %s: %s", method, envelope.Description)
	}
	if out != nil && len(envelope.Result) > 0 {
		if err := json.Unmarshal(envelope.Result, out); err != nil {
			return fmt.Errorf("telegram %s: decode result: %w", method, err)
		}
	}
	return nil
}

// SendMessage delivers plain text to a chat (a direct-message chat ID is
// the same as the user's tg_user_id).
func (c *Client) SendMessage(ctx context.Context, chatID int64, text string) error {
	return c.call(ctx, "sendMessage", map[string]any{
		"chat_id": chatID,
		"text":    text,
	}, nil)
}

// GetUpdates long-polls for new updates, starting at offset (the caller is
// expected to pass lastUpdateID+1 on the next call — Telegram's own
// "confirm receipt by asking past it" convention). Used only by the dev
// long-poll runner; webhook mode never calls this.
func (c *Client) GetUpdates(ctx context.Context, offset int64, timeoutSec int) ([]Update, error) {
	var updates []Update
	err := c.call(ctx, "getUpdates", map[string]any{
		"offset":  offset,
		"timeout": timeoutSec,
	}, &updates)
	return updates, err
}

// SetWebhook registers url as the bot's webhook target, with secretToken
// round-tripped on every call as X-Telegram-Bot-Api-Secret-Token — that
// header is how webhook.Handler verifies a request actually came from
// Telegram. Not called automatically anywhere in this codebase; it's a
// one-time operational step (design §7).
func (c *Client) SetWebhook(ctx context.Context, url, secretToken string) error {
	return c.call(ctx, "setWebhook", map[string]any{
		"url":          url,
		"secret_token": secretToken,
	}, nil)
}

// DeleteWebhook removes the webhook, e.g. before switching a deployment to
// long-poll mode (the two are mutually exclusive on Telegram's side).
func (c *Client) DeleteWebhook(ctx context.Context) error {
	return c.call(ctx, "deleteWebhook", map[string]any{}, nil)
}
