package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

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
	// TelegramResetNeedConfirm: the Telegram identity matched; the bot must
	// now ask «Ha, men» / «Yo'q» with ConfirmNonce in the buttons. Nothing is
	// verified until AnswerTelegramPasswordResetConfirm gets a «Ha, men».
	TelegramResetNeedConfirm = "need_confirm"
	TelegramResetVerified    = "verified"
	// TelegramResetCancelled: the learner answered «Yo'q»; the reset is spent.
	TelegramResetCancelled = "cancelled"
	// TelegramResetStale: a confirm tap that no longer applies (expired,
	// already answered, re-armed by a newer /start, or from another user).
	TelegramResetStale = "stale"
	// TelegramResetNone: a contact arrived with no reset waiting for it (the
	// Mini App's phone share lands in the bot chat too). Not an error.
	TelegramResetNone = "none"
)

type TelegramResetBegin struct {
	Outcome string
	// ConfirmNonce and MaskedPhone are set only with TelegramResetNeedConfirm.
	// The nonce is a fresh random value, never the reset token: callback_data
	// is visible to Telegram clients and must not be able to complete a reset.
	ConfirmNonce string
	MaskedPhone  string
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
	res, err := s.beginTelegramPasswordReset(ctx, rawToken, tgUserID)
	if isUniqueViolation(err) {
		// Two /start pwr_ of the same Telegram user for different resets ran
		// at once: each cleared the other's (not yet committed) pending mark,
		// then one lost on the pending index. Once is enough: the retry sees
		// the winner's committed row and clears it like any older reset.
		res, err = s.beginTelegramPasswordReset(ctx, rawToken, tgUserID)
	}
	return res, err
}

func (s *Service) beginTelegramPasswordReset(ctx context.Context, rawToken string, tgUserID int64) (TelegramResetBegin, error) {
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

	// Only a phone-verified link to THIS profile is identity. A legacy
	// /start <token> link carries no phone proof and an intruder can plant one
	// on a victim's profile; a link to another profile proves nothing about
	// this one. Both take the contact step (phone must match, then «Ha, men»).
	account, err := q.GetTelegramAccountByTgUserID(ctx, tgUserID)
	linked := false
	switch {
	case err == nil:
		linked = account.ProfileID == row.ProfileID && account.PhoneVerifiedAt.Valid
	case errors.Is(err, pgx.ErrNoRows):
	default:
		return TelegramResetBegin{}, err
	}

	// Either way this Telegram user now owns the reset's next step; any other
	// reset still armed for them is dropped (the pending index is unique).
	if err := q.ClearPasswordResetPendingForTg(ctx, sqlc.ClearPasswordResetPendingForTgParams{
		PendingTgUserID: pgtype.Int8{Int64: tgUserID, Valid: true},
		ID:              row.ID,
	}); err != nil {
		return TelegramResetBegin{}, err
	}
	// The same goes for a website login this Telegram user opened in the bot
	// (/start login_) and left waiting: from here on their phone share and
	// taps are about the reset. Without this, the share meant for the reset
	// answered the login instead (audit F1, scenario A1).
	if err := q.ClearAllTelegramLoginPendingForTg(ctx, pgtype.Int8{Int64: tgUserID, Valid: true}); err != nil {
		return TelegramResetBegin{}, err
	}
	if err := q.SetPasswordResetPendingTg(ctx, sqlc.SetPasswordResetPendingTgParams{
		ID:              row.ID,
		PendingTgUserID: pgtype.Int8{Int64: tgUserID, Valid: true},
	}); err != nil {
		return TelegramResetBegin{}, err
	}
	if !linked {
		if err := tx.Commit(ctx); err != nil {
			return TelegramResetBegin{}, err
		}
		return TelegramResetBegin{Outcome: TelegramResetNeedContact}, nil
	}
	// A linked account proves who is tapping, not that they asked for this
	// reset: anyone can start a reset for their phone and send them the link.
	return askTelegramResetConfirm(ctx, tx, q, row.ID, profile.Phone)
}

// askTelegramResetConfirm stores a fresh confirm nonce for the reset and
// commits tx. A previous nonce (an older question message) stops working.
func askTelegramResetConfirm(ctx context.Context, tx pgx.Tx, q *sqlc.Queries, resetID uuid.UUID, phone string) (TelegramResetBegin, error) {
	nonce, err := newResetConfirmNonce()
	if err != nil {
		return TelegramResetBegin{}, err
	}
	if err := q.SetPasswordResetConfirmNonce(ctx, sqlc.SetPasswordResetConfirmNonceParams{
		ID:               resetID,
		ConfirmNonceHash: pgtype.Text{String: HashToken(nonce), Valid: true},
	}); err != nil {
		return TelegramResetBegin{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return TelegramResetBegin{}, err
	}
	return TelegramResetBegin{
		Outcome:      TelegramResetNeedConfirm,
		ConfirmNonce: nonce,
		MaskedPhone:  MaskResetPhone(phone),
	}, nil
}

// newResetConfirmNonce is 128 bits, base64url: 22 chars, so "pwr:y:" + nonce
// stays well inside Telegram's 64-byte callback_data limit.
func newResetConfirmNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate reset confirm nonce: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// MaskResetPhone renders a stored "+998901234567" as "+998 90 ••• •• 67":
// enough for the owner to recognise their number in the bot question, not
// enough to leak it to whoever else sees the chat.
func MaskResetPhone(phone string) string {
	d := strings.TrimPrefix(phone, "+")
	if len(d) != 12 || !strings.HasPrefix(d, "998") {
		return "•••"
	}
	return "+998 " + d[3:5] + " ••• •• " + d[10:]
}

// ConfirmTelegramPasswordResetContact proves the Telegram user owns the
// account phone via Telegram's request_contact keyboard. contactUserID must
// equal tgUserID so a forwarded third-party contact cannot be used.
//
// A match only earns the «Ha, men» question (TelegramResetNeedConfirm): the
// same contact message is also what the Mini App's phone share produces, so
// it is not evidence that the learner wants this reset.
func (s *Service) ConfirmTelegramPasswordResetContact(ctx context.Context, tgUserID, contactUserID int64, contactPhone string) (TelegramResetBegin, error) {
	if tgUserID == 0 || contactUserID != tgUserID {
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
			return TelegramResetBegin{Outcome: TelegramResetNone}, nil
		}
		return TelegramResetBegin{}, err
	}
	if !resetTokenLive(row) {
		// Expired while waiting: for this contact nothing is waiting any more,
		// and it may well be the Mini App's phone share rather than a reply.
		return TelegramResetBegin{Outcome: TelegramResetNone}, nil
	}
	normalized, err := NormalizeTelegramContactPhone(contactPhone)
	if err != nil {
		return TelegramResetBegin{Outcome: TelegramResetInvalid}, nil
	}

	profile, err := q.GetProfileByID(ctx, row.ProfileID)
	if err != nil {
		return TelegramResetBegin{}, err
	}
	if assertProfileActive(profile) != nil || profile.Phone != normalized {
		return TelegramResetBegin{Outcome: TelegramResetInvalid}, nil
	}
	return askTelegramResetConfirm(ctx, tx, q, row.ID, profile.Phone)
}

// AnswerTelegramPasswordResetConfirm handles a «Ha, men» (accept) or «Yo'q»
// tap. Only the Telegram user the reset is pending for, with the current
// nonce, on a live unverified reset, changes anything; every other tap is
// TelegramResetStale and a no-op. «Ha, men» verifies and leaves this
// Telegram account phone-verified-linked to the profile (moving it off any
// other profile); «Yo'q» spends the reset.
func (s *Service) AnswerTelegramPasswordResetConfirm(ctx context.Context, tgUserID int64, nonce string, accept bool) (TelegramResetBegin, error) {
	stale := TelegramResetBegin{Outcome: TelegramResetStale}
	if tgUserID == 0 || strings.TrimSpace(nonce) == "" {
		return stale, nil
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return TelegramResetBegin{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := sqlc.New(tx)

	row, err := q.GetPasswordResetByConfirmNonceForUpdate(ctx, pgtype.Text{String: HashToken(nonce), Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return stale, nil
		}
		return TelegramResetBegin{}, err
	}
	if !resetTokenLive(row) || row.VerifiedAt.Valid ||
		!row.PendingTgUserID.Valid || row.PendingTgUserID.Int64 != tgUserID {
		return stale, nil
	}
	profile, err := q.GetProfileByID(ctx, row.ProfileID)
	if err != nil {
		return TelegramResetBegin{}, err
	}
	if assertProfileActive(profile) != nil {
		return stale, nil
	}

	if !accept {
		if err := q.CancelPasswordReset(ctx, row.ID); err != nil {
			return TelegramResetBegin{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return TelegramResetBegin{}, err
		}
		return TelegramResetBegin{Outcome: TelegramResetCancelled}, nil
	}

	// What «Ha, men» changed, for the avatar service after commit.
	link := telegramLinkChange{profileID: row.ProfileID}
	account, err := q.GetTelegramAccountByTgUserID(ctx, tgUserID)
	switch {
	case err == nil && account.ProfileID == row.ProfileID:
		if !account.PhoneVerifiedAt.Valid {
			link.linked = true
			// A legacy link that got here passed the contact step, so the
			// phone is now proven for this very Telegram user.
			if err := q.UpsertTelegramAccount(ctx, sqlc.UpsertTelegramAccountParams{
				ProfileID:     row.ProfileID,
				TgUserID:      tgUserID,
				Username:      account.Username,
				PhoneVerified: true,
			}); err != nil {
				return TelegramResetBegin{}, err
			}
		}
	case err == nil || errors.Is(err, pgx.ErrNoRows):
		// This Telegram user is not linked to the profile (any more), yet a
		// pending reset with their current nonce is proof they own its phone:
		// either the contact step matched Telegram's contact of this very user
		// to the profile phone, or Begin took the shortcut through a
		// phone-verified link to this profile that has since been unlinked or
		// re-pointed. Either way the account is linked here, phone-verified.
		username := ""
		if err == nil {
			// Linked to another profile (e.g. the learner's second account).
			// Proving this phone moves the link, exactly as the Mini App's
			// phone share does (linkTelegramInTx); refusing left the reset
			// pending with no way to finish it.
			s.logger().Info("auth.telegram_link_moved",
				zap.String("from_profile_id", account.ProfileID.String()),
				zap.String("to_profile_id", row.ProfileID.String()))
			link.movedFrom = account.ProfileID
			if err := q.DeleteTelegramAccountForOtherProfiles(ctx, sqlc.DeleteTelegramAccountForOtherProfilesParams{
				TgUserID: tgUserID, ProfileID: row.ProfileID,
			}); err != nil {
				return TelegramResetBegin{}, err
			}
			username = account.Username
		}
		if prev, err := q.GetTelegramAccountByProfileID(ctx, row.ProfileID); err == nil && prev.TgUserID != tgUserID {
			s.logger().Info("auth.telegram_link_replaced", zap.String("profile_id", row.ProfileID.String()))
		} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return TelegramResetBegin{}, err
		}
		if err := q.UpsertTelegramAccount(ctx, sqlc.UpsertTelegramAccountParams{
			ProfileID:     row.ProfileID,
			TgUserID:      tgUserID,
			Username:      username,
			PhoneVerified: true,
		}); err != nil {
			if isUniqueViolation(err) {
				return stale, nil
			}
			return TelegramResetBegin{}, err
		}
		link.linked = true
	default:
		return TelegramResetBegin{}, err
	}
	if err := q.MarkPasswordResetVerified(ctx, row.ID); err != nil {
		return TelegramResetBegin{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return TelegramResetBegin{}, err
	}
	s.afterTelegramLink(ctx, link)
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
	dropped, err := s.dropUnattributedTelegramLink(ctx, q, row)
	if err != nil {
		return err
	}
	if err := q.RevokeAllRefreshTokens(ctx, row.ProfileID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	if dropped && s.Avatars != nil {
		s.Avatars.TelegramUnlinked(row.ProfileID)
	}
	return nil
}

// dropUnattributedTelegramLink runs in the reset transaction. A reset is the
// owner taking the account back, so a Telegram link survives it only when it
// is phone-verified AND belongs to the Telegram user who confirmed this very
// reset in the bot. Anything else — a legacy link an intruder planted, a link
// written after the confirmation, a reset verified before the confirmer was
// recorded — goes; the learner re-links with one phone share.
// It reports whether a link was dropped.
func (s *Service) dropUnattributedTelegramLink(ctx context.Context, q *sqlc.Queries, row sqlc.PasswordResetToken) (bool, error) {
	n, err := q.DeleteUnattributedTelegramAccount(ctx, sqlc.DeleteUnattributedTelegramAccountParams{
		ProfileID:         row.ProfileID,
		ConfirmedTgUserID: row.VerifiedTgUserID,
	})
	if err != nil {
		return false, err
	}
	if n > 0 {
		s.logger().Info("auth.telegram_link_dropped_on_reset", zap.String("profile_id", row.ProfileID.String()))
	}
	return n > 0, nil
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
