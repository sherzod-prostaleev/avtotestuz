package bot

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/flags"
)

const (
	// KindTelegramDueDigest is notification.kind for linked-user DM digests.
	KindTelegramDueDigest = "tg_fsrs_due"

	// DigestCooldown skips profiles already messaged recently.
	DigestCooldown = 20 * time.Hour

	defaultDigestLimit = 500
)

// DigestService sends soft due/streak reminders to linked Telegram accounts.
// It does NOT post into groups — groups use on-demand /quiz.
type DigestService struct {
	Q             *sqlc.Queries
	Pool          *pgxpool.Pool
	TG            *Client
	PublicBaseURL string
	Log           *zap.Logger
}

type DigestOpts struct {
	Limit  int
	DryRun bool
}

type DigestResult struct {
	Candidates int
	Notified   int
	Errors     int
	Skipped    int
}

func (s *DigestService) logger() *zap.Logger {
	if s != nil && s.Log != nil {
		return s.Log
	}
	return zap.NewNop()
}

func (s *DigestService) ctaURL() string {
	base := strings.TrimRight(s.PublicBaseURL, "/")
	if base == "" {
		base = "https://avtotest.uz"
	}
	return base + "/uz-Latn"
}

// RunDueDigest lists linked profiles with due cards and optionally DMs them.
func (s *DigestService) RunDueDigest(ctx context.Context, opts DigestOpts) (DigestResult, error) {
	if s == nil || s.Pool == nil || s.Q == nil {
		return DigestResult{}, fmt.Errorf("digest service not configured")
	}
	if !opts.DryRun {
		enabled, err := flags.Bool(ctx, s.Pool, flags.KeyTelegramDMDigest, true)
		if err != nil {
			return DigestResult{}, err
		}
		if !enabled {
			return DigestResult{}, ErrDigestDisabled
		}
		if s.TG == nil || s.TG.Token == "" {
			return DigestResult{}, ErrDigestUnconfigured
		}
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = defaultDigestLimit
	}

	rows, err := s.Q.ListTelegramDigestCandidates(ctx, sqlc.ListTelegramDigestCandidatesParams{
		Kind:       KindTelegramDueDigest,
		Cooldown:   fmt.Sprintf("%f seconds", DigestCooldown.Seconds()),
		LimitCount: int32(limit),
	})
	if err != nil {
		return DigestResult{}, err
	}
	res := DigestResult{Candidates: len(rows)}
	if opts.DryRun {
		return res, nil
	}

	for _, row := range rows {
		text := fmt.Sprintf(
			"Bugun takrorlash navbati: %d ta savol.\n\nBotda /quiz yozing yoki ilovada mashq qiling:\n%s",
			row.DueCount, s.ctaURL(),
		)
		if _, err := s.TG.SendText(ctx, row.TgUserID, text, &InlineKeyboardMarkup{
			InlineKeyboard: [][]InlineKeyboardButton{{
				{Text: "Ilovada ochish", URL: s.ctaURL()},
			}},
		}); err != nil {
			s.logger().Warn("tg digest send failed",
				zap.Int64("tg_user_id", row.TgUserID), zap.Error(err))
			res.Errors++
			continue
		}
		if _, err := s.Q.InsertNotification(ctx, sqlc.InsertNotificationParams{
			ProfileID: row.ProfileID,
			Kind:      KindTelegramDueDigest,
			Payload:   []byte(`{"channel":"telegram"}`),
			Channel:   "telegram",
		}); err != nil {
			s.logger().Warn("tg digest notification row failed", zap.Error(err))
			res.Errors++
			continue
		}
		res.Notified++
	}
	return res, nil
}

var (
	ErrDigestDisabled     = errors.New("telegram_dm_digest feature flag is off")
	ErrDigestUnconfigured = errors.New("telegram bot token not configured")
)
