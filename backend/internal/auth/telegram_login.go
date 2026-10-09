package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	"avtotest.uz/backend/internal/db/sqlc"
)

// «Telegram orqali kirish»: a browser (website / native app) signs in, or
// signs up, by approving the request in our bot. See migration 0080 for the
// storage and .superpowers/sdd/tglogin-report.md for the threat model.
//
//  1. StartTelegramLogin — the browser gets a t.me/<bot>?start=login_<token>
//     link; the BFF keeps the browser secret in an HttpOnly cookie.
//  2. BeginTelegramLogin — the bot got /start login_<token>. A Telegram user
//     with a phone-verified link is asked «✅ Kirish»; anyone else is asked to
//     share their own number, and that share is the consent.
//  3. ConfirmTelegramLoginContact / AnswerTelegramLoginConfirm — approve:
//     the profile is found by phone (kind='user' only) or created exactly like
//     Register creates one, without a password.
//  4. TelegramLoginStatus / CompleteTelegramLogin — the browser polls, then
//     trades the approved request (token + browser secret) for a session once.

const (
	// TelegramLoginTTL bounds the whole dance: open the bot, tap, come back.
	TelegramLoginTTL = 5 * time.Minute
	// telegramLoginCompleteGrace lets an approval that landed in the last
	// poll interval before expiry still be completed.
	telegramLoginCompleteGrace = 2 * time.Minute

	// TelegramLoginStartPrefix keeps login payloads apart from password
	// resets (pwr_) and legacy link tokens. "login_" + 43-char token = 49
	// bytes, inside Telegram's 64-byte start parameter.
	TelegramLoginStartPrefix = "login_"

	telegramLoginExpiresSec = int(TelegramLoginTTL / time.Second)

	// Abuse limits. Start is per IP (CGNAT and classrooms share one) with a
	// global ceiling; status is polled every 2 s so it is bounded per request
	// and generously per IP; complete reuses the auth limiter.
	telegramLoginStartPerIP     = 30
	telegramLoginStartGlobal    = 3000
	telegramLoginStatusPerToken = 300
	telegramLoginStatusPerIP    = 6000
	// Bot-side steps (open link, share phone, tap) per Telegram user.
	telegramLoginBotPerUser = 30

	// telegramNameMaxRunes caps a name taken from Telegram for a new profile.
	telegramNameMaxRunes = 64
)

var (
	ErrTelegramLoginInvalid     = errors.New("telegram login request invalid")
	ErrTelegramLoginNotApproved = errors.New("telegram login request not approved yet")
)

// Website states (TelegramLoginStatus). Nothing else is ever revealed before
// completion — no name, no phone, not even whether an account existed.
const (
	TelegramLoginStatePending   = "pending"
	TelegramLoginStateApproved  = "approved"
	TelegramLoginStateCancelled = "cancelled"
	TelegramLoginStateBlocked   = "blocked"
	TelegramLoginStateInvalid   = "invalid"
)

// Bot outcomes.
const (
	TelegramLoginInvalid       = "invalid"       // unknown, expired or used link
	TelegramLoginNeedContact   = "need_contact"  // ask for the user's own phone
	TelegramLoginNeedConfirm   = "need_confirm"  // ask «✅ Kirish» / «✖️ Bekor qilish»
	TelegramLoginApproved      = "approved"      // done; tell them to go back
	TelegramLoginCancelled     = "cancelled"     // they said no
	TelegramLoginBlocked       = "blocked"       // the account is banned
	TelegramLoginStale         = "stale"         // a tap that no longer applies
	TelegramLoginNone          = "none"          // a contact with no login waiting
	TelegramLoginNotOwnContact = "not_own"       // a forwarded/other person's card
	TelegramLoginForeignPhone  = "foreign_phone" // not +998
	TelegramLoginRateLimited   = "rate_limited"  // too many bot steps
	TelegramLoginFailed        = "failed"        // could not be approved (e.g. phone held by a station row)
	telegramLoginStatusPending = TelegramLoginStatePending
)

// TelegramLoginStart is what the browser needs. BrowserSecret goes into an
// HttpOnly cookie at the BFF and never reaches page script.
type TelegramLoginStart struct {
	BotURL        string `json:"bot_url"`
	Token         string `json:"token"`
	BrowserSecret string `json:"browser_secret"`
	ExpiresInSec  int    `json:"expires_in_sec"`
}

// TelegramLoginUser is the Telegram user in the bot chat, as the update
// (webhook / long-poll) reports them — never anything the browser sent.
type TelegramLoginUser struct {
	ID           int64
	Username     string
	FirstName    string
	LastName     string
	LanguageCode string
}

// TelegramLoginBegin is a bot step's result.
type TelegramLoginBegin struct {
	Outcome string
	// Device: "Chrome · Android" or "" (unknown) — for the bot prompt.
	Device string
	// ConfirmNonce (need_confirm only) is fresh randomness for callback_data,
	// never the token: callback data is visible to Telegram clients.
	ConfirmNonce string
	MaskedPhone  string
	// Created: this approval created the profile.
	Created bool
}

var botUsernameRE = regexp.MustCompile(`^[A-Za-z0-9_]{5,32}$`)

func telegramLoginDeepLink(botUsername, token string) string {
	return "https://t.me/" + botUsername + "?start=" + TelegramLoginStartPrefix + token
}

// ParseTelegramLoginStartPayload extracts the token of a /start login_<token>.
func ParseTelegramLoginStartPayload(arg string) (string, bool) {
	raw, ok := strings.CutPrefix(arg, TelegramLoginStartPrefix)
	if !ok || raw == "" {
		return "", false
	}
	return raw, true
}

// StartTelegramLogin creates a login request for the calling browser.
func (s *Service) StartTelegramLogin(ctx context.Context, ip, userAgent, botUsername string) (TelegramLoginStart, error) {
	bot := strings.TrimPrefix(strings.TrimSpace(botUsername), "@")
	if !botUsernameRE.MatchString(bot) {
		return TelegramLoginStart{}, ErrTelegramBotUnconfigured
	}
	if ip != "" {
		if err := s.allow(ctx, "tglogin:start:ip:"+ip, telegramLoginStartPerIP, time.Hour); err != nil {
			return TelegramLoginStart{}, err
		}
	}
	if err := s.allow(ctx, "tglogin:start:all", telegramLoginStartGlobal, time.Hour); err != nil {
		return TelegramLoginStart{}, err
	}
	token, err := NewRefreshToken()
	if err != nil {
		return TelegramLoginStart{}, err
	}
	secret, err := NewRefreshToken()
	if err != nil {
		return TelegramLoginStart{}, err
	}
	if err := s.Q.DeleteStaleTelegramLoginRequests(ctx); err != nil {
		// Retention only; never worth failing a sign-in over.
		s.logger().Warn("auth.telegram_login_sweep_failed", zap.Error(err))
	}
	if _, err := s.Q.CreateTelegramLoginRequest(ctx, sqlc.CreateTelegramLoginRequestParams{
		TokenHash:         HashToken(token),
		BrowserSecretHash: HashToken(secret),
		Device:            DescribeDevice(userAgent),
		ExpiresAt:         pgtype.Timestamptz{Time: time.Now().Add(TelegramLoginTTL), Valid: true},
	}); err != nil {
		return TelegramLoginStart{}, err
	}
	return TelegramLoginStart{
		BotURL:        telegramLoginDeepLink(bot, token),
		Token:         token,
		BrowserSecret: secret,
		ExpiresInSec:  telegramLoginExpiresSec,
	}, nil
}

func (s *Service) allow(ctx context.Context, key string, limit int, window time.Duration) error {
	ok, err := s.Lim.Allow(ctx, key, limit, window)
	if err != nil {
		return err
	}
	if !ok {
		return ErrRateLimited
	}
	return nil
}

// browserOwns compares digests in constant time: the browser secret is the
// half of the credential an attacker holding only the link does not have.
func browserOwns(row sqlc.TelegramLoginRequest, secret string) bool {
	if secret == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(HashToken(secret)), []byte(row.BrowserSecretHash)) == 1
}

func telegramLoginLive(row sqlc.TelegramLoginRequest, now time.Time) bool {
	return row.ExpiresAt.Valid && now.Before(row.ExpiresAt.Time)
}

func telegramLoginCompletable(row sqlc.TelegramLoginRequest, now time.Time) bool {
	return row.Status == TelegramLoginStateApproved && row.ExpiresAt.Valid &&
		now.Before(row.ExpiresAt.Time.Add(telegramLoginCompleteGrace))
}

// TelegramLoginStatus is the waiting browser's poll. Anything that does not
// belong to this browser, or no longer leads anywhere, is "invalid".
func (s *Service) TelegramLoginStatus(ctx context.Context, token, secret, ip string) (string, error) {
	if strings.TrimSpace(token) == "" {
		return TelegramLoginStateInvalid, nil
	}
	if err := s.allow(ctx, "tglogin:status:"+HashToken(token), telegramLoginStatusPerToken, 10*time.Minute); err != nil {
		return "", err
	}
	if ip != "" {
		if err := s.allow(ctx, "tglogin:status:ip:"+ip, telegramLoginStatusPerIP, time.Hour); err != nil {
			return "", err
		}
	}
	row, err := s.Q.GetTelegramLoginRequestByTokenHash(ctx, HashToken(token))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TelegramLoginStateInvalid, nil
		}
		return "", err
	}
	if !browserOwns(row, secret) {
		return TelegramLoginStateInvalid, nil
	}
	now := time.Now()
	switch row.Status {
	case telegramLoginStatusPending:
		if telegramLoginLive(row, now) {
			return TelegramLoginStatePending, nil
		}
	case TelegramLoginStateApproved:
		if telegramLoginCompletable(row, now) {
			return TelegramLoginStateApproved, nil
		}
	case TelegramLoginStateCancelled, TelegramLoginStateBlocked:
		return row.Status, nil
	}
	return TelegramLoginStateInvalid, nil
}

// CompleteTelegramLogin trades an approved request for a session, once, and
// only for the browser that started it.
func (s *Service) CompleteTelegramLogin(ctx context.Context, token, secret, ip string) (VerifyResult, error) {
	if strings.TrimSpace(token) == "" {
		return VerifyResult{}, ErrTelegramLoginInvalid
	}
	if err := s.rateLimitAuth(ctx, "tglogin_complete", HashToken(token), ip); err != nil {
		return VerifyResult{}, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return VerifyResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := sqlc.New(tx)

	row, err := q.GetTelegramLoginRequestByTokenHashForUpdate(ctx, HashToken(token))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return VerifyResult{}, ErrTelegramLoginInvalid
		}
		return VerifyResult{}, err
	}
	now := time.Now()
	if !browserOwns(row, secret) {
		return VerifyResult{}, ErrTelegramLoginInvalid
	}
	if row.Status == telegramLoginStatusPending && telegramLoginLive(row, now) {
		return VerifyResult{}, ErrTelegramLoginNotApproved
	}
	if !telegramLoginCompletable(row, now) || !row.ProfileID.Valid {
		return VerifyResult{}, ErrTelegramLoginInvalid
	}
	profile, err := q.GetProfileByID(ctx, row.ProfileID.UUID)
	if err != nil {
		return VerifyResult{}, err
	}
	if profile.Kind != profileKindUser {
		return VerifyResult{}, ErrTelegramLoginInvalid
	}
	// Banned between approval and completion: refuse, like every sign-in.
	toks, err := s.issueSession(ctx, q, profile)
	if err != nil {
		return VerifyResult{}, err
	}
	if n, err := q.ConsumeTelegramLoginRequest(ctx, row.ID); err != nil {
		return VerifyResult{}, err
	} else if n != 1 {
		return VerifyResult{}, ErrTelegramLoginInvalid
	}
	if err := tx.Commit(ctx); err != nil {
		return VerifyResult{}, err
	}
	// "Created" = the profile is no older than the request: it was made by
	// this very approval (a new learner lands on onboarding-friendly pages).
	created := profile.CreatedAt.Valid && row.CreatedAt.Valid && !profile.CreatedAt.Time.Before(row.CreatedAt.Time)
	return VerifyResult{Tokens: toks, Profile: profile, Created: created}, nil
}

const profileKindUser = "user"

// BeginTelegramLogin handles /start login_<token> in a private chat.
func (s *Service) BeginTelegramLogin(ctx context.Context, rawToken string, who TelegramLoginUser) (TelegramLoginBegin, error) {
	if strings.TrimSpace(rawToken) == "" || who.ID <= 0 {
		return TelegramLoginBegin{Outcome: TelegramLoginInvalid}, nil
	}
	if err := s.allow(ctx, telegramLoginBotKey(who.ID), telegramLoginBotPerUser, time.Hour); err != nil {
		if errors.Is(err, ErrRateLimited) {
			return TelegramLoginBegin{Outcome: TelegramLoginRateLimited}, nil
		}
		return TelegramLoginBegin{}, err
	}
	res, err := s.beginTelegramLogin(ctx, rawToken, who)
	if isUniqueViolation(err) {
		// Two /start login_ of the same user raced on the one-pending-per-user
		// index; the retry sees the winner committed and clears it.
		res, err = s.beginTelegramLogin(ctx, rawToken, who)
	}
	return res, err
}

func telegramLoginBotKey(tgUserID int64) string {
	return "tglogin:tg:" + strconv.FormatInt(tgUserID, 10)
}

func (s *Service) beginTelegramLogin(ctx context.Context, rawToken string, who TelegramLoginUser) (TelegramLoginBegin, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return TelegramLoginBegin{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := sqlc.New(tx)

	row, err := q.GetTelegramLoginRequestByTokenHashForUpdate(ctx, HashToken(rawToken))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TelegramLoginBegin{Outcome: TelegramLoginInvalid}, nil
		}
		return TelegramLoginBegin{}, err
	}
	if row.Status == TelegramLoginStateApproved {
		return TelegramLoginBegin{Outcome: TelegramLoginApproved, Device: row.Device}, nil
	}
	if row.Status != telegramLoginStatusPending || !telegramLoginLive(row, time.Now()) {
		return TelegramLoginBegin{Outcome: TelegramLoginInvalid}, nil
	}
	tg := pgtype.Int8{Int64: who.ID, Valid: true}
	if err := q.ClearTelegramLoginPendingForTg(ctx, sqlc.ClearTelegramLoginPendingForTgParams{PendingTgUserID: tg, ID: row.ID}); err != nil {
		return TelegramLoginBegin{}, err
	}

	// Only a phone-verified link is identity. A legacy /start <token> link
	// proves nothing about who owns the profile, so it takes the phone step.
	profile, linked, err := verifiedLinkedProfile(ctx, q, who.ID)
	if err != nil {
		return TelegramLoginBegin{}, err
	}
	if !linked {
		if err := q.ArmTelegramLoginForTg(ctx, sqlc.ArmTelegramLoginForTgParams{ID: row.ID, PendingTgUserID: tg}); err != nil {
			return TelegramLoginBegin{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return TelegramLoginBegin{}, err
		}
		return TelegramLoginBegin{Outcome: TelegramLoginNeedContact, Device: row.Device}, nil
	}
	if assertProfileActive(profile) != nil {
		return s.endTelegramLogin(ctx, tx, q, row, TelegramLoginStateBlocked, TelegramLoginBlocked)
	}
	nonce, err := newResetConfirmNonce()
	if err != nil {
		return TelegramLoginBegin{}, err
	}
	if err := q.ArmTelegramLoginForTg(ctx, sqlc.ArmTelegramLoginForTgParams{
		ID: row.ID, PendingTgUserID: tg, ConfirmNonceHash: pgtype.Text{String: HashToken(nonce), Valid: true},
	}); err != nil {
		return TelegramLoginBegin{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return TelegramLoginBegin{}, err
	}
	return TelegramLoginBegin{
		Outcome:      TelegramLoginNeedConfirm,
		Device:       row.Device,
		ConfirmNonce: nonce,
		MaskedPhone:  MaskResetPhone(profile.Phone),
	}, nil
}

// verifiedLinkedProfile is the learner tgUserID is phone-verified-linked to.
func verifiedLinkedProfile(ctx context.Context, q *sqlc.Queries, tgUserID int64) (sqlc.Profile, bool, error) {
	account, err := q.GetTelegramAccountByTgUserID(ctx, tgUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Profile{}, false, nil
	}
	if err != nil {
		return sqlc.Profile{}, false, err
	}
	if !account.PhoneVerifiedAt.Valid {
		return sqlc.Profile{}, false, nil
	}
	profile, err := q.GetProfileByID(ctx, account.ProfileID)
	if err != nil {
		return sqlc.Profile{}, false, err
	}
	return profile, profile.Kind == profileKindUser, nil
}

func (s *Service) endTelegramLogin(ctx context.Context, tx pgx.Tx, q *sqlc.Queries, row sqlc.TelegramLoginRequest, status, outcome string) (TelegramLoginBegin, error) {
	if _, err := q.EndTelegramLoginRequest(ctx, sqlc.EndTelegramLoginRequestParams{ID: row.ID, Status: status}); err != nil {
		return TelegramLoginBegin{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return TelegramLoginBegin{}, err
	}
	return TelegramLoginBegin{Outcome: outcome, Device: row.Device}, nil
}

// ConfirmTelegramLoginContact handles a contact the user shared in the bot
// while a login request waits for them. Sharing their OWN number after the
// prompt (which names the device asking) is the consent: one tap for a first
// sign-in. It returns TelegramLoginNone when no login waits, so the caller
// can hand the contact to the password-reset flow instead.
func (s *Service) ConfirmTelegramLoginContact(ctx context.Context, who TelegramLoginUser, contactUserID int64, contactPhone string) (TelegramLoginBegin, error) {
	if who.ID <= 0 {
		return TelegramLoginBegin{Outcome: TelegramLoginNone}, nil
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return TelegramLoginBegin{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := sqlc.New(tx)

	row, err := q.GetLiveTelegramLoginByPendingTgForUpdate(ctx, pgtype.Int8{Int64: who.ID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TelegramLoginBegin{Outcome: TelegramLoginNone}, nil
		}
		return TelegramLoginBegin{}, err
	}
	// A «✅ Kirish» question is open: the tap, not a contact, answers it.
	if row.ConfirmNonceHash.Valid {
		return TelegramLoginBegin{Outcome: TelegramLoginNone}, nil
	}
	if contactUserID != who.ID {
		return TelegramLoginBegin{Outcome: TelegramLoginNotOwnContact, Device: row.Device}, nil
	}
	phone, err := NormalizeTelegramContactPhone(contactPhone)
	if err != nil {
		return TelegramLoginBegin{Outcome: TelegramLoginForeignPhone, Device: row.Device}, nil
	}
	if err := s.allow(ctx, telegramLoginBotKey(who.ID), telegramLoginBotPerUser, time.Hour); err != nil {
		if errors.Is(err, ErrRateLimited) {
			return TelegramLoginBegin{Outcome: TelegramLoginRateLimited}, nil
		}
		return TelegramLoginBegin{}, err
	}

	referral := s.pendingBotReferral(ctx, who.ID)
	profile, created, err := s.findOrCreateTelegramProfile(ctx, tx, phone, who, referral)
	switch {
	case errors.Is(err, ErrAccountBlocked):
		return s.endTelegramLogin(ctx, tx, q, row, TelegramLoginStateBlocked, TelegramLoginBlocked)
	case errors.Is(err, errPhoneNotALearner):
		s.logger().Warn("auth.telegram_login_phone_not_learner", zap.String("request_id", row.ID.String()))
		return TelegramLoginBegin{Outcome: TelegramLoginFailed}, nil
	case err != nil:
		return TelegramLoginBegin{}, err
	}
	link := s.linkVerifiedTelegramInTx(ctx, tx, profile, who.ID, who.Username, nil)
	if n, err := q.ApproveTelegramLoginRequest(ctx, sqlc.ApproveTelegramLoginRequestParams{
		ID:               row.ID,
		ProfileID:        uuid.NullUUID{UUID: profile.ID, Valid: true},
		ApprovedTgUserID: pgtype.Int8{Int64: who.ID, Valid: true},
	}); err != nil {
		return TelegramLoginBegin{}, err
	} else if n != 1 {
		return TelegramLoginBegin{Outcome: TelegramLoginStale}, nil
	}
	if err := tx.Commit(ctx); err != nil {
		return TelegramLoginBegin{}, err
	}
	s.afterTelegramLink(ctx, link)
	s.logger().Info("auth.telegram_login_approved",
		zap.String("profile_id", profile.ID.String()), zap.Bool("created", created), zap.String("via", "contact"))
	return TelegramLoginBegin{Outcome: TelegramLoginApproved, Device: row.Device, Created: created}, nil
}

// AnswerTelegramLoginConfirm handles «✅ Kirish» (accept) or «✖️ Bekor
// qilish». Only the Telegram user the request is pending for, with the
// current nonce, on a live pending request, changes anything.
func (s *Service) AnswerTelegramLoginConfirm(ctx context.Context, tgUserID int64, nonce string, accept bool) (TelegramLoginBegin, error) {
	stale := TelegramLoginBegin{Outcome: TelegramLoginStale}
	if tgUserID <= 0 || strings.TrimSpace(nonce) == "" {
		return stale, nil
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return TelegramLoginBegin{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := sqlc.New(tx)

	row, err := q.GetTelegramLoginByConfirmNonceForUpdate(ctx, pgtype.Text{String: HashToken(nonce), Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return stale, nil
		}
		return TelegramLoginBegin{}, err
	}
	if row.Status != telegramLoginStatusPending || !telegramLoginLive(row, time.Now()) ||
		!row.PendingTgUserID.Valid || row.PendingTgUserID.Int64 != tgUserID {
		return stale, nil
	}
	if !accept {
		return s.endTelegramLogin(ctx, tx, q, row, TelegramLoginStateCancelled, TelegramLoginCancelled)
	}
	// Re-resolved at the tap: an unlink since the question means no shortcut.
	profile, linked, err := verifiedLinkedProfile(ctx, q, tgUserID)
	if err != nil {
		return TelegramLoginBegin{}, err
	}
	if !linked {
		return stale, nil
	}
	if assertProfileActive(profile) != nil {
		return s.endTelegramLogin(ctx, tx, q, row, TelegramLoginStateBlocked, TelegramLoginBlocked)
	}
	if n, err := q.ApproveTelegramLoginRequest(ctx, sqlc.ApproveTelegramLoginRequestParams{
		ID:               row.ID,
		ProfileID:        uuid.NullUUID{UUID: profile.ID, Valid: true},
		ApprovedTgUserID: pgtype.Int8{Int64: tgUserID, Valid: true},
	}); err != nil {
		return TelegramLoginBegin{}, err
	} else if n != 1 {
		return stale, nil
	}
	if err := tx.Commit(ctx); err != nil {
		return TelegramLoginBegin{}, err
	}
	s.logger().Info("auth.telegram_login_approved",
		zap.String("profile_id", profile.ID.String()), zap.Bool("created", false), zap.String("via", "confirm"))
	return TelegramLoginBegin{Outcome: TelegramLoginApproved, Device: row.Device}, nil
}

// errPhoneNotALearner: the phone is taken by a row that is not a learner (a
// station). Never signed in to, never "fixed" — logged and refused.
var errPhoneNotALearner = errors.New("phone belongs to a non-learner profile")

// findOrCreateTelegramProfile returns the learner (kind='user') whose phone
// Telegram just vouched for, creating one exactly as Register does — same
// helper, same signup trial — but with no password and the Telegram name. It
// runs in the caller's transaction; a concurrent creation of the same phone
// resolves to the winner's row (the unique index makes the loser wait for the
// winner's commit, after which the re-read sees it). A banned learner is
// ErrAccountBlocked. referralCode ("" for none) is attached to a NEW profile
// only.
func (s *Service) findOrCreateTelegramProfile(ctx context.Context, tx pgx.Tx, phone string, who TelegramLoginUser, referralCode string) (sqlc.Profile, bool, error) {
	q := sqlc.New(tx)
	existing := func() (sqlc.Profile, bool, error) {
		p, err := q.GetUserProfileByPhone(ctx, phone)
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.Profile{}, false, errPhoneNotALearner
		}
		if err != nil {
			return sqlc.Profile{}, false, err
		}
		if err := assertProfileActive(p); err != nil {
			return sqlc.Profile{}, false, err
		}
		return p, false, nil
	}
	p, err := q.GetUserProfileByPhone(ctx, phone)
	if err == nil {
		if err := assertProfileActive(p); err != nil {
			return sqlc.Profile{}, false, err
		}
		return p, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Profile{}, false, err
	}

	sp, err := tx.Begin(ctx)
	if err != nil {
		return sqlc.Profile{}, false, err
	}
	defer func() { _ = sp.Rollback(ctx) }()
	qs := sqlc.New(sp)
	p, err = createProfileWithReferral(ctx, qs, phone, "", telegramProfileName(who))
	if err != nil {
		if phoneTaken(err) {
			// Lost the race (or the phone is a station's): back to the
			// savepoint, then read what is there now.
			if rbErr := sp.Rollback(ctx); rbErr != nil {
				return sqlc.Profile{}, false, rbErr
			}
			return existing()
		}
		return sqlc.Profile{}, false, err
	}
	if err := grantSignupTrial(ctx, qs, p.ID); err != nil {
		return sqlc.Profile{}, false, err
	}
	if err := sp.Commit(ctx); err != nil {
		return sqlc.Profile{}, false, err
	}
	s.applySignupReferral(ctx, tx, p.ID, referralCode)
	if who.ID > 0 {
		// Spent on this account whatever the outcome; it was for a first one.
		if err := q.ClearTelegramBotUserPendingReferral(ctx, who.ID); err != nil {
			return sqlc.Profile{}, false, err
		}
	}
	s.logger().Info("auth.telegram_profile_created", zap.String("profile_id", p.ID.String()))
	return p, true, nil
}

// telegramProfileName is the new profile's name: Telegram's first + last
// name, control characters dropped, trimmed, capped.
func telegramProfileName(who TelegramLoginUser) string {
	full := strings.TrimSpace(strings.TrimSpace(who.FirstName) + " " + strings.TrimSpace(who.LastName))
	full = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, full)
	full = strings.TrimSpace(full)
	if utf8.RuneCountInString(full) > telegramNameMaxRunes {
		full = strings.TrimSpace(string([]rune(full)[:telegramNameMaxRunes]))
	}
	return full
}

var referralCodeRE = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// ReferralStartPrefix marks a referral in a start / startapp parameter.
const ReferralStartPrefix = "ref_"

// ParseReferralStartParam reads ref_<CODE> (t.me/<bot>?startapp=ref_<CODE> or
// /start ref_<CODE>). Telegram caps the whole parameter at 64 bytes of
// [A-Za-z0-9_-]; anything else is not a referral.
func ParseReferralStartParam(param string) (string, bool) {
	if len(param) > 64 {
		return "", false
	}
	code, ok := strings.CutPrefix(param, ReferralStartPrefix)
	if !ok || !referralCodeRE.MatchString(code) {
		return "", false
	}
	return code, true
}

// pendingBotReferral is a /start ref_<CODE> the bot kept for this Telegram
// user within the last 30 days ("" for none or on any error).
func (s *Service) pendingBotReferral(ctx context.Context, tgUserID int64) string {
	code, err := s.Q.GetTelegramBotUserPendingReferral(ctx, tgUserID)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			s.logger().Warn("auth.pending_referral_lookup_failed", zap.Error(err))
		}
		return ""
	}
	return code
}

// applySignupReferral attaches referralCode to a profile created in tx.
//
// It is billing.ApplyReferralCode's rule set for the one case it can see
// here — a profile created a moment ago has no payment, is inside the attach
// window and cannot own the code — written against the transaction because
// auth cannot import billing (billing imports auth) and the referral must
// commit with the profile. Unknown codes and every failure only skip the
// referral (savepoint): a bonus must never cost anyone their sign-up.
func (s *Service) applySignupReferral(ctx context.Context, tx pgx.Tx, refereeID uuid.UUID, referralCode string) {
	code := strings.TrimSpace(referralCode)
	if !referralCodeRE.MatchString(code) {
		return
	}
	sp, err := tx.Begin(ctx)
	if err != nil {
		s.logger().Warn("auth.signup_referral_skipped", zap.Error(err))
		return
	}
	defer func() { _ = sp.Rollback(ctx) }()
	q := sqlc.New(sp)
	owner, err := q.GetReferralCodeOwner(ctx, code)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			s.logger().Warn("auth.signup_referral_skipped", zap.Error(err))
		}
		return
	}
	if owner.UserID == refereeID {
		return
	}
	if _, err := q.CreateReferral(ctx, sqlc.CreateReferralParams{
		ReferrerID: owner.UserID, RefereeID: refereeID, ReferralCode: owner.Code,
	}); err != nil {
		s.logger().Warn("auth.signup_referral_skipped", zap.Error(err))
		return
	}
	if err := sp.Commit(ctx); err != nil {
		s.logger().Warn("auth.signup_referral_skipped", zap.Error(err))
		return
	}
	s.logger().Info("auth.signup_referral_applied",
		zap.String("referee_id", refereeID.String()), zap.String("referrer_id", owner.UserID.String()))
}

// WebAppPhoneResult is a Mini App one-tap phone sign-in.
type WebAppPhoneResult struct {
	Tokens
	Profile sqlc.Profile
	Created bool
}

// TelegramWebAppPhoneSignIn is the Mini App's «📱 Raqam bilan davom etish»:
// launch data plus Telegram's signed share of the same user's phone find the
// learner with that phone — or create one, with no password — link the
// Telegram account phone-verified and sign in. Both payloads must be fresh
// (InitDataLinkMaxAge): this creates and moves links. A referral comes from
// the signed start_param (t.me/<bot>?startapp=ref_<CODE>) or, failing that,
// from a /start ref_<CODE> the bot kept, and only for a new profile.
func (s *Service) TelegramWebAppPhoneSignIn(ctx context.Context, initData, contact, ip string) (WebAppPhoneResult, error) {
	if err := s.checkTelegramIPFailures(ctx, ip); err != nil {
		return WebAppPhoneResult{}, err
	}
	if !s.miniAppEnabled() {
		return WebAppPhoneResult{}, ErrTelegramBotUnconfigured
	}
	u, err := ValidateInitData(initData, s.TelegramBotToken, s.clock(), InitDataLinkMaxAge)
	if err != nil {
		if limErr := s.noteTelegramIPFailure(ctx, ip); limErr != nil {
			return WebAppPhoneResult{}, limErr
		}
		return WebAppPhoneResult{}, err
	}
	if err := s.rateLimitTelegramUser(ctx, u.ID); err != nil {
		return WebAppPhoneResult{}, err
	}
	c, err := ValidateContact(contact, s.TelegramBotToken, s.clock(), InitDataLinkMaxAge)
	if err != nil {
		if limErr := s.noteTelegramIPFailure(ctx, ip); limErr != nil {
			return WebAppPhoneResult{}, limErr
		}
		return WebAppPhoneResult{}, err
	}
	if c.UserID != u.ID {
		return WebAppPhoneResult{}, ErrInitDataInvalid
	}
	phone, err := NormalizeTelegramContactPhone(c.Phone)
	if err != nil {
		return WebAppPhoneResult{}, ErrInvalidPhone
	}
	referral, ok := ParseReferralStartParam(u.StartParam)
	if !ok {
		referral = s.pendingBotReferral(ctx, u.ID)
	}
	who := TelegramLoginUser{ID: u.ID, Username: u.Username, FirstName: u.FirstName, LastName: u.LastName, LanguageCode: u.LanguageCode}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return WebAppPhoneResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	profile, created, err := s.findOrCreateTelegramProfile(ctx, tx, phone, who, referral)
	if errors.Is(err, errPhoneNotALearner) {
		s.logger().Warn("auth.telegram_phone_not_learner")
		return WebAppPhoneResult{}, ErrInvalidPhone
	}
	if err != nil {
		return WebAppPhoneResult{}, err
	}
	link := s.linkVerifiedTelegramInTx(ctx, tx, profile, u.ID, u.Username, &u)
	toks, err := s.issueSession(ctx, sqlc.New(tx), profile)
	if err != nil {
		return WebAppPhoneResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return WebAppPhoneResult{}, err
	}
	s.afterTelegramLink(ctx, link)
	if !link.linked {
		// Signed in without the link (logged by linkVerifiedTelegramInTx);
		// the next launch asks for the phone again, nothing is lost.
		s.joinBotAudience(ctx, u)
	}
	return WebAppPhoneResult{Tokens: toks, Profile: profile, Created: created}, nil
}
