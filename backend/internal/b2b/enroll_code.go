package b2b

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// EnrollCodeRow is an org-scoped enrollment window. One code enrolls every PC
// in a school, capped by free seats and a short expiry — the per-PC codes it
// replaced made a 100-machine rollout unmanageable.
type EnrollCodeRow struct {
	ID        uuid.UUID  `json:"id"`
	OrgID     uuid.UUID  `json:"org_id"`
	Code      string     `json:"code"`
	MaxUses   int        `json:"max_uses"`
	UsedCount int        `json:"used_count"`
	ExpiresAt time.Time  `json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// enrollAlphabet excludes I, O, 0 and 1 — codes get read aloud and retyped by
// school IT staff.
const enrollAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func newEnrollCode() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, 0, 13) // AVTO- + 4 + - + 4
	out = append(out, "AVTO-"...)
	for i, b := range buf {
		if i == 4 {
			out = append(out, '-')
		}
		out = append(out, enrollAlphabet[int(b)%len(enrollAlphabet)])
	}
	return string(out), nil
}

// OpenEnrollWindow revokes any live window and mints a new one sized to the
// org's free seats.
//
// Deprecated: superseded by OpenInstallerKey, whose expiry tracks the
// licence instead of a fixed TTL. Kept only because b2b's other test files
// still use it as enroll-code test scaffolding.
func (s Store) OpenEnrollWindow(ctx context.Context, orgID uuid.UUID, ttl time.Duration, createdBy string) (EnrollCodeRow, error) {
	if ttl <= 0 {
		ttl = 2 * time.Hour
	}
	if ttl > 24*time.Hour {
		ttl = 24 * time.Hour
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return EnrollCodeRow{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var status string
	if err := tx.QueryRow(ctx,
		`SELECT status FROM b2b_org WHERE id = $1 FOR UPDATE`, orgID).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return EnrollCodeRow{}, ErrNotFound
		}
		return EnrollCodeRow{}, err
	}
	if status != "active" {
		return EnrollCodeRow{}, ErrOrgSuspended
	}

	var seats int64
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(seats), 0) FROM b2b_org_license
		WHERE org_id = $1 AND starts_at <= now() AND ends_at > now()`, orgID).Scan(&seats); err != nil {
		return EnrollCodeRow{}, err
	}
	if seats <= 0 {
		return EnrollCodeRow{}, ErrNoLicense
	}
	var used int64
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM b2b_station WHERE org_id = $1 AND status = 'active'`, orgID).Scan(&used); err != nil {
		return EnrollCodeRow{}, err
	}
	free := seats - used
	if free <= 0 {
		return EnrollCodeRow{}, ErrSeatsExhausted
	}

	if _, err := tx.Exec(ctx, `
		UPDATE b2b_org_enroll_code SET revoked_at = now()
		WHERE org_id = $1 AND revoked_at IS NULL AND expires_at > now()`, orgID); err != nil {
		return EnrollCodeRow{}, err
	}

	code, err := newEnrollCode()
	if err != nil {
		return EnrollCodeRow{}, err
	}
	row, err := insertEnrollCode(ctx, tx, orgID, code, int(free), time.Now().UTC().Add(ttl), createdBy)
	if err != nil {
		return EnrollCodeRow{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return EnrollCodeRow{}, err
	}
	return row, nil
}

func insertEnrollCode(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, code string, maxUses int, expires time.Time, createdBy string) (EnrollCodeRow, error) {
	var row EnrollCodeRow
	var revoked pgtype.Timestamptz
	err := tx.QueryRow(ctx, `
		INSERT INTO b2b_org_enroll_code (org_id, code, max_uses, expires_at, created_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, org_id, code, max_uses, used_count, expires_at, revoked_at, created_at`,
		orgID, code, maxUses, expires, createdBy,
	).Scan(&row.ID, &row.OrgID, &row.Code, &row.MaxUses, &row.UsedCount,
		&row.ExpiresAt, &revoked, &row.CreatedAt)
	if err != nil {
		return EnrollCodeRow{}, err
	}
	if revoked.Valid {
		t := revoked.Time.UTC()
		row.RevokedAt = &t
	}
	row.ExpiresAt = row.ExpiresAt.UTC()
	row.CreatedAt = row.CreatedAt.UTC()
	return row, nil
}

// mintEnrollCode revokes any live code and inserts a fresh one expiring at
// expiresAt. The caller holds the org row lock.
func mintEnrollCode(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, expiresAt time.Time, maxUses int, createdBy string) (EnrollCodeRow, error) {
	if _, err := tx.Exec(ctx, `
		UPDATE b2b_org_enroll_code SET revoked_at = now()
		WHERE org_id = $1 AND revoked_at IS NULL AND expires_at > now()`, orgID); err != nil {
		return EnrollCodeRow{}, err
	}
	code, err := newEnrollCode()
	if err != nil {
		return EnrollCodeRow{}, err
	}
	return insertEnrollCode(ctx, tx, orgID, code, maxUses, expiresAt, createdBy)
}

// installerKeyTx is the shared body of OpenInstallerKey and RotateInstallerKey.
// reuse=true returns a live key untouched; reuse=false always mints a new one.
//
// The org row is locked before any seat arithmetic for the same reason
// EnrollStation locks it: the free-seat count feeds max_uses, and two
// concurrent admins would otherwise both size a key against a stale count.
func (s Store) installerKeyTx(ctx context.Context, orgID uuid.UUID, createdBy string, reuse bool) (EnrollCodeRow, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return EnrollCodeRow{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var status string
	if err := tx.QueryRow(ctx,
		`SELECT status FROM b2b_org WHERE id = $1 FOR UPDATE`, orgID).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return EnrollCodeRow{}, ErrNotFound
		}
		return EnrollCodeRow{}, err
	}
	if status != "active" {
		return EnrollCodeRow{}, ErrOrgSuspended
	}

	var seats int64
	var licenseEnds time.Time
	err = tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(seats), 0), COALESCE(MAX(ends_at), now())
		FROM b2b_org_license
		WHERE org_id = $1 AND starts_at <= now() AND ends_at > now()`, orgID).Scan(&seats, &licenseEnds)
	if err != nil {
		return EnrollCodeRow{}, err
	}
	if seats <= 0 {
		return EnrollCodeRow{}, ErrNoLicense
	}

	if reuse {
		row, err := activeEnrollCodeTx(ctx, tx, orgID)
		if err != nil {
			return EnrollCodeRow{}, err
		}
		if row != nil {
			if err := tx.Commit(ctx); err != nil {
				return EnrollCodeRow{}, err
			}
			return *row, nil
		}
	}

	var used int64
	if err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM b2b_station WHERE org_id = $1 AND status = 'active'`, orgID).Scan(&used); err != nil {
		return EnrollCodeRow{}, err
	}
	free := seats - used
	if free <= 0 {
		return EnrollCodeRow{}, ErrSeatsExhausted
	}

	row, err := mintEnrollCode(ctx, tx, orgID, licenseEnds.UTC(), int(free), createdBy)
	if err != nil {
		return EnrollCodeRow{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return EnrollCodeRow{}, err
	}
	return row, nil
}

// OpenInstallerKey returns the org's live installer key, minting one only if
// there is none. It is idempotent on purpose: an admin installing 30 PCs over
// several days downloads the installer more than once, and a fresh key each
// time would kill the copies already handed out.
func (s Store) OpenInstallerKey(ctx context.Context, orgID uuid.UUID, createdBy string) (EnrollCodeRow, error) {
	return s.installerKeyTx(ctx, orgID, createdBy, true)
}

// RotateInstallerKey revokes the live key and mints a new one. Stations already
// enrolled are unaffected — they authenticate with their own Ed25519 key, not
// with this one.
func (s Store) RotateInstallerKey(ctx context.Context, orgID uuid.UUID, createdBy string) (EnrollCodeRow, error) {
	return s.installerKeyTx(ctx, orgID, createdBy, false)
}

// ActiveInstallerKey returns the live key, or nil when none is open.
func (s Store) ActiveInstallerKey(ctx context.Context, orgID uuid.UUID) (*EnrollCodeRow, error) {
	return activeEnrollCodeTx(ctx, s.Pool, orgID)
}

// pgxQuerier is satisfied by both *pgxpool.Pool and pgx.Tx.
type pgxQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func activeEnrollCodeTx(ctx context.Context, q pgxQuerier, orgID uuid.UUID) (*EnrollCodeRow, error) {
	var row EnrollCodeRow
	var revoked pgtype.Timestamptz
	err := q.QueryRow(ctx, `
		SELECT id, org_id, code, max_uses, used_count, expires_at, revoked_at, created_at
		FROM b2b_org_enroll_code
		WHERE org_id = $1 AND revoked_at IS NULL AND expires_at > now()
		  AND used_count < max_uses
		ORDER BY created_at DESC LIMIT 1`, orgID,
	).Scan(&row.ID, &row.OrgID, &row.Code, &row.MaxUses, &row.UsedCount,
		&row.ExpiresAt, &revoked, &row.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("active installer key: %w", err)
	}
	row.ExpiresAt = row.ExpiresAt.UTC()
	row.CreatedAt = row.CreatedAt.UTC()
	return &row, nil
}
