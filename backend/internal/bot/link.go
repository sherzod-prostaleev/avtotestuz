package bot

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/db/sqlc"
)

// LinkTokenTTL is how long a generated link token stays redeemable. Short
// enough that a leaked/screenshotted deep link is a narrow window, long
// enough to survive switching from a browser to the Telegram app.
const LinkTokenTTL = 10 * time.Minute

var (
	ErrLinkTokenNotFound              = errors.New("link token not found")
	ErrLinkTokenExpired               = errors.New("link token expired")
	ErrLinkTokenAlreadyUsed           = errors.New("link token already used")
	ErrTelegramAccountLinkedElsewhere = errors.New("telegram account already linked to another profile")
)

// LinkService owns the auth-link flow: minting profile-scoped tokens for an
// authenticated web session, and redeeming them from the bot's trusted
// Telegram-update path. See design §3 for why there is no HTTP endpoint
// that redeems directly from client-supplied identifiers.
type LinkService struct {
	Q    *sqlc.Queries
	Pool *pgxpool.Pool
}

func NewLinkService(pool *pgxpool.Pool, q *sqlc.Queries) *LinkService {
	return &LinkService{Q: q, Pool: pool}
}

type LinkToken struct {
	Token     string
	ExpiresAt time.Time
}

// GenerateLinkToken mints a fresh, single-use token for profileID, first
// invalidating any of that profile's earlier unused tokens (design §3.2) so
// only the newest deep link a user requested is ever live.
func (s *LinkService) GenerateLinkToken(ctx context.Context, profileID uuid.UUID) (LinkToken, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return LinkToken{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := sqlc.New(tx)

	if err := q.DeleteUnusedLinkTokensForProfile(ctx, profileID); err != nil {
		return LinkToken{}, err
	}

	raw := newOpaqueToken()
	expiresAt := time.Now().Add(LinkTokenTTL)
	if _, err := q.CreateLinkToken(ctx, sqlc.CreateLinkTokenParams{
		ProfileID: profileID,
		TokenHash: hashToken(raw),
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	}); err != nil {
		return LinkToken{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return LinkToken{}, err
	}
	return LinkToken{Token: raw, ExpiresAt: expiresAt}, nil
}

// RedeemResult describes what RedeemLinkToken actually did, so the bot can
// phrase its reply ("linked!" vs "already linked").
type RedeemResult struct {
	ProfileID     uuid.UUID
	AlreadyLinked bool // true if this exact (profile, tg_user_id) pair was already bound
}

// RedeemLinkToken binds tgUserID/username to the profile that generated
// rawToken. tgUserID and username must come from a source Telegram itself
// vouches for (a verified webhook call or our own long-poll connection) —
// see design §3.1; this function does not and cannot verify that on its
// own, it only trusts its caller to have already done so.
//
// Runs inside a transaction with the token row locked (FOR UPDATE) for the
// whole check-then-act sequence, mirroring billing.LockProfileForGrant: two
// concurrent redemptions of the same token must not both observe
// used_at IS NULL before either writes it (design §3.4).
func (s *LinkService) RedeemLinkToken(ctx context.Context, rawToken string, tgUserID int64, username string) (RedeemResult, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return RedeemResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := sqlc.New(tx)

	tokenRow, err := q.GetLinkTokenByHashForUpdate(ctx, hashToken(rawToken))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RedeemResult{}, ErrLinkTokenNotFound
		}
		return RedeemResult{}, err
	}
	if tokenRow.UsedAt.Valid {
		return RedeemResult{}, ErrLinkTokenAlreadyUsed
	}
	if time.Now().After(tokenRow.ExpiresAt.Time) {
		return RedeemResult{}, ErrLinkTokenExpired
	}

	alreadyLinked := false
	existing, err := q.GetTelegramAccountByTgUserID(ctx, tgUserID)
	switch {
	case err == nil:
		if existing.ProfileID != tokenRow.ProfileID {
			// This Telegram account already belongs to a different profile.
			// Reject without touching either binding or marking the token
			// used — see design §3.3's hijack row. The legitimate owner of
			// this token can still redeem it after unlinking their existing
			// account (or a fresh generate() if this one turns out stale).
			return RedeemResult{}, ErrTelegramAccountLinkedElsewhere
		}
		alreadyLinked = true
	case errors.Is(err, pgx.ErrNoRows):
		// No existing binding for this Telegram account — proceed to link.
	default:
		return RedeemResult{}, err
	}

	if err := q.UpsertTelegramAccount(ctx, sqlc.UpsertTelegramAccountParams{
		ProfileID: tokenRow.ProfileID,
		TgUserID:  tgUserID,
		Username:  username,
	}); err != nil {
		if isUniqueViolation(err) {
			// Backstop for the race two concurrent redemptions targeting the
			// same tg_user_id under different tokens: the app-level check
			// above passed for both, but only one UPSERT can win the
			// tg_user_id unique constraint. See UpsertTelegramAccount's
			// query comment.
			return RedeemResult{}, ErrTelegramAccountLinkedElsewhere
		}
		return RedeemResult{}, err
	}
	if err := q.MarkLinkTokenUsed(ctx, tokenRow.ID); err != nil {
		return RedeemResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return RedeemResult{}, err
	}
	return RedeemResult{ProfileID: tokenRow.ProfileID, AlreadyLinked: alreadyLinked}, nil
}

// TelegramStatus is the learner-facing link state for GET /me/telegram.
type TelegramStatus struct {
	Linked   bool   `json:"linked"`
	Username string `json:"username,omitempty"`
	LinkedAt string `json:"linked_at,omitempty"`
}

// Status reports whether profileID has a bound Telegram account.
func (s *LinkService) Status(ctx context.Context, profileID uuid.UUID) (TelegramStatus, error) {
	acc, err := s.Q.GetTelegramAccountByProfileID(ctx, profileID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TelegramStatus{Linked: false}, nil
		}
		return TelegramStatus{}, err
	}
	out := TelegramStatus{Linked: true, Username: acc.Username}
	if acc.LinkedAt.Valid {
		out.LinkedAt = acc.LinkedAt.Time.UTC().Format(time.RFC3339)
	}
	return out, nil
}

// isUniqueViolation reports whether err is a Postgres unique-constraint
// violation (SQLSTATE 23505). Duplicated from billing/auth/payme's copies
// rather than imported — bot has no other reason to depend on any of them.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
