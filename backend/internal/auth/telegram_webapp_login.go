package auth

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
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
	// The IP check runs first so an IP that already sent a flood of forged
	// init data is refused before we spend HMAC work on it; the per-user
	// limit needs a valid id.
	if err := s.checkTelegramIPFailures(ctx, ip); err != nil {
		return WebAppLoginResult{}, err
	}
	if !s.miniAppEnabled() {
		return WebAppLoginResult{}, ErrTelegramBotUnconfigured
	}
	u, err := ValidateInitData(initData, s.TelegramBotToken, s.clock(), InitDataMaxAge)
	if err != nil {
		if limErr := s.noteTelegramIPFailure(ctx, ip); limErr != nil {
			return WebAppLoginResult{}, limErr
		}
		return WebAppLoginResult{}, err
	}
	if err := s.rateLimitTelegramUser(ctx, u.ID); err != nil {
		return WebAppLoginResult{}, err
	}
	// Linked or not, a validated launch is a reachable Telegram user: the
	// unlinked ones are exactly who the reminder's signup pitch is for.
	s.joinBotAudience(ctx, u)
	account, err := s.Q.GetTelegramAccountByTgUserID(ctx, u.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return WebAppLoginResult{NeedPhone: true, FirstName: u.FirstName}, nil
	}
	if err != nil {
		return WebAppLoginResult{}, err
	}
	// Only a link Telegram vouched for (a signed phone share, or the bot
	// reset's contact + «Ha, men») stands in for the password. A legacy
	// /start <token> link proves nothing about who owns the profile: a token
	// minted on an attacker's profile binds the victim's Telegram to it, and
	// an intruder's link would outlive the owner's password reset. Those
	// learners share their phone once; the row itself stays for bot digests.
	if !account.PhoneVerifiedAt.Valid {
		return WebAppLoginResult{NeedPhone: true, FirstName: u.FirstName}, nil
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

// telegramIPFailureLimit caps FAILED init data validations per IP per hour.
// Only failures count: Uzbek mobile carriers put many phones behind one CGNAT
// IP and a classroom shares one Wi-Fi IP, so successful sign-ins are limited
// per Telegram user (30/h) only. This bucket just brakes forged-payload floods.
const telegramIPFailureLimit = 300

func telegramIPFailureKey(ip string) string { return "tgwebapp:ip:" + ip }

// checkTelegramIPFailures refuses an IP whose failure budget is spent,
// without counting this request.
func (s *Service) checkTelegramIPFailures(ctx context.Context, ip string) error {
	if ip == "" {
		return nil
	}
	n, err := s.Lim.Count(ctx, telegramIPFailureKey(ip))
	if err != nil {
		return err
	}
	if n >= telegramIPFailureLimit {
		return ErrRateLimited
	}
	return nil
}

// noteTelegramIPFailure counts one failed validation against ip.
func (s *Service) noteTelegramIPFailure(ctx context.Context, ip string) error {
	if ip == "" {
		return nil
	}
	_, err := s.Lim.Allow(ctx, telegramIPFailureKey(ip), telegramIPFailureLimit, time.Hour)
	return err
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

// logLinkSkipped is Warn for a proof that failed and Debug for no contact at
// all, which is simply a learner who typed their phone (the Mini App then
// asks for the share via link-webapp) and would otherwise flood the Warn log.
func (s *Service) logLinkSkipped(profileID uuid.UUID, err error) {
	level := zap.WarnLevel
	if errors.Is(err, errLinkNoContact) {
		level = zap.DebugLevel
	}
	s.logger().Log(level, "auth.telegram_link_skipped", zap.String("profile_id", profileID.String()), zap.Error(err))
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
// not linked (logged for diagnosis). The caller hands the result to
// afterTelegramLink once its transaction has committed.
func (s *Service) linkTelegramInTx(ctx context.Context, tx pgx.Tx, profile sqlc.Profile, initData, contact string) telegramLinkChange {
	none := telegramLinkChange{}
	if initData == "" || !s.miniAppEnabled() {
		return none
	}
	u, err := ValidateInitData(initData, s.TelegramBotToken, s.clock(), InitDataLinkMaxAge)
	if err != nil {
		s.logLinkSkipped(profile.ID, err)
		return none
	}
	if contact == "" {
		s.logLinkSkipped(profile.ID, errLinkNoContact)
		return none
	}
	c, err := ValidateContact(contact, s.TelegramBotToken, s.clock(), InitDataLinkMaxAge)
	if err != nil {
		s.logLinkSkipped(profile.ID, err)
		return none
	}
	if c.UserID != u.ID {
		s.logLinkSkipped(profile.ID, errLinkOtherUser)
		return none
	}
	if phone, err := NormalizeTelegramContactPhone(c.Phone); err != nil || phone != profile.Phone {
		s.logLinkSkipped(profile.ID, errLinkPhoneMismatch)
		return none
	}
	return s.linkVerifiedTelegramInTx(ctx, tx, profile, u.ID, u.Username, &u)
}

// linkVerifiedTelegramInTx writes a phone-verified link from Telegram user
// tgUserID to profile inside the caller's transaction. The caller must hold
// Telegram's own proof that this Telegram user owns profile.Phone (a signed
// Mini App contact, or a contact the bot received from the user themselves).
// It is the one place that moves and replaces links, shared by the Mini App
// sign-in/link paths and the Telegram login (website + Mini App phone share).
// webAppUser is the signed Mini App user, when there is one (bot audience).
func (s *Service) linkVerifiedTelegramInTx(ctx context.Context, tx pgx.Tx, profile sqlc.Profile, tgUserID int64, username string, webAppUser *WebAppUser) telegramLinkChange {
	none := telegramLinkChange{}
	// A nested tx (SAVEPOINT) keeps a failed link — e.g. a concurrent link of
	// the same Telegram account hitting the unique constraint — from aborting
	// the sign-in transaction around it.
	sp, err := tx.Begin(ctx)
	if err != nil {
		s.logLinkSkipped(profile.ID, err)
		return none
	}
	defer func() { _ = sp.Rollback(ctx) }()
	q := sqlc.New(sp)

	// Both re-pointings are the phone's proven owner acting on their own link.
	movedFrom := uuid.Nil
	if prev, err := q.GetTelegramAccountByTgUserID(ctx, tgUserID); err == nil && prev.ProfileID != profile.ID {
		s.logger().Info("auth.telegram_link_moved",
			zap.String("from_profile_id", prev.ProfileID.String()),
			zap.String("to_profile_id", profile.ID.String()))
		movedFrom = prev.ProfileID
	}
	if prev, err := q.GetTelegramAccountByProfileID(ctx, profile.ID); err == nil && prev.TgUserID != tgUserID {
		s.logger().Info("auth.telegram_link_replaced", zap.String("profile_id", profile.ID.String()))
	}
	if err := q.DeleteTelegramAccountForOtherProfiles(ctx, sqlc.DeleteTelegramAccountForOtherProfilesParams{TgUserID: tgUserID, ProfileID: profile.ID}); err != nil {
		s.logLinkSkipped(profile.ID, err)
		return none
	}
	if err := q.UpsertTelegramAccount(ctx, sqlc.UpsertTelegramAccountParams{ProfileID: profile.ID, TgUserID: tgUserID, Username: username, PhoneVerified: true}); err != nil {
		s.logLinkSkipped(profile.ID, err)
		return none
	}
	// Defence in depth: disarm any bot password reset pending for this
	// Telegram user, so a «Ha, men» question already sent for it stops
	// working once the learner is in the Mini App. It does NOT stop the
	// Mini App's phone share from reaching the bot first (the contact message
	// and this request race) and does not run for shares that do not link.
	// What actually keeps a stray contact from completing a reset is that a
	// contact never verifies on its own — see
	// AnswerTelegramPasswordResetConfirm.
	if err := q.ClearAllPasswordResetPendingForTg(ctx, pgtype.Int8{Int64: tgUserID, Valid: true}); err != nil {
		s.logLinkSkipped(profile.ID, err)
		return none
	}
	// Same for a website Telegram login opened in the bot: a phone share
	// that already served the Mini App is not consent to that login.
	if err := q.ClearAllTelegramLoginPendingForTg(ctx, pgtype.Int8{Int64: tgUserID, Valid: true}); err != nil {
		s.logLinkSkipped(profile.ID, err)
		return none
	}
	if err := sp.Commit(ctx); err != nil {
		s.logLinkSkipped(profile.ID, err)
		return none
	}
	return telegramLinkChange{linked: true, profileID: profile.ID, movedFrom: movedFrom, webAppUser: webAppUser}
}

// joinBotAudience adds a Mini App user to the daily reminder's audience
// (telegram_bot_user) when their signed launch data says the bot may message
// them (allows_write_to_pm). Without it a learner who only ever uses the
// Mini App would never get the reminder: the registry otherwise learns
// about people from messages they type to the bot. It never fails the
// caller — a missed row only means a missed reminder — and the query skips
// the write when the row was touched within the last minute.
func (s *Service) joinBotAudience(ctx context.Context, u WebAppUser) {
	if !u.AllowsWriteToPM {
		return
	}
	if err := s.Q.UpsertTelegramBotUserFromWebApp(ctx, sqlc.UpsertTelegramBotUserFromWebAppParams{
		TgUserID:     u.ID,
		FirstName:    clipRunes(u.FirstName, 64),
		Username:     clipRunes(u.Username, 64),
		LanguageCode: clipRunes(u.LanguageCode, 16),
	}); err != nil {
		s.logger().Warn("auth.telegram_bot_user_upsert_failed", zap.Error(err))
	}
}

// clipRunes bounds a Telegram-supplied string to the same lengths the bot's
// own registry upsert uses (bot.truncateRunes; auth cannot import bot).
func clipRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
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
	link := s.linkTelegramInTx(ctx, tx, profile, initData, contact)
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	s.afterTelegramLink(ctx, link)
	return link.linked, nil
}
