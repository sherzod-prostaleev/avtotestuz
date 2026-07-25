package bot

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"time"

	"go.uber.org/zap"
)

const webhookDispatchTimeout = 20 * time.Second

// WebhookHandler serves Telegram's webhook callback. The
// X-Telegram-Bot-Api-Secret-Token header is the entire trust boundary here
// (design §3.1) — it is set once via Client.SetWebhook and Telegram echoes
// it back on every call, so a mismatch means the request did not come from
// Telegram (or the secret is misconfigured) and update.message.from.id
// cannot be trusted.
type WebhookHandler struct {
	Bot    *Bot
	Secret string
	Log    *zap.Logger
}

func (h *WebhookHandler) logger() *zap.Logger {
	if h.Log != nil {
		return h.Log
	}
	return zap.NewNop()
}

func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	got := r.Header.Get("X-Telegram-Bot-Api-Secret-Token")
	if subtle.ConstantTimeCompare([]byte(got), []byte(h.Secret)) != 1 {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	var u Update
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		// A malformed body is not something retrying will fix. Swallow it
		// (logged) and still return 200 so Telegram doesn't hammer retries
		// on a payload that will never parse.
		h.logger().Warn("bot webhook: malformed update body", zap.Error(err))
		w.WriteHeader(http.StatusOK)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), webhookDispatchTimeout)
	defer cancel()
	if err := h.Bot.HandleUpdate(ctx, u); err != nil {
		// HandleUpdate already turns user-facing failures into a reply and
		// only returns an error for infra faults; log it but still 200 —
		// Telegram retrying the same update wouldn't help if e.g. our DB is
		// down for that one request, and a 5xx storm makes it worse.
		h.logger().Error("bot webhook: dispatch failed", zap.Error(err))
	}
	w.WriteHeader(http.StatusOK)
}
