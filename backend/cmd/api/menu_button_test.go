package main

import (
	"testing"

	"avtotest.uz/backend/internal/config"
)

// Clearing TELEGRAM_WEBAPP_URL is the Mini App kill switch, and an operator
// turning the bot off (TELEGRAM_BOT_MODE=off) is exactly when it must also
// reset the menu button — so the sync depends on the token alone.
func TestMenuButtonSyncWanted(t *testing.T) {
	cases := []struct {
		mode, token string
		want        bool
	}{
		{"off", "123:tok", true},
		{"webhook", "123:tok", true},
		{"longpoll", "123:tok", true},
		{"off", "", false},
		{"webhook", "  ", false},
	}
	for _, tc := range cases {
		cfg := config.Config{TelegramBotMode: tc.mode, TelegramBotToken: tc.token}
		if got := menuButtonSyncWanted(cfg); got != tc.want {
			t.Errorf("mode=%q token=%q: got %v, want %v", tc.mode, tc.token, got, tc.want)
		}
	}
}
