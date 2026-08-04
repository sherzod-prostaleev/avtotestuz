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

// defaultEnrollTTL and maxEnrollTTL bound how long a code stays live. Short
// is the point: a leaked code that expires in two hours is worth little.
const (
	defaultEnrollTTL = 2 * time.Hour
	maxEnrollTTL     = 24 * time.Hour
)

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
func (s Store) OpenEnrollWindow(ctx context.Context, orgID uuid.UUID, ttl time.Duration, createdBy string) (EnrollCodeRow, error) {
	if ttl <= 0 {
		ttl = defaultEnrollTTL
	}
	if ttl > maxEnrollTTL {
		ttl = maxEnrollTTL
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

// OpenEnrollWindowAsTeacher requires owner/teacher membership.
func (s Store) OpenEnrollWindowAsTeacher(ctx context.Context, actorID, orgID uuid.UUID, ttl time.Duration) (EnrollCodeRow, error) {
	if _, err := s.teacherRole(ctx, actorID, orgID); err != nil {
		return EnrollCodeRow{}, err
	}
	return s.OpenEnrollWindow(ctx, orgID, ttl, "profile:"+actorID.String())
}

// CloseEnrollWindowAsTeacher revokes a live window early.
func (s Store) CloseEnrollWindowAsTeacher(ctx context.Context, actorID, orgID, codeID uuid.UUID) error {
	if _, err := s.teacherRole(ctx, actorID, orgID); err != nil {
		return err
	}
	tag, err := s.Pool.Exec(ctx, `
		UPDATE b2b_org_enroll_code SET revoked_at = now()
		WHERE id = $1 AND org_id = $2 AND revoked_at IS NULL`, codeID, orgID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ActiveEnrollCodeAsTeacher returns the live window, or nil when none is open.
func (s Store) ActiveEnrollCodeAsTeacher(ctx context.Context, actorID, orgID uuid.UUID) (*EnrollCodeRow, error) {
	if _, err := s.teacherRole(ctx, actorID, orgID); err != nil {
		return nil, err
	}
	var row EnrollCodeRow
	var revoked pgtype.Timestamptz
	err := s.Pool.QueryRow(ctx, `
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
		return nil, fmt.Errorf("active enroll code: %w", err)
	}
	row.ExpiresAt = row.ExpiresAt.UTC()
	row.CreatedAt = row.CreatedAt.UTC()
	return &row, nil
}
