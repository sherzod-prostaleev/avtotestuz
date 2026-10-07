package auth

import (
	"context"
	"errors"
	"strconv"
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

func (s *Service) logLinkSkipped(profileID uuid.UUID, err error) {
	s.logger().Warn("auth.telegram_link_skipped", zap.String("profile_id", profileID.String()), zap.Error(err))
}

// linkTelegramInTx links the Mini App's Telegram account to profileID inside
// the caller's sign-in transaction. It never fails the sign-in: a person who
// typed the right phone and password is signed in even if the Telegram half
// is unusable, they just are not linked (logged for diagnosis).
func (s *Service) linkTelegramInTx(ctx context.Context, tx pgx.Tx, profileID uuid.UUID, initData string) bool {
	if initData == "" {
		return false
	}
	u, err := ValidateInitData(initData, s.TelegramBotToken, s.clock(), InitDataLinkMaxAge)
	if err != nil {
		s.logLinkSkipped(profileID, err)
		return false
	}
	// A nested tx (SAVEPOINT) keeps a failed link — e.g. a concurrent link of
	// the same Telegram account hitting the unique constraint — from aborting
	// the sign-in transaction around it.
	sp, err := tx.Begin(ctx)
	if err != nil {
		s.logLinkSkipped(profileID, err)
		return false
	}
	defer func() { _ = sp.Rollback(ctx) }()
	q := sqlc.New(sp)

	if prev, err := q.GetTelegramAccountByTgUserID(ctx, u.ID); err == nil && prev.ProfileID != profileID {
		s.logger().Info("auth.telegram_link_moved",
			zap.String("from_profile_id", prev.ProfileID.String()),
			zap.String("to_profile_id", profileID.String()))
	}
	if err := q.DeleteTelegramAccountForOtherProfiles(ctx, sqlc.DeleteTelegramAccountForOtherProfilesParams{TgUserID: u.ID, ProfileID: profileID}); err != nil {
		s.logLinkSkipped(profileID, err)
		return false
	}
	if err := q.UpsertTelegramAccount(ctx, sqlc.UpsertTelegramAccountParams{ProfileID: profileID, TgUserID: u.ID, Username: u.Username}); err != nil {
		s.logLinkSkipped(profileID, err)
		return false
	}
	if err := sp.Commit(ctx); err != nil {
		s.logLinkSkipped(profileID, err)
		return false
	}
	return true
}
