package server

import (
	"net/http"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"

	"avtotest.uz/backend/internal/avatar"
	"avtotest.uz/backend/internal/blob"
	"avtotest.uz/backend/internal/bot"
	"avtotest.uz/backend/internal/config"
	"avtotest.uz/backend/internal/db/sqlc"
)

// NewAvatarService wires learners' Telegram photos: written to the public
// media bucket (MINIO_MEDIA_BUCKET, default "media" — the bucket behind
// MEDIA_BASE_URL) and fetched with the bot token. It returns nil (avatars
// off, every learner keeps the initial letter) when the bucket cannot be
// opened; without a bot token photos are only ever cleared, never fetched.
// cmd/api calls it once more for the dev long-poll bot.
func NewAvatarService(cfg config.Config, q *sqlc.Queries, log *zap.Logger) *avatar.Service {
	bucket := strings.TrimSpace(os.Getenv("MINIO_MEDIA_BUCKET"))
	if bucket == "" {
		bucket = "media"
	}
	store, err := blob.NewS3FromEnv(bucket)
	if err != nil {
		log.Warn("avatars disabled: media bucket unavailable", zap.Error(err))
		return nil
	}
	var photos avatar.PhotoSource
	if strings.TrimSpace(cfg.TelegramBotToken) != "" {
		// Own client with a timeout: the shared bot client has none, and a
		// stalled Telegram must not pin a background worker.
		photos = bot.NewClient(cfg.TelegramBotAPIBaseURL, cfg.TelegramBotToken, &http.Client{Timeout: 20 * time.Second})
	}
	return avatar.New(q, store, photos, cfg.MediaBaseURL, log)
}
