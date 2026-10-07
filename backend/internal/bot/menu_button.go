package bot

import "context"

// SyncMenuButton makes the bot's menu button match config on every start:
// a Mini App launcher when TELEGRAM_WEBAPP_URL is set, Telegram's default
// commands menu when it is cleared, so unsetting the variable and
// restarting is the kill switch (spec section 1.4).
func SyncMenuButton(ctx context.Context, c *Client, webAppURL string) error {
	if webAppURL == "" {
		return c.SetChatMenuButton(ctx, map[string]any{"type": "default"})
	}
	return c.SetChatMenuButton(ctx, map[string]any{
		"type":    "web_app",
		"text":    "Ochish",
		"web_app": WebAppInfo{URL: webAppURL},
	})
}
