package auth

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"avtotest.uz/backend/internal/db/sqlc"
)

type WebAppLoginResult struct {
	Tokens
	Profile   sqlc.Profile
	NeedPhone bool
	FirstName string
}

// TelegramWebAppLogin exchanges a Mini App launch payload for a session when
// the Telegram account is linked to a profile. Unlinked users get NeedPhone
// and go through the ordinary phone sign-in/registration (spec §1.2) — a
// Telegram identity never creates a profile on its own.
func (s *Service) TelegramWebAppLogin(ctx context.Context, initData, ip string) (WebAppLoginResult, error) {
	// The IP limit runs first so a flood of forged init data is throttled
	// before we spend HMAC work on it; the per-user limit needs a valid id.
	if err := s.rateLimitTelegramIP(ctx, ip); err != nil {
		return WebAppLoginResult{}, err
	}
	if !s.miniAppEnabled() {
		return WebAppLoginResult{}, ErrTelegramBotUnconfigured
	}
	u, err := ValidateInitData(initData, s.TelegramBotToken, s.clock(), InitDataMaxAge)
	if err != nil {
		return WebAppLoginResult{}, err
	}
	if err := s.rateLimitTelegramUser(ctx, u.ID); err != nil {
		return WebAppLoginResult{}, err
	}
	account, err := s.Q.GetTelegramAccountByTgUserID(ctx, u.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return WebAppLoginResult{NeedPhone: true, FirstName: u.FirstName}, nil
	}
	if err != nil {
		return WebAppLoginResult{}, err
	}
	profile, err := s.Q.GetProfileByID(ctx, account.ProfileID)
	if err != nil {
		return WebAppLoginResult{}, err
	}
	if err := assertProfileActive(profile); err != nil {
		return WebAppLoginResult{}, err
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return WebAppLoginResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// issueSession adds one refresh-token row, exactly like a new device; it
	// never revokes other sessions.
	toks, err := s.issueSession(ctx, sqlc.New(tx), profile)
	if err != nil {
		return WebAppLoginResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return WebAppLoginResult{}, err
	}
	return WebAppLoginResult{Tokens: toks, Profile: profile, FirstName: u.FirstName}, nil
}

func (s *Service) rateLimitTelegramIP(ctx context.Context, ip string) error {
	if ip == "" {
		return nil
	}
	// Generous on purpose: Uzbek mobile carriers put many phones behind one
	// CGNAT IP and a classroom shares one Wi-Fi IP. The per-Telegram-user limit
	// (30/h) is the real per-person brake; this only caps garbage floods.
	ok, err := s.Lim.Allow(ctx, "tgwebapp:ip:"+ip, 300, time.Hour)
	if err != nil {
		return err
	}
	if !ok {
		return ErrRateLimited
	}
	return nil
}

func (s *Service) rateLimitTelegramUser(ctx context.Context, tgUserID int64) error {
	ok, err := s.Lim.Allow(ctx, "tgwebapp:tg:"+strconv.FormatInt(tgUserID, 10), 30, time.Hour)
	if err != nil {
		return err
	}
	if !ok {
		return ErrRateLimited
	}
	return nil
}

// miniAppEnabled is the kill switch (spec §1.4): clearing TELEGRAM_WEBAPP_URL
// must stop Mini App sign-in and linking, not just hide the menu button —
// otherwise a cached launch keeps working after the operator turned it off.
func (s *Service) miniAppEnabled() bool {
	return strings.TrimSpace(s.TelegramBotToken) != "" && strings.TrimSpace(s.TelegramWebAppURL) != ""
}

// Reasons a Mini App sign-in did not link, for auth.telegram_link_skipped.
var (
	errLinkNoContact     = errors.New("no telegram-signed contact")
	errLinkOtherUser     = errors.New("contact belongs to another telegram user")
	errLinkPhoneMismatch = errors.New("telegram phone is not the profile phone")
)

func (s *Service) logLinkSkipped(profileID uuid.UUID, err error) {
	s.logger().Warn("auth.telegram_link_skipped", zap.String("profile_id", profileID.String()), zap.Error(err))
}

// linkTelegramInTx links the Mini App's Telegram account to profile inside
// the caller's transaction. Launch data alone is NOT enough: anyone can copy
// their own fresh initData into a link (#tgWebAppData=…) that a victim opens
// in a browser, and a sign-in would then hand the attacker's Telegram the
// victim's account (and the bot's password reset). So the link also needs
// Telegram's own signature over a phone number — the requestContact response
// — for the same Telegram user, equal to the profile's phone. An attacker can
// only obtain that for their own number.
//
// It never fails the sign-in: a person who typed the right phone and
// password is signed in even if the Telegram half is unusable, they just are
// not linked (logged for diagnosis).
func (s *Service) linkTelegramInTx(ctx context.Context, tx pgx.Tx, profile sqlc.Profile, initData, contact string) bool {
	if initData == "" || !s.miniAppEnabled() {
		return false
	}
	u, err := ValidateInitData(initData, s.TelegramBotToken, s.clock(), InitDataLinkMaxAge)
	if err != nil {
		s.logLinkSkipped(profile.ID, err)
		return false
	}
	if contact == "" {
		s.logLinkSkipped(profile.ID, errLinkNoContact)
		return false
	}
	c, err := ValidateContact(contact, s.TelegramBotToken, s.clock(), InitDataLinkMaxAge)
	if err != nil {
		s.logLinkSkipped(profile.ID, err)
		return false
	}
	if c.UserID != u.ID {
		s.logLinkSkipped(profile.ID, errLinkOtherUser)
		return false
	}
	if phone, err := NormalizeTelegramContactPhone(c.Phone); err != nil || phone != profile.Phone {
		s.logLinkSkipped(profile.ID, errLinkPhoneMismatch)
		return false
	}
	// A nested tx (SAVEPOINT) keeps a failed link — e.g. a concurrent link of
	// the same Telegram account hitting the unique constraint — from aborting
	// the sign-in transaction around it.
	sp, err := tx.Begin(ctx)
	if err != nil {
		s.logLinkSkipped(profile.ID, err)
		return false
	}
	defer func() { _ = sp.Rollback(ctx) }()
	q := sqlc.New(sp)

	// Both re-pointings are the phone's proven owner acting on their own link.
	if prev, err := q.GetTelegramAccountByTgUserID(ctx, u.ID); err == nil && prev.ProfileID != profile.ID {
		s.logger().Info("auth.telegram_link_moved",
			zap.String("from_profile_id", prev.ProfileID.String()),
			zap.String("to_profile_id", profile.ID.String()))
	}
	if prev, err := q.GetTelegramAccountByProfileID(ctx, profile.ID); err == nil && prev.TgUserID != u.ID {
		s.logger().Info("auth.telegram_link_replaced", zap.String("profile_id", profile.ID.String()))
	}
	if err := q.DeleteTelegramAccountForOtherProfiles(ctx, sqlc.DeleteTelegramAccountForOtherProfilesParams{TgUserID: u.ID, ProfileID: profile.ID}); err != nil {
		s.logLinkSkipped(profile.ID, err)
		return false
	}
	if err := q.UpsertTelegramAccount(ctx, sqlc.UpsertTelegramAccountParams{ProfileID: profile.ID, TgUserID: u.ID, Username: u.Username}); err != nil {
		s.logLinkSkipped(profile.ID, err)
		return false
	}
	if err := sp.Commit(ctx); err != nil {
		s.logLinkSkipped(profile.ID, err)
		return false
	}
	return true
}

// linkWebAppPerProfileLimit caps POST /me/telegram/link-webapp per learner.
// A real learner calls it once after sign-in; the cap only keeps a script on
// a stolen session from spending HMAC work and DB writes in a loop.
const linkWebAppPerProfileLimit = 20

// LinkTelegramWebApp links the launching Telegram account to an already
// signed-in learner under the same rule as a Mini App sign-in (see
// linkTelegramInTx). It is for learners who typed their phone instead of
// sharing it: afterwards the Mini App asks Telegram for the number once. A
// proof that does not hold is (false, nil), not an error — the caller just
// stays unlinked.
func (s *Service) LinkTelegramWebApp(ctx context.Context, profileID uuid.UUID, initData, contact string) (bool, error) {
	if !s.miniAppEnabled() {
		return false, ErrTelegramBotUnconfigured
	}
	ok, err := s.Lim.Allow(ctx, "tglink:profile:"+profileID.String(), linkWebAppPerProfileLimit, time.Hour)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, ErrRateLimited
	}
	if u, err := ValidateInitData(initData, s.TelegramBotToken, s.clock(), InitDataLinkMaxAge); err == nil {
		if err := s.rateLimitTelegramUser(ctx, u.ID); err != nil {
			return false, err
		}
	}
	profile, err := s.Q.GetProfileByID(ctx, profileID)
	if err != nil {
		return false, err
	}
	if err := assertProfileActive(profile); err != nil {
		return false, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	linked := s.linkTelegramInTx(ctx, tx, profile, initData, contact)
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return linked, nil
}
