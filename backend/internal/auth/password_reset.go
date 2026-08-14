package auth

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"avtotest.uz/backend/internal/db/sqlc"
)

const (
	// PasswordResetTTL is long enough to switch from the browser to Telegram
	// and back, short enough that a leaked deep link is a narrow window.
	PasswordResetTTL = 15 * time.Minute

	// PasswordResetStartPrefix is the Telegram /start payload prefix. Link
	// tokens are unprefixed opaque values; this prefix keeps the two flows
	// from colliding (Telegram's start argument is capped at 64 bytes —
	// "pwr_" + 43-char token = 47).
	PasswordResetStartPrefix = "pwr_"

	passwordResetExpiresSec = int(PasswordResetTTL / time.Second)
)

var (
	ErrTelegramBotUnconfigured = errors.New("telegram bot unconfigured")
	ErrResetInvalid            = errors.New("invalid reset token")
	ErrResetNotVerified        = errors.New("reset not verified")
)

// PasswordResetStart is the public start-response. BotURL is always populated
// with a pwr_ payload so missing accounts are indistinguishable from real ones.
type PasswordResetStart struct {
	BotURL       string `json:"bot_url"`
	ExpiresInSec int    `json:"expires_in_sec"`
}

// PasswordResetStatus is a coarse state for the waiting website tab.
type PasswordResetStatus struct {
	State string `json:"state"` // pending | verified | invalid
}

const (
	ResetStatePending        = "pending"
	ResetStateVerified       = "verified"
	ResetStateInvalid        = "invalid"
	TelegramResetInvalid     = "invalid"
	TelegramResetNeedContact = "need_contact"
	TelegramResetVerified    = "verified"
)

type TelegramResetBegin struct {
	Outcome string
}

func FormatPasswordResetStartPayload(raw string) string {
	return PasswordResetStartPrefix + raw
}

func ParsePasswordResetStartPayload(arg string) (raw string, ok bool) {
	if !strings.HasPrefix(arg, PasswordResetStartPrefix) {
		return "", false
	}
	raw = strings.TrimPrefix(arg, PasswordResetStartPrefix)
	if raw == "" {
		return "", false
	}
	return raw, true
}

func passwordResetDeepLink(botUsername, raw string) string {
	return "https://t.me/" + strings.TrimPrefix(strings.TrimSpace(botUsername), "@") +
		"?start=" + FormatPasswordResetStartPayload(raw)
}

// StartPasswordReset always returns a deep link. A row is stored only when the
// phone belongs to an active learner — callers must not branch on that.
func (s *Service) StartPasswordReset(ctx context.Context, rawPhone, ip, botUsername string) (PasswordResetStart, error) {
	if strings.TrimSpace(botUsername) == "" {
		return PasswordResetStart{}, ErrTelegramBotUnconfigured
	}
	phone, err := NormalizePhone(rawPhone)
	if err != nil {
		return PasswordResetStart{}, ErrInvalidPhone
	}
	if err := s.rateLimitAuth(ctx, "reset", phone, ip); err != nil {
		return PasswordResetStart{}, err
	}
	if ok, err := s.Lim.Cooldown(ctx, "reset:cooldown:"+phone, 45*time.Second); err != nil {
		return PasswordResetStart{}, err
	} else if !ok {
		return PasswordResetStart{}, ErrRateLimited
	}

	raw, err := NewRefreshToken()
	if err != nil {
		return PasswordResetStart{}, err
	}
	// Issued marker is written for every start — including unknown/banned
	// phones — so GET status cannot be used to enumerate accounts.
	if err := s.markResetTokenIssued(ctx, raw); err != nil {
		return PasswordResetStart{}, err
	}
	out := PasswordResetStart{
		BotURL:       passwordResetDeepLink(botUsername, raw),
		ExpiresInSec: passwordResetExpiresSec,
	}

	profile, err := s.Q.GetProfileByPhone(ctx, phone)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return out, nil
		}
		return PasswordResetStart{}, err
	}
	if assertProfileActive(profile) != nil {
		return out, nil
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return PasswordResetStart{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := sqlc.New(tx)
	if err := q.DeleteUnusedPasswordResetTokensForProfile(ctx, profile.ID); err != nil {
		return PasswordResetStart{}, err
	}
	if _, err := q.CreatePasswordResetToken(ctx, sqlc.CreatePasswordResetTokenParams{
		ProfileID: profile.ID,
		TokenHash: HashToken(raw),
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(PasswordResetTTL), Valid: true},
	}); err != nil {
		return PasswordResetStart{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return PasswordResetStart{}, err
	}
	return out, nil
}

func (s *Service) PasswordResetStatus(ctx context.Context, rawToken string) PasswordResetStatus {
	if strings.TrimSpace(rawToken) == "" {
		return PasswordResetStatus{State: ResetStateInvalid}
	}

	row, dbErr := s.Q.GetPasswordResetTokenByHash(ctx, HashToken(rawToken))
	issued := s.resetTokenWasIssued(ctx, rawToken)

	if dbErr == nil {
		if !resetTokenLive(row) {
			return PasswordResetStatus{State: ResetStateInvalid}
		}
		if row.VerifiedAt.Valid {
			return PasswordResetStatus{State: ResetStateVerified}
		}
		return PasswordResetStatus{State: ResetStatePending}
	}
	if !errors.Is(dbErr, pgx.ErrNoRows) {
		// A DB blip must not look like "no account".
		return PasswordResetStatus{State: ResetStatePending}
	}
	if issued {
		return PasswordResetStatus{State: ResetStatePending}
	}
	return PasswordResetStatus{State: ResetStateInvalid}
}

// BeginTelegramPasswordReset is called from the bot /start pwr_ path. The
// Telegram user id must come from Telegram itself (webhook / long-poll).
func (s *Service) BeginTelegramPasswordReset(ctx context.Context, rawToken string, tgUserID int64) (TelegramResetBegin, error) {
	if strings.TrimSpace(rawToken) == "" || tgUserID == 0 {
		return TelegramResetBegin{Outcome: TelegramResetInvalid}, nil
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return TelegramResetBegin{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := sqlc.New(tx)

	row, err := q.GetPasswordResetTokenByHashForUpdate(ctx, HashToken(rawToken))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TelegramResetBegin{Outcome: TelegramResetInvalid}, nil
		}
		return TelegramResetBegin{}, err
	}
	if !resetTokenLive(row) {
		return TelegramResetBegin{Outcome: TelegramResetInvalid}, nil
	}

	profile, err := q.GetProfileByID(ctx, row.ProfileID)
	if err != nil {
		return TelegramResetBegin{}, err
	}
	if assertProfileActive(profile) != nil {
		return TelegramResetBegin{Outcome: TelegramResetInvalid}, nil
	}

	if row.VerifiedAt.Valid {
		if err := tx.Commit(ctx); err != nil {
			return TelegramResetBegin{}, err
		}
		return TelegramResetBegin{Outcome: TelegramResetVerified}, nil
	}

	account, err := q.GetTelegramAccountByTgUserID(ctx, tgUserID)
	switch {
	case err == nil:
		if account.ProfileID != row.ProfileID {
			return TelegramResetBegin{Outcome: TelegramResetInvalid}, nil
		}
		if err := q.MarkPasswordResetVerified(ctx, row.ID); err != nil {
			return TelegramResetBegin{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return TelegramResetBegin{}, err
		}
		return TelegramResetBegin{Outcome: TelegramResetVerified}, nil
	case errors.Is(err, pgx.ErrNoRows):
		if err := q.ClearPasswordResetPendingForTg(ctx, sqlc.ClearPasswordResetPendingForTgParams{
			PendingTgUserID: pgtype.Int8{Int64: tgUserID, Valid: true},
			ID:              row.ID,
		}); err != nil {
			return TelegramResetBegin{}, err
		}
		if err := q.SetPasswordResetPendingTg(ctx, sqlc.SetPasswordResetPendingTgParams{
			ID:              row.ID,
			PendingTgUserID: pgtype.Int8{Int64: tgUserID, Valid: true},
		}); err != nil {
			return TelegramResetBegin{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return TelegramResetBegin{}, err
		}
		return TelegramResetBegin{Outcome: TelegramResetNeedContact}, nil
	default:
		return TelegramResetBegin{}, err
	}
}

// ConfirmTelegramPasswordResetContact proves the Telegram user owns the
// account phone via Telegram's request_contact keyboard. contactUserID must
// equal tgUserID so a forwarded third-party contact cannot be used.
func (s *Service) ConfirmTelegramPasswordResetContact(ctx context.Context, tgUserID, contactUserID int64, contactPhone string) (TelegramResetBegin, error) {
	if tgUserID == 0 || contactUserID != tgUserID {
		return TelegramResetBegin{Outcome: TelegramResetInvalid}, nil
	}
	normalized, err := NormalizePhone(contactPhone)
	if err != nil {
		return TelegramResetBegin{Outcome: TelegramResetInvalid}, nil
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return TelegramResetBegin{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := sqlc.New(tx)

	row, err := q.GetLivePasswordResetByPendingTgForUpdate(ctx, pgtype.Int8{Int64: tgUserID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TelegramResetBegin{Outcome: TelegramResetInvalid}, nil
		}
		return TelegramResetBegin{}, err
	}
	if !resetTokenLive(row) {
		return TelegramResetBegin{Outcome: TelegramResetInvalid}, nil
	}

	profile, err := q.GetProfileByID(ctx, row.ProfileID)
	if err != nil {
		return TelegramResetBegin{}, err
	}
	if assertProfileActive(profile) != nil || profile.Phone != normalized {
		return TelegramResetBegin{Outcome: TelegramResetInvalid}, nil
	}

	if err := q.MarkPasswordResetVerified(ctx, row.ID); err != nil {
		return TelegramResetBegin{}, err
	}
	if err := q.UpsertTelegramAccount(ctx, sqlc.UpsertTelegramAccountParams{
		ProfileID: row.ProfileID,
		TgUserID:  tgUserID,
		Username:  "",
	}); err != nil {
		if isUniqueViolation(err) {
			return TelegramResetBegin{Outcome: TelegramResetInvalid}, nil
		}
		return TelegramResetBegin{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return TelegramResetBegin{}, err
	}
	return TelegramResetBegin{Outcome: TelegramResetVerified}, nil
}

func (s *Service) CompletePasswordReset(ctx context.Context, rawToken, newPassword, ip string) error {
	if utf8.RuneCountInString(newPassword) < minPasswordLen {
		return ErrWeakPassword
	}
	if err := s.rateLimitAuth(ctx, "reset_complete", HashToken(rawToken), ip); err != nil {
		return err
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := sqlc.New(tx)

	row, err := q.GetPasswordResetTokenByHashForUpdate(ctx, HashToken(rawToken))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrResetInvalid
		}
		return err
	}
	if !resetTokenLive(row) {
		return ErrResetInvalid
	}
	if !row.VerifiedAt.Valid {
		return ErrResetNotVerified
	}

	profile, err := q.GetProfileByID(ctx, row.ProfileID)
	if err != nil {
		return err
	}
	if err := assertProfileActive(profile); err != nil {
		return err
	}

	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}

	if _, err := q.SetProfilePassword(ctx, sqlc.SetProfilePasswordParams{
		ID:                 row.ProfileID,
		PasswordHash:       pgtype.Text{String: hash, Valid: true},
		MustChangePassword: false,
	}); err != nil {
		return err
	}
	if err := q.MarkPasswordResetUsed(ctx, row.ID); err != nil {
		return err
	}
	if err := q.RevokeAllRefreshTokens(ctx, row.ProfileID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func resetTokenLive(row sqlc.PasswordResetToken) bool {
	if row.UsedAt.Valid {
		return false
	}
	if !row.ExpiresAt.Valid || time.Now().After(row.ExpiresAt.Time) {
		return false
	}
	return true
}

func resetIssuedKey(raw string) string {
	return "pwdreset:issued:" + HashToken(raw)
}

func (s *Service) markResetTokenIssued(ctx context.Context, raw string) error {
	if s.Lim.R == nil {
		return errors.New("reset issued store unavailable")
	}
	return s.Lim.R.Set(ctx, resetIssuedKey(raw), "1", PasswordResetTTL).Err()
}

func (s *Service) resetTokenWasIssued(ctx context.Context, raw string) bool {
	if s.Lim.R == nil || strings.TrimSpace(raw) == "" {
		return false
	}
	n, err := s.Lim.R.Exists(ctx, resetIssuedKey(raw)).Result()
	if err != nil {
		return true
	}
	return n > 0
}
